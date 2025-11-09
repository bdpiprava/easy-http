package httpx_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

// TestMiddleware is a simple test middleware for testing
type TestMiddleware struct {
	name     string
	executed bool
	mu       sync.Mutex
}

func NewTestMiddleware(name string) *TestMiddleware {
	return &TestMiddleware{name: name}
}

func (m *TestMiddleware) Name() string {
	return m.name
}

func (m *TestMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	m.mu.Lock()
	m.executed = true
	m.mu.Unlock()

	// Add a custom header to track execution
	req.Header.Set("X-Middleware-"+m.name, "executed")

	return next(ctx, req)
}

func (m *TestMiddleware) WasExecuted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executed
}

func (m *TestMiddleware) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = false
}

// MockMetricsCollector for testing metrics middleware
type MockMetricsCollector struct {
	requests  map[string]int
	errors    map[string]int
	durations map[string]time.Duration
	mu        sync.Mutex
}

func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{
		requests:  make(map[string]int),
		errors:    make(map[string]int),
		durations: make(map[string]time.Duration),
	}
}

func (m *MockMetricsCollector) IncrementRequests(method, url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + url
	m.requests[key]++
}

func (m *MockMetricsCollector) IncrementErrors(method, url string, _ int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + url
	m.errors[key]++
}

func (m *MockMetricsCollector) RecordDuration(method, url string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + url
	m.durations[key] = duration
}

func (m *MockMetricsCollector) GetRequests(method, url string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + url
	return m.requests[key]
}

func (m *MockMetricsCollector) GetErrors(method, url string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + " " + url
	return m.errors[key]
}

func TestMiddlewareChain(t *testing.T) {
	t.Run("basic middleware chain execution", func(t *testing.T) {
		// Setup test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Echo back headers set by middlewares
			headers := make(map[string]string)
			for key, values := range r.Header {
				if strings.HasPrefix(key, "X-Middleware-") {
					headers[key] = values[0]
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"middleware_headers": "received"}`))
		}))
		defer server.Close()

		// Create middlewares
		middleware1 := NewTestMiddleware("Test1")
		middleware2 := NewTestMiddleware("Test2")

		// Create client with middlewares
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware1),
			httpx.WithClientMiddleware(middleware2),
		)

		type TestResponse struct {
			MiddlewareHeaders string `json:"middleware_headers"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		response, err := client.Execute(*req, TestResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		// Verify middlewares were executed
		assert.True(t, middleware1.WasExecuted())
		assert.True(t, middleware2.WasExecuted())

		result, ok := response.Body.(TestResponse)
		require.True(t, ok)
		assert.Equal(t, "received", result.MiddlewareHeaders)
	})

	t.Run("middleware execution order", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check the order of middleware headers
			test1Header := r.Header.Get("X-Middleware-Test1")
			test2Header := r.Header.Get("X-Middleware-Test2")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"test1": "` + test1Header + `", "test2": "` + test2Header + `"}`))
		}))
		defer server.Close()

		// Create client with multiple middlewares
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddlewares(
				NewTestMiddleware("Test1"),
				NewTestMiddleware("Test2"),
			),
		)

		type OrderResponse struct {
			Test1 string `json:"test1"`
			Test2 string `json:"test2"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/order"))
		response, err := client.Execute(*req, OrderResponse{})

		require.NoError(t, err)
		result, ok := response.Body.(OrderResponse)
		require.True(t, ok)

		// Both middlewares should have been executed
		assert.Equal(t, "executed", result.Test1)
		assert.Equal(t, "executed", result.Test2)
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("logging middleware captures requests and responses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"logged": true}`))
		}))
		defer server.Close()

		// Create logger that writes to buffer
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

		// Create client with logging middleware
		loggingMiddleware := httpx.NewLoggingMiddleware(logger, slog.LevelDebug)
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(loggingMiddleware),
		)

		type LogResponse struct {
			Logged bool `json:"logged"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/logged"))
		response, err := client.Execute(*req, LogResponse{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		// Verify logging occurred
		logOutput := logBuffer.String()
		assert.Contains(t, logOutput, "HTTP request")
		assert.Contains(t, logOutput, "HTTP response")
		assert.Contains(t, logOutput, "GET")
		assert.Contains(t, logOutput, "/logged")
	})
}

func TestRetryMiddleware(t *testing.T) {
	t.Run("retry middleware retries on server errors", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "server error"}`))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"attempt": ` + string(rune(attemptCount+'0')) + `}`))
		}))
		defer server.Close()

		// Create client with retry middleware
		retryConfig := httpx.DefaultRetryConfig()
		retryConfig.MaxRetries = 3
		retryConfig.BaseDelay = 10 * time.Millisecond
		retryMiddleware := httpx.NewRetryMiddleware(retryConfig)

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(retryMiddleware),
		)

		type RetryResponse struct {
			Attempt int `json:"attempt"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/retry"))
		response, err := client.Execute(*req, RetryResponse{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, 3, attemptCount) // Should have made 3 attempts

		result, ok := response.Body.(RetryResponse)
		require.True(t, ok)
		assert.Equal(t, 3, result.Attempt)
	})

	t.Run("retry middleware stops retrying on client errors", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			w.WriteHeader(http.StatusBadRequest) // 4xx error should not retry
			_, _ = w.Write([]byte(`{"error": "bad request"}`))
		}))
		defer server.Close()

		// Create client with retry middleware
		retryConfig := httpx.DefaultRetryConfig()
		retryConfig.MaxRetries = 3
		retryMiddleware := httpx.NewRetryMiddleware(retryConfig)

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(retryMiddleware),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/no-retry"))
		response, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.Equal(t, 1, attemptCount) // Should have made only 1 attempt
	})
}

