package health

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/eslutz/torarr/internal/config"
	"github.com/eslutz/torarr/internal/tor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	torClient       *tor.Client
	externalChecker *ExternalChecker
	config          *config.Config
	metrics         *metrics
}

func NewHandler(cfg *config.Config) *Handler {
	torClient := tor.NewClient(cfg.TorControlAddress, cfg.TorControlPassword)
	metrics := newMetrics()

	externalChecker := NewExternalChecker(
		cfg.HealthExternalEndpoints,
		time.Duration(cfg.HealthFullTimeout)*time.Second,
		"socks5://127.0.0.1:9050",
	)

	return &Handler{
		torClient:       torClient,
		externalChecker: externalChecker,
		config:          cfg,
		metrics:         metrics,
	}
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "OK",
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status, err := h.torClient.GetStatus()
	if err != nil {
		if h.metrics != nil {
			h.metrics.torReady.Set(0)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "NOT_READY",
			"error":  "tor not ready",
		})
		return
	}

	if status.BootstrapPhase < 100 {
		if h.metrics != nil {
			h.metrics.observeTorStatus(status)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "NOT_READY",
			"error":  "tor not ready",
		})
		return
	}

	if h.metrics != nil {
		h.metrics.observeTorStatus(status)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "READY",
	})
}

func (h *Handler) External(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result := h.externalChecker.Check()

	if h.metrics != nil {
		h.metrics.observeExternalCheck(result.Endpoint, result.Success, result.IsTor)
	}

	if !result.Success || !result.IsTor {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(result)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status, err := h.torClient.GetStatus()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ERROR",
			"error":  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":              "OK",
		"version":             status.Version,
		"bootstrap_phase":     status.BootstrapPhase,
		"circuit_established": status.CircuitEstablished,
		"num_circuits":        status.NumCircuits,
		"traffic": map[string]int64{
			"bytes_read":    status.Traffic.BytesRead,
			"bytes_written": status.Traffic.BytesWritten,
		},
	})

	if h.metrics != nil {
		h.metrics.observeTorStatus(status)
	}
}

func (h *Handler) Close() error {
	return h.torClient.Close()
}

func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ping", h.instrument("/ping", h.Ping))
	mux.HandleFunc("/health", h.instrument("/health", h.Health))
	mux.HandleFunc("/health/external", h.instrument("/health/external", h.External))
	mux.HandleFunc("/status", h.instrument("/status", h.Status))
	mux.Handle("/metrics", promhttp.Handler())
}

func (h *Handler) instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next(recorder, r)
		duration := time.Since(start)

		log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.status, duration)

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
