package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/eslutz/torarr/internal/config"
	"github.com/eslutz/torarr/internal/tor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	torClient        *tor.Client
	readinessChecker *ExternalChecker
	config           *config.Config
	metrics          *metrics
}

func NewHandler(cfg *config.Config) *Handler {
	torClient := tor.NewClient(cfg.TorControlAddress, cfg.TorControlPassword)
	metrics := newMetrics()

	readinessChecker := NewExternalChecker(
		cfg.HealthExternalEndpoints,
		time.Duration(cfg.HealthExternalTimeout)*time.Second,
		"socks5://127.0.0.1:9050",
	)

	return &Handler{
		torClient:        torClient,
		readinessChecker: readinessChecker,
		config:           cfg,
		metrics:          metrics,
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
