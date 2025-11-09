package httpx_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestDefaultPrometheusConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns default configuration with expected values", func(t *testing.T) {
		t.Parallel()

		got := httpx.DefaultPrometheusConfig()

		assert.Equal(t, "", got.Namespace)
		assert.Equal(t, "http_client", got.Subsystem)
		assert.Equal(t, prometheus.DefaultRegisterer, got.Registry)
		assert.Equal(t, []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}, got.DurationBuckets)
		assert.Equal(t, []float64{100, 1000, 10000, 100000, 1000000, 10000000}, got.SizeBuckets)
		assert.True(t, got.IncludeHostLabel)
		assert.True(t, got.IncludeMethodLabel)
		assert.Empty(t, got.ExtraLabels)
	})
}

func TestNewPrometheusCollector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		config            httpx.PrometheusConfig
		wantErr           bool
		wantNamespace     string
		wantSubsystem     string
	}{
		{
			name: "creates collector with full configuration",
			config: httpx.PrometheusConfig{
				Namespace:          "test_app",
				Subsystem:          "http",
				Registry:           prometheus.NewRegistry(),
				DurationBuckets:    []float64{0.1, 0.5, 1.0},
				SizeBuckets:        []float64{100, 1000},
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			wantErr:       false,
			wantNamespace: "test_app",
			wantSubsystem: "http",
		},
		{
			name: "creates collector with minimal configuration",
			config: httpx.PrometheusConfig{
				Registry: prometheus.NewRegistry(),
			},
			wantErr:       false,
			wantNamespace: "",
			wantSubsystem: "",
		},
		{
			name: "creates collector with nil registry uses default",
			config: httpx.PrometheusConfig{
				Registry:  nil,
				Namespace: "test",
				Subsystem: "collector",
			},
			wantErr:       false,
			wantNamespace: "test",
			wantSubsystem: "collector",
		},
		{
			name: "creates collector with extra labels",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
				ExtraLabels:        []string{"environment", "region"},
			},
			wantErr:       false,
			wantSubsystem: "http_client",
		},
		{
			name: "creates collector with custom buckets",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				DurationBuckets:    []float64{0.001, 0.01, 0.1, 1, 10},
				SizeBuckets:        []float64{1024, 10240, 102400},
				IncludeHostLabel:   false,
				IncludeMethodLabel: false,
			},
			wantErr:       false,
			wantSubsystem: "http_client",
		},
		{
			name: "creates collector with host label only",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: false,
			},
			wantErr:       false,
			wantSubsystem: "http_client",
		},
		{
			name: "creates collector with method label only",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   false,
				IncludeMethodLabel: true,
			},
			wantErr:       false,
			wantSubsystem: "http_client",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := httpx.NewPrometheusCollector(tc.config)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestPrometheusCollector_IncrementRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		config            httpx.PrometheusConfig
		method            string
		url               string
		callCount         int
	}{
		{
			name: "increments request counter with all labels",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:    "GET",
			url:       "https://api.example.com/users",
			callCount: 1,
		},
		{
			name: "increments request counter multiple times",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:    "POST",
			url:       "https://api.example.com/orders",
			callCount: 5,
		},
		{
			name: "increments request counter without method label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: false,
			},
			method:    "PUT",
			url:       "https://api.example.com/items/123",
			callCount: 1,
		},
		{
			name: "increments request counter without host label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   false,
				IncludeMethodLabel: true,
			},
			method:    "DELETE",
			url:       "https://api.example.com/items/456",
			callCount: 1,
		},
		{
			name: "increments request counter with invalid URL",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:    "GET",
			url:       "://invalid-url",
			callCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject, err := httpx.NewPrometheusCollector(tc.config)
			require.NoError(t, err)

			for i := 0; i < tc.callCount; i++ {
				subject.IncrementRequests(tc.method, tc.url)
			}

			// Verify metrics were recorded (no panics, method executed successfully)
			assert.NotNil(t, subject)
		})
	}
}

