package health

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/eslutz/torarr/internal/config"
	"github.com/eslutz/torarr/internal/tor"
)

type Handler struct {
	torClient       *tor.Client
	externalChecker *ExternalChecker
	config          *config.Config
}

func NewHandler(cfg *config.Config) *Handler {
	torClient := tor.NewClient(cfg.TorControlAddress, cfg.TorControlPassword)
	
	externalChecker := NewExternalChecker(
		cfg.HealthExternalEndpoints,
		time.Duration(cfg.HealthFullTimeout)*time.Second,
		"socks5://127.0.0.1:9050",
	)

	return &Handler{
		torClient:       torClient,
		externalChecker: externalChecker,
		config:          cfg,
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

	if !h.torClient.IsReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "NOT_READY",
			"error":  "tor not ready",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "READY",
	})
}

func (h *Handler) External(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result := h.externalChecker.Check()

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
}

func (h *Handler) Close() error {
	return h.torClient.Close()
}

func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ping", h.Ping)
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/health/external", h.External)
	mux.HandleFunc("/status", h.Status)
}

func (h *Handler) LogRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	}
}
