package router

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RPM metrics
	rpmGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gateway_rpm",
		Help: "Current requests per minute",
	}, []string{"provider", "key", "model"})

	rateLimitedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_rate_limited_total",
		Help: "Total 429 responses",
	}, []string{"provider", "model"})

	latencyHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help: "Request latency distribution",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "model", "status"})

	cooldownGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gateway_cooldown_remaining_seconds",
		Help: "Remaining cooldown time",
	}, []string{"model"})

	counterGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gateway_active_requests",
		Help: "Number of active requests",
	}, []string{"provider", "key"})
)

// RecordMetrics records metrics for a request.
func RecordMetrics(provider, key, model string, latency float64, isRateLimited bool, status string) {
	if isRateLimited {
		rateLimitedCounter.WithLabelValues(provider, model).Inc()
	}
	latencyHistogram.WithLabelValues(provider, model, status).Observe(latency)
}

// UpdateRPM updates the RPM gauge for a key.
func UpdateRPM(provider, key, model string, rpm int) {
	rpmGauge.WithLabelValues(provider, key, model).Set(float64(rpm))
}

// UpdateCooldown updates the cooldown gauge.
func UpdateCooldown(model string, remaining float64) {
	cooldownGauge.WithLabelValues(model).Set(remaining)
}

// IncActiveRequests increments the active request counter.
func IncActiveRequests(provider, key string) {
	counterGauge.WithLabelValues(provider, key).Inc()
}

// DecActiveRequests decrements the active request counter.
func DecActiveRequests(provider, key string) {
	counterGauge.WithLabelValues(provider, key).Dec()
}