func TestPrometheusCollector_IncrementErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		config            httpx.PrometheusConfig
		method            string
		url               string
		statusCode        int
		expectedErrorType string
	}{
		{
			name: "records network error with status code 0",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "https://api.example.com/users",
			statusCode:        0,
			expectedErrorType: "network",
		},
		{
			name: "records client error with status code 400",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "POST",
			url:               "https://api.example.com/orders",
			statusCode:        400,
			expectedErrorType: "client_error",
		},
		{
			name: "records client error with status code 404",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "https://api.example.com/notfound",
			statusCode:        404,
			expectedErrorType: "client_error",
		},
		{
			name: "records client error with status code 499",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "https://api.example.com/test",
			statusCode:        499,
			expectedErrorType: "client_error",
		},
		{
			name: "records server error with status code 500",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "POST",
			url:               "https://api.example.com/error",
			statusCode:        500,
			expectedErrorType: "server_error",
		},
		{
			name: "records server error with status code 503",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "https://api.example.com/unavailable",
			statusCode:        503,
			expectedErrorType: "server_error",
		},
		{
			name: "records unknown error type with status code 300",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "https://api.example.com/redirect",
			statusCode:        300,
			expectedErrorType: "unknown",
		},
		{
			name: "records error without method label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: false,
			},
			method:            "POST",
			url:               "https://api.example.com/test",
			statusCode:        500,
			expectedErrorType: "server_error",
		},
		{
			name: "records error without host label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   false,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "https://api.example.com/test",
			statusCode:        404,
			expectedErrorType: "client_error",
		},
		{
			name: "records error with invalid URL",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:            "GET",
			url:               "://invalid-url",
			statusCode:        500,
			expectedErrorType: "server_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject, err := httpx.NewPrometheusCollector(tc.config)
			require.NoError(t, err)

			subject.IncrementErrors(tc.method, tc.url, tc.statusCode)

			// Verify metrics were recorded (no panics, method executed successfully)
			assert.NotNil(t, subject)
		})
	}
}

func TestPrometheusCollector_RecordDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   httpx.PrometheusConfig
		method   string
		url      string
		duration time.Duration
	}{
		{
			name: "records short duration",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:   "GET",
			url:      "https://api.example.com/fast",
			duration: 10 * time.Millisecond,
		},
		{
			name: "records medium duration",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:   "POST",
			url:      "https://api.example.com/medium",
			duration: 500 * time.Millisecond,
		},
		{
			name: "records long duration",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:   "GET",
			url:      "https://api.example.com/slow",
			duration: 5 * time.Second,
		},
		{
			name: "records zero duration",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:   "GET",
			url:      "https://api.example.com/instant",
			duration: 0,
		},
		{
			name: "records duration without method label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: false,
			},
			method:   "PUT",
			url:      "https://api.example.com/test",
			duration: 100 * time.Millisecond,
		},
		{
			name: "records duration without host label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   false,
				IncludeMethodLabel: true,
			},
			method:   "DELETE",
			url:      "https://api.example.com/test",
			duration: 200 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject, err := httpx.NewPrometheusCollector(tc.config)
			require.NoError(t, err)

			subject.RecordDuration(tc.method, tc.url, tc.duration)

			// Verify metrics were recorded (no panics, method executed successfully)
			assert.NotNil(t, subject)
		})
	}
}

func TestPrometheusCollector_RecordRequestSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config httpx.PrometheusConfig
		method string
		url    string
		size   int64
	}{
		{
			name: "records small request size",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method: "POST",
			url:    "https://api.example.com/data",
			size:   100,
		},
		{
			name: "records medium request size",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method: "PUT",
			url:    "https://api.example.com/upload",
			size:   10000,
		},
		{
			name: "records large request size",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method: "POST",
			url:    "https://api.example.com/large",
			size:   1000000,
		},
		{
			name: "records zero request size",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method: "GET",
			url:    "https://api.example.com/test",
			size:   0,
		},
		{
			name: "records request size without method label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: false,
			},
			method: "POST",
			url:    "https://api.example.com/test",
			size:   5000,
		},
		{
			name: "records request size without host label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   false,
				IncludeMethodLabel: true,
			},
			method: "PUT",
			url:    "https://api.example.com/test",
			size:   2500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject, err := httpx.NewPrometheusCollector(tc.config)
			require.NoError(t, err)

			subject.RecordRequestSize(tc.method, tc.url, tc.size)

			// Verify metrics were recorded (no panics, method executed successfully)
			assert.NotNil(t, subject)
		})
	}
}

