package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/eslutz/torarr/internal/config"
	"github.com/eslutz/torarr/internal/notify"
	"github.com/eslutz/torarr/internal/tor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	torClient        *tor.Client
	readinessChecker *ExternalChecker
	config           *config.Config
	metrics          *metrics
	webhook          *notify.Webhook
	webhookEvents    []string
	previousHealthy  *bool      // Tracks previous health state for change detection
	healthMu         sync.Mutex // Protects previousHealthy from concurrent access
}

func NewHandler(cfg *config.Config) *Handler {
	torClient := tor.NewClient(cfg.TorControlAddress, cfg.TorControlPassword)
	metrics := newMetrics()

	readinessChecker := NewExternalChecker(
		cfg.HealthExternalEndpoints,
		time.Duration(cfg.HealthExternalTimeout)*time.Second,
		"socks5://127.0.0.1:9050",
	)

	// Initialize webhook if URL is configured
	var webhook *notify.Webhook
	if cfg.WebhookURL != "" {
		template := notify.Template(cfg.WebhookTemplate)
		webhook = notify.NewWebhook(cfg.WebhookURL, template)
	}

	return &Handler{
		torClient:        torClient,
		readinessChecker: readinessChecker,
		config:           cfg,
		metrics:          metrics,
		webhook:          webhook,
		webhookEvents:    cfg.WebhookEvents,
	}
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "OK",
	}); err != nil {
		slog.Error("Failed to encode ping response", "error", err)
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status, err := h.torClient.GetStatus()
	if err != nil {
		if h.metrics != nil {
			h.metrics.torReady.Set(0)
		}

		// Check for health state change first to avoid duplicate notifications
		stateChanged := h.checkHealthStateChange(false)

		// Send EventBootstrapFailed webhook only if state didn't change
		// (EventHealthChanged already sent if state changed)
		if !stateChanged {
			h.sendWebhook(notify.EventBootstrapFailed, "Tor bootstrap failed", notify.Details{
				Error: err.Error(),
			})
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "NOT_READY",
			"error":  "tor not ready",
		}); err != nil {
			slog.Error("Failed to encode health response", "error", err)
		}
		return
	}

	if status.BootstrapPhase < 100 {
		if h.metrics != nil {
			h.metrics.observeTorStatus(status)
		}

		// Check for health state change first to avoid duplicate notifications
		stateChanged := h.checkHealthStateChange(false)

		// Send EventBootstrapFailed webhook only if state didn't change
		// (EventHealthChanged already sent if state changed)
		if !stateChanged {
			h.sendWebhook(notify.EventBootstrapFailed, "Tor bootstrap incomplete", notify.Details{
				Bootstrap: &status.BootstrapPhase,
				Circuits:  status.NumCircuits,
			})
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "NOT_READY",
			"error":  "tor not ready",
		}); err != nil {
			slog.Error("Failed to encode health response", "error", err)
		}
		return
	}

	if h.metrics != nil {
		h.metrics.observeTorStatus(status)
	}

	// Check for health state change to healthy
	h.checkHealthStateChange(true)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "READY",
	}); err != nil {
		slog.Error("Failed to encode health response", "error", err)
	}
}

// Ready checks whether Tor egress is functioning by hitting external endpoints through the SOCKS proxy.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result := h.readinessChecker.Check()

	if h.metrics != nil {
		h.metrics.observeExternalCheck(result.Endpoint, result.Success, result.IsTor)
	}

	if !result.Success || !result.IsTor {
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(result); err != nil {
			slog.Error("Failed to encode ready response", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("Failed to encode ready response", "error", err)
	}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status, err := h.torClient.GetStatus()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ERROR",
			"error":  err.Error(),
		}); err != nil {
			slog.Error("Failed to encode status response", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":              "OK",
		"version":             status.Version,
		"bootstrap_phase":     status.BootstrapPhase,
		"circuit_established": status.CircuitEstablished,
		"num_circuits":        status.NumCircuits,
		"traffic": map[string]int64{
			"bytes_read":    status.Traffic.BytesRead,
			"bytes_written": status.Traffic.BytesWritten,
		},
	}); err != nil {
		slog.Error("Failed to encode status response", "error", err)
	}

	if h.metrics != nil {
		h.metrics.observeTorStatus(status)
	}
}

