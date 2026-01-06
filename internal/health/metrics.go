package health

import (
	"strconv"
	"time"

	"github.com/eslutz/torarr/internal/tor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type metrics struct {
	requestDuration *prometheus.HistogramVec
	requestTotal    *prometheus.CounterVec

	torBootstrap     prometheus.Gauge
	torCircuit       prometheus.Gauge
	torReady         prometheus.Gauge
	torBytesRead     prometheus.Gauge
	torBytesWritten  prometheus.Gauge
	externalAttempts *prometheus.CounterVec

	webhookRequests *prometheus.CounterVec
	webhookDuration *prometheus.HistogramVec
}

func newMetrics() *metrics {
	return &metrics{
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "torarr_http_request_duration_seconds",
			Help:    "HTTP request duration for the health server.",
			Buckets: prometheus.DefBuckets,
		}, []string{"path", "method", "code"}),
		requestTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "torarr_http_requests_total",
			Help: "Total HTTP requests processed by the health server.",
		}, []string{"path", "method", "code"}),
		torBootstrap: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "torarr_tor_bootstrap_percent",
			Help: "Bootstrap progress reported by Tor.",
		}),
		torCircuit: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "torarr_tor_circuit_established",
			Help: "Whether Tor reports an established circuit (1 = yes, 0 = no).",
		}),
		torReady: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "torarr_tor_ready",
			Help: "Tor readiness derived from bootstrap progress (1 = ready, 0 = not ready).",
		}),
		torBytesRead: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "torarr_tor_bytes_read",
			Help: "Bytes read as reported by Tor traffic stats.",
		}),
		torBytesWritten: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "torarr_tor_bytes_written",
			Help: "Bytes written as reported by Tor traffic stats.",
		}),
		externalAttempts: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "torarr_external_check_total",
			Help: "External check attempts with result labels.",
		}, []string{"endpoint", "success", "is_tor"}),
		webhookRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "torarr_webhook_requests_total",
			Help: "Total webhook notification attempts.",
		}, []string{"event", "status"}),
		webhookDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "torarr_webhook_duration_seconds",
			Help:    "Webhook notification duration.",
			Buckets: prometheus.DefBuckets,
		}, []string{"event"}),
	}
}

func (m *metrics) observeRequest(path, method string, code int, duration time.Duration) {
	codeStr := strconv.Itoa(code)
	m.requestDuration.WithLabelValues(path, method, codeStr).Observe(duration.Seconds())
	m.requestTotal.WithLabelValues(path, method, codeStr).Inc()
}

func (m *metrics) observeTorStatus(status *tor.Status) {
	m.torBootstrap.Set(float64(status.BootstrapPhase))
	if status.CircuitEstablished {
		m.torCircuit.Set(1)
		m.torReady.Set(1)
	} else {
		m.torCircuit.Set(0)
		m.torReady.Set(0)
	}
	m.torBytesRead.Set(float64(status.Traffic.BytesRead))
	m.torBytesWritten.Set(float64(status.Traffic.BytesWritten))
}

func (m *metrics) observeExternalCheck(endpoint string, success, isTor bool) {
	m.externalAttempts.WithLabelValues(endpoint, strconv.FormatBool(success), strconv.FormatBool(isTor)).Inc()
}

func (m *metrics) observeWebhook(event string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "failed"
	}
	m.webhookRequests.WithLabelValues(event, status).Inc()
	m.webhookDuration.WithLabelValues(event).Observe(duration.Seconds())
}