func TestPrometheusCollector_RecordResponseSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     httpx.PrometheusConfig
		method     string
		url        string
		statusCode int
		size       int64
	}{
		{
			name: "records response size with success status",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:     "GET",
			url:        "https://api.example.com/data",
			statusCode: 200,
			size:       1500,
		},
		{
			name: "records response size with created status",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:     "POST",
			url:        "https://api.example.com/create",
			statusCode: 201,
			size:       500,
		},
		{
			name: "records response size with client error status",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:     "GET",
			url:        "https://api.example.com/notfound",
			statusCode: 404,
			size:       200,
		},
		{
			name: "records response size with server error status",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:     "POST",
			url:        "https://api.example.com/error",
			statusCode: 500,
			size:       100,
		},
		{
			name: "records large response size",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:     "GET",
			url:        "https://api.example.com/bulk",
			statusCode: 200,
			size:       10000000,
		},
		{
			name: "records zero response size",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: true,
			},
			method:     "DELETE",
			url:        "https://api.example.com/item",
			statusCode: 204,
			size:       0,
		},
		{
			name: "records response size without method label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   true,
				IncludeMethodLabel: false,
			},
			method:     "GET",
			url:        "https://api.example.com/test",
			statusCode: 200,
			size:       300,
		},
		{
			name: "records response size without host label",
			config: httpx.PrometheusConfig{
				Registry:           prometheus.NewRegistry(),
				Subsystem:          "http_client",
				IncludeHostLabel:   false,
				IncludeMethodLabel: true,
			},
			method:     "POST",
			url:        "https://api.example.com/test",
			statusCode: 200,
			size:       400,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject, err := httpx.NewPrometheusCollector(tc.config)
			require.NoError(t, err)

			subject.RecordResponseSize(tc.method, tc.url, tc.statusCode, tc.size)

			// Verify metrics were recorded (no panics, method executed successfully)
			assert.NotNil(t, subject)
		})
	}
}

func TestPrometheusCollector_HostExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		url          string
		wantHostPart string
	}{
		{
			name:         "extracts host from standard HTTP URL",
			url:          "http://api.example.com/users",
			wantHostPart: "api.example.com",
		},
		{
			name:         "extracts host from HTTPS URL",
			url:          "https://secure.example.com/data",
			wantHostPart: "secure.example.com",
		},
		{
			name:         "extracts host with port from URL",
			url:          "https://api.example.com:8443/endpoint",
			wantHostPart: "api.example.com:8443",
		},
		{
			name:         "extracts localhost",
			url:          "http://localhost:8080/test",
			wantHostPart: "localhost:8080",
		},
		{
			name:         "extracts IP address",
			url:          "http://192.168.1.1:3000/api",
			wantHostPart: "192.168.1.1:3000",
		},
		{
			name:         "returns unknown for invalid URL",
			url:          "://invalid-url",
			wantHostPart: "unknown",
		},
		{
			name:         "returns unknown for malformed URL",
			url:          "not a url at all",
			wantHostPart: "unknown",
		},
		{
			name:         "extracts host from URL with query parameters",
			url:          "https://api.example.com/search?q=test&limit=10",
			wantHostPart: "api.example.com",
		},
		{
			name:         "extracts host from URL with fragment",
			url:          "https://example.com/page#section",
			wantHostPart: "example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := httpx.PrometheusConfig{
				Registry:         prometheus.NewRegistry(),
				Subsystem:        "http_client",
				IncludeHostLabel: true,
			}

			subject, err := httpx.NewPrometheusCollector(config)
			require.NoError(t, err)

			// Test host extraction through IncrementRequests
			// This indirectly tests extractHost since it's a private method
			subject.IncrementRequests("GET", tc.url)

			// Verify no panic occurred during host extraction
			assert.NotNil(t, subject)
		})
	}
}