func (h *Handler) Renew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := h.torClient.Signal("NEWNYM"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			slog.Error("Failed to encode renew error response", "error", err)
		}
		return
	}

	// Build details for webhook notification using current Tor status, if available
	if status, err := h.torClient.GetStatus(); err != nil {
		// If we can't retrieve status, skip sending a potentially misleading webhook
		slog.Warn("Failed to get Tor status after NEWNYM; skipping circuit renewal webhook", "error", err)
	} else {
		details := notify.Details{
			Circuits: status.NumCircuits,
			Healthy:  status.CircuitEstablished,
		}

		// Send webhook notification if configured
		h.sendWebhook(notify.EventCircuitRenewed, "Tor circuit renewal requested", details)
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "OK", "message": "Signal NEWNYM sent"}); err != nil {
		slog.Error("Failed to encode renew response", "error", err)
	}
}

func (h *Handler) Close() error {
	return h.torClient.Close()
}

func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ping", h.instrument("/ping", h.Ping))
	mux.HandleFunc("/health", h.instrument("/health", h.Health))
	mux.HandleFunc("/ready", h.instrument("/ready", h.Ready))
	mux.HandleFunc("/status", h.instrument("/status", h.Status))
	mux.HandleFunc("/renew", h.instrument("/renew", h.Renew))
	mux.Handle("/metrics", promhttp.Handler())
}

func (h *Handler) instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next(recorder, r)
		duration := time.Since(start)

		slog.Info("request handled",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration", duration,
		)

		if h.metrics != nil {
			h.metrics.observeRequest(path, r.Method, recorder.status, duration)
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// sendWebhook sends a webhook notification if enabled and the event is configured
func (h *Handler) sendWebhook(event notify.Event, message string, details notify.Details) {
	if h.webhook == nil {
		return
	}

	// Check if this event is enabled in configuration
	if !slices.Contains(h.webhookEvents, string(event)) {
		return
	}

	payload := notify.Payload{
		Event:     event,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}

	// Send webhook in background with timeout
	go func() {
		// Recover from panics to prevent crashing the application
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Webhook goroutine panicked",
					"event", event,
					"panic", r,
				)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), h.config.WebhookTimeout)
		defer cancel()

		start := time.Now()
		err := h.webhook.Send(ctx, payload)
		duration := time.Since(start)

		success := err == nil
		if h.metrics != nil {
			h.metrics.observeWebhook(string(event), success, duration)
		}

		if err != nil {
			slog.Error("Webhook notification failed",
				"event", event,
				"error", err,
				"duration", duration,
			)
		} else {
			slog.Debug("Webhook notification sent",
				"event", event,
				"duration", duration,
			)
		}
	}()
}

// checkHealthStateChange detects health state transitions and sends EventHealthChanged webhook
// Returns true if the state changed, false otherwise
func (h *Handler) checkHealthStateChange(currentlyHealthy bool) bool {
	h.healthMu.Lock()
	defer h.healthMu.Unlock()

	if h.previousHealthy == nil {
		// First call - initialize state
		h.previousHealthy = &currentlyHealthy
		return false
	}

	// Check if state has changed
	if *h.previousHealthy != currentlyHealthy {
		var message string
		if currentlyHealthy {
			message = "Tor health status changed to healthy"
		} else {
			message = "Tor health status changed to unhealthy"
		}

		h.sendWebhook(notify.EventHealthChanged, message, notify.Details{
			Healthy: currentlyHealthy,
		})

		// Update previous state
		*h.previousHealthy = currentlyHealthy
		return true
	}

	return false
}
