package httpx

import (
	"net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusConfig configures Prometheus metrics collection
type PrometheusConfig struct {
	Namespace          string
	Subsystem          string
	Registry           prometheus.Registerer
	DurationBuckets    []float64 // Seconds
	SizeBuckets        []float64 // Bytes
	IncludeHostLabel   bool
	IncludeMethodLabel bool
	ExtraLabels        []string
}

// DefaultPrometheusConfig returns sensible defaults for Prometheus metrics
func DefaultPrometheusConfig() PrometheusConfig {
	return PrometheusConfig{
		Namespace:          "",
		Subsystem:          "http_client",
		Registry:           prometheus.DefaultRegisterer,
		DurationBuckets:    []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		SizeBuckets:        []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		IncludeHostLabel:   true,
		IncludeMethodLabel: true,
	}
}

// PrometheusCollector implements MetricsCollector interface for Prometheus
type PrometheusCollector struct {
	config PrometheusConfig

	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestSize      *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	errorsTotal      *prometheus.CounterVec
	inFlightRequests prometheus.Gauge
}

// NewPrometheusCollector creates a new Prometheus metrics collector
func NewPrometheusCollector(config PrometheusConfig) (*PrometheusCollector, error) {
	if config.Registry == nil {
		config.Registry = prometheus.DefaultRegisterer
	}

	// Base labels
	labels := []string{"status_code"}
	if config.IncludeMethodLabel {
		labels = append(labels, "method")
	}
	if config.IncludeHostLabel {
		labels = append(labels, "host")
	}
	labels = append(labels, config.ExtraLabels...)

	// Error labels
	errorLabels := []string{"error_type"}
	if config.IncludeMethodLabel {
		errorLabels = append(errorLabels, "method")
	}
	if config.IncludeHostLabel {
		errorLabels = append(errorLabels, "host")
	}

	collector := &PrometheusCollector{
		config: config,
	}

	// Register metrics
	factory := promauto.With(config.Registry)

	collector.requestsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests made",
		},
		labels,
	)

	collector.requestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency distribution",
			Buckets:   config.DurationBuckets,
		},
		labels,
	)

	collector.requestSize = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "request_size_bytes",
			Help:      "HTTP request size distribution",
			Buckets:   config.SizeBuckets,
		},
		labels,
	)

	collector.responseSize = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "response_size_bytes",
			Help:      "HTTP response size distribution",
			Buckets:   config.SizeBuckets,
		},
		labels,
	)

	collector.errorsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "errors_total",
			Help:      "Total number of HTTP errors",
		},
		errorLabels,
	)

	collector.inFlightRequests = factory.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "in_flight_requests",
			Help:      "Current number of in-flight HTTP requests",
		},
	)

	return collector, nil
}

// IncrementRequests implements MetricsCollector interface
func (c *PrometheusCollector) IncrementRequests(method, rawURL string) {
	c.inFlightRequests.Inc()

	labels := c.buildLabels(method, rawURL, 0)
	c.requestsTotal.With(labels).Inc()
}

// IncrementErrors implements MetricsCollector interface
func (c *PrometheusCollector) IncrementErrors(method, rawURL string, statusCode int) {
	c.inFlightRequests.Dec()

	// Determine error type
	var errorType string
	switch {
	case statusCode == 0:
		errorType = "network"
	case statusCode >= 400 && statusCode < 500:
		errorType = "client_error"
	case statusCode >= 500:
		errorType = "server_error"
	default:
		errorType = "unknown"
	}

	errorLabels := prometheus.Labels{"error_type": errorType}
	if c.config.IncludeMethodLabel {
		errorLabels["method"] = method
	}
	if c.config.IncludeHostLabel {
		errorLabels["host"] = c.extractHost(rawURL)
	}

	c.errorsTotal.With(errorLabels).Inc()
}

// RecordDuration implements MetricsCollector interface
func (c *PrometheusCollector) RecordDuration(method, rawURL string, duration time.Duration) {
	c.inFlightRequests.Dec()

	labels := c.buildLabels(method, rawURL, 0)
	c.requestDuration.With(labels).Observe(duration.Seconds())
}

// RecordRequestSize records the size of the request body
func (c *PrometheusCollector) RecordRequestSize(method, rawURL string, size int64) {
	labels := c.buildLabels(method, rawURL, 0)
	c.requestSize.With(labels).Observe(float64(size))
}

// RecordResponseSize records the size of the response body
func (c *PrometheusCollector) RecordResponseSize(method, rawURL string, statusCode int, size int64) {
	labels := c.buildLabels(method, rawURL, statusCode)
	c.responseSize.With(labels).Observe(float64(size))
}

// buildLabels constructs Prometheus labels from request information
func (c *PrometheusCollector) buildLabels(method, rawURL string, statusCode int) prometheus.Labels {
	labels := prometheus.Labels{}

	if statusCode > 0 {
		labels["status_code"] = strconv.Itoa(statusCode)
	} else {
		labels["status_code"] = "0"
	}

	if c.config.IncludeMethodLabel {
		labels["method"] = method
	}

	if c.config.IncludeHostLabel {
		labels["host"] = c.extractHost(rawURL)
	}

	return labels
}

// extractHost extracts the host from a URL string
func (c *PrometheusCollector) extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}
	return u.Host
}