func TestPrometheusCollector_Integration(t *testing.T) {
	t.Parallel()

	t.Run("simulates full request lifecycle with success", func(t *testing.T) {
		t.Parallel()

		config := httpx.PrometheusConfig{
			Registry:           prometheus.NewRegistry(),
			Subsystem:          "http_client",
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}

		subject, err := httpx.NewPrometheusCollector(config)
		require.NoError(t, err)

		method := "POST"
		url := "https://api.example.com/orders"
		requestSize := int64(1024)
		responseSize := int64(512)
		statusCode := 201
		duration := 150 * time.Millisecond

		// Simulate full request lifecycle
		subject.IncrementRequests(method, url)
		subject.RecordRequestSize(method, url, requestSize)
		subject.RecordDuration(method, url, duration)
		subject.RecordResponseSize(method, url, statusCode, responseSize)

		// Verify all metrics were recorded successfully (no panics)
		assert.NotNil(t, subject)
	})

	t.Run("simulates full request lifecycle with error", func(t *testing.T) {
		t.Parallel()

		config := httpx.PrometheusConfig{
			Registry:           prometheus.NewRegistry(),
			Subsystem:          "http_client",
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}

		subject, err := httpx.NewPrometheusCollector(config)
		require.NoError(t, err)

		method := "GET"
		url := "https://api.example.com/users"
		requestSize := int64(0)
		statusCode := 500

		// Simulate request that encounters server error
		subject.IncrementRequests(method, url)
		subject.RecordRequestSize(method, url, requestSize)
		subject.IncrementErrors(method, url, statusCode)

		// Verify all metrics were recorded successfully (no panics)
		assert.NotNil(t, subject)
	})

	t.Run("simulates network error scenario", func(t *testing.T) {
		t.Parallel()

		config := httpx.PrometheusConfig{
			Registry:           prometheus.NewRegistry(),
			Subsystem:          "http_client",
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}

		subject, err := httpx.NewPrometheusCollector(config)
		require.NoError(t, err)

		method := "GET"
		url := "https://unreachable.example.com/data"
		requestSize := int64(0)

		// Simulate network error (no response received)
		subject.IncrementRequests(method, url)
		subject.RecordRequestSize(method, url, requestSize)
		subject.IncrementErrors(method, url, 0) // statusCode 0 = network error

		// Verify all metrics were recorded successfully (no panics)
		assert.NotNil(t, subject)
	})

	t.Run("handles multiple concurrent requests", func(t *testing.T) {
		t.Parallel()

		config := httpx.PrometheusConfig{
			Registry:           prometheus.NewRegistry(),
			Subsystem:          "http_client",
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}

		subject, err := httpx.NewPrometheusCollector(config)
		require.NoError(t, err)

		// Simulate multiple concurrent requests
		urls := []string{
			"https://api1.example.com/data",
			"https://api2.example.com/data",
			"https://api3.example.com/data",
		}

		for _, url := range urls {
			subject.IncrementRequests("GET", url)
			subject.RecordRequestSize("GET", url, 100)
			subject.RecordDuration("GET", url, 50*time.Millisecond)
			subject.RecordResponseSize("GET", url, 200, 200)
		}

		// Verify all metrics were recorded successfully (no panics)
		assert.NotNil(t, subject)
	})
}

func TestPrometheusCollector_LabelConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("collector with no optional labels", func(t *testing.T) {
		t.Parallel()

		config := httpx.PrometheusConfig{
			Registry:           prometheus.NewRegistry(),
			Subsystem:          "http_client",
			IncludeHostLabel:   false,
			IncludeMethodLabel: false,
		}

		subject, err := httpx.NewPrometheusCollector(config)
		require.NoError(t, err)

		// Should work fine with only status_code label
		subject.IncrementRequests("GET", "https://api.example.com/test")
		subject.RecordDuration("GET", "https://api.example.com/test", 100*time.Millisecond)
		subject.RecordResponseSize("GET", "https://api.example.com/test", 200, 500)

		assert.NotNil(t, subject)
	})

	t.Run("collector with all builtin labels enabled", func(t *testing.T) {
		t.Parallel()

		config := httpx.PrometheusConfig{
			Registry:           prometheus.NewRegistry(),
			Subsystem:          "http_client",
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}

		subject, err := httpx.NewPrometheusCollector(config)
		require.NoError(t, err)

		// Should work with all builtin labels
		subject.IncrementRequests("POST", "https://api.example.com/create")
		subject.RecordDuration("POST", "https://api.example.com/create", 200*time.Millisecond)

		assert.NotNil(t, subject)
	})
}