func TestMetricsMiddleware(t *testing.T) {
	t.Run("metrics middleware collects request metrics", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"metrics": "collected"}`))
		}))
		defer server.Close()

		// Create metrics collector
		collector := NewMockMetricsCollector()
		metricsMiddleware := httpx.NewMetricsMiddleware(collector)

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(metricsMiddleware),
		)

		type MetricsResponse struct {
			Metrics string `json:"metrics"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/metrics"))
		response, err := client.Execute(*req, MetricsResponse{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		// Verify metrics were collected
		assert.Equal(t, 1, collector.GetRequests("GET", server.URL+"/metrics"))
		assert.Equal(t, 0, collector.GetErrors("GET", server.URL+"/metrics"))
	})
}

func TestUserAgentMiddleware(t *testing.T) {
	t.Run("user agent middleware sets user agent header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userAgent := r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"user_agent": "` + userAgent + `"}`))
		}))
		defer server.Close()

		// Create client with user agent middleware
		userAgentMiddleware := httpx.NewUserAgentMiddleware("TestClient/1.0", false)
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(userAgentMiddleware),
		)

		type UAResponse struct {
			UserAgent string `json:"user_agent"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/ua"))
		response, err := client.Execute(*req, UAResponse{})

		require.NoError(t, err)
		result, ok := response.Body.(UAResponse)
		require.True(t, ok)
		assert.Equal(t, "TestClient/1.0", result.UserAgent)
	})
}

func TestMiddlewareWithLegacyClient(t *testing.T) {
	t.Run("legacy client without middleware works normally", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"legacy": true}`))
		}))
		defer server.Close()

		// Create old-style client (no middlewares)

		client := httpx.NewClient(
			httpx.WithDefaultBaseURL(server.URL),
		)

		type LegacyResponse struct {
			Legacy bool `json:"legacy"`
		}

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/legacy"))
		response, err := client.Execute(*req, LegacyResponse{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(LegacyResponse)
		require.True(t, ok)
		assert.True(t, result.Legacy)
	})
}
