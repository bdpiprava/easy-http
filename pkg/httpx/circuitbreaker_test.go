package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestCircuitBreakerConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()

		assert.Equal(t, "default", config.Name)
		assert.Equal(t, uint32(1), config.MaxRequests)
		assert.Equal(t, 60*time.Second, config.Interval)
		assert.Equal(t, 60*time.Second, config.Timeout)
		assert.NotNil(t, config.ReadyToTrip)
		assert.NotNil(t, config.OnStateChange)
		assert.NotNil(t, config.IsSuccessful)
	})

	t.Run("aggressive config", func(t *testing.T) {
		config := httpx.AggressiveCircuitBreakerConfig()

		assert.Equal(t, "aggressive", config.Name)
		assert.Equal(t, uint32(1), config.MaxRequests)
		assert.Equal(t, 30*time.Second, config.Interval)
		assert.Equal(t, 30*time.Second, config.Timeout)

		// Test aggressive ReadyToTrip function
		counts := httpx.Counts{Requests: 3, TotalFailures: 1} // 33% failure rate
		assert.True(t, config.ReadyToTrip(counts))
	})

	t.Run("conservative config", func(t *testing.T) {
		config := httpx.ConservativeCircuitBreakerConfig()

		assert.Equal(t, "conservative", config.Name)
		assert.Equal(t, uint32(3), config.MaxRequests)
		assert.Equal(t, 120*time.Second, config.Interval)
		assert.Equal(t, 120*time.Second, config.Timeout)

		// Test conservative ReadyToTrip function
		counts := httpx.Counts{Requests: 10, TotalFailures: 7} // 70% failure rate
		assert.False(t, config.ReadyToTrip(counts))            // Should need 80%+

		counts.TotalFailures = 8 // 80% failure rate
		assert.True(t, config.ReadyToTrip(counts))
	})
}

func TestCounts(t *testing.T) {
	t.Run("request counting", func(t *testing.T) {
		var counts httpx.Counts

		assert.Equal(t, uint32(0), counts.Requests)

		counts.OnRequest()
		assert.Equal(t, uint32(1), counts.Requests)

		counts.OnRequest()
		assert.Equal(t, uint32(2), counts.Requests)
	})

	t.Run("success counting", func(t *testing.T) {
		var counts httpx.Counts

		counts.OnRequest()
		counts.OnSuccess()

		assert.Equal(t, uint32(1), counts.TotalSuccesses)
		assert.Equal(t, uint32(1), counts.ConsecutiveSuccesses)
		assert.Equal(t, uint32(0), counts.ConsecutiveFailures)

		counts.OnRequest()
		counts.OnSuccess()

		assert.Equal(t, uint32(2), counts.TotalSuccesses)
		assert.Equal(t, uint32(2), counts.ConsecutiveSuccesses)
	})

	t.Run("failure counting", func(t *testing.T) {
		var counts httpx.Counts

		counts.OnRequest()
		counts.OnFailure()

		assert.Equal(t, uint32(1), counts.TotalFailures)
		assert.Equal(t, uint32(1), counts.ConsecutiveFailures)
		assert.Equal(t, uint32(0), counts.ConsecutiveSuccesses)

		counts.OnRequest()
		counts.OnFailure()

		assert.Equal(t, uint32(2), counts.TotalFailures)
		assert.Equal(t, uint32(2), counts.ConsecutiveFailures)
	})

	t.Run("mixed success and failure", func(t *testing.T) {
		var counts httpx.Counts

		// Success resets consecutive failures
		counts.OnRequest()
		counts.OnFailure()
		counts.OnRequest()
		counts.OnSuccess()

		assert.Equal(t, uint32(2), counts.Requests)
		assert.Equal(t, uint32(1), counts.TotalSuccesses)
		assert.Equal(t, uint32(1), counts.TotalFailures)
		assert.Equal(t, uint32(1), counts.ConsecutiveSuccesses)
		assert.Equal(t, uint32(0), counts.ConsecutiveFailures)
	})

	t.Run("clear counts", func(t *testing.T) {
		var counts httpx.Counts

		counts.OnRequest()
		counts.OnSuccess()
		counts.OnRequest()
		counts.OnFailure()

		counts.Clear()

		assert.Equal(t, uint32(0), counts.Requests)
		assert.Equal(t, uint32(0), counts.TotalSuccesses)
		assert.Equal(t, uint32(0), counts.TotalFailures)
		assert.Equal(t, uint32(0), counts.ConsecutiveSuccesses)
		assert.Equal(t, uint32(0), counts.ConsecutiveFailures)
	})
}

func TestCircuitBreakerStates(t *testing.T) {
	t.Run("initial state is closed", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		cb := httpx.NewCircuitBreakerMiddleware(config)

		assert.Equal(t, httpx.StateClosed, cb.State())
	})

	t.Run("state transitions on failures", func(t *testing.T) {
		var stateChanges []httpx.CircuitBreakerState
		var mu sync.Mutex

		config := httpx.DefaultCircuitBreakerConfig()
		config.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 2 // Trip after 2 failures
		}
		config.OnStateChange = func(_ string, _, to httpx.CircuitBreakerState) {
			mu.Lock()
			stateChanges = append(stateChanges, to)
			mu.Unlock()
		}

		cb := httpx.NewCircuitBreakerMiddleware(config)

		// Create a mock next function that always fails
		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			return nil, errors.New("service error")
		}

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		// First failure - should remain closed
		resp, _ := cb.Execute(ctx, req, next)
		defer closeSafely(resp)
		assert.Equal(t, httpx.StateClosed, cb.State())

		// Second failure - should trip to open
		resp, _ = cb.Execute(ctx, req, next)
		defer closeSafely(resp)
		assert.Equal(t, httpx.StateOpen, cb.State())

		mu.Lock()
		assert.Contains(t, stateChanges, httpx.StateOpen)
		mu.Unlock()
	})

	t.Run("open state rejects requests", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		config.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 1 // Trip immediately
		}

		cb := httpx.NewCircuitBreakerMiddleware(config)

		// Cause the circuit breaker to trip
		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			return nil, errors.New("service error")
		}

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		// First request fails and trips the circuit breaker
		resp, _ := cb.Execute(ctx, req, next)
		defer closeSafely(resp)
		assert.Equal(t, httpx.StateOpen, cb.State())

		// Second request should be rejected immediately
		resp, err := cb.Execute(ctx, req, next)
		defer closeSafely(resp)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.True(t, httpx.IsCircuitBreakerError(err))
	})

	t.Run("half-open state allows limited requests", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		config.Timeout = 10 * time.Millisecond // Short timeout for testing
		config.MaxRequests = 1
		config.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 1
		}

		cb := httpx.NewCircuitBreakerMiddleware(config)

		// Trip the circuit breaker
		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			return nil, errors.New("service error")
		}

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		resp, _ := cb.Execute(ctx, req, next)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		assert.Equal(t, httpx.StateOpen, cb.State())

		// Wait for timeout to pass
		time.Sleep(20 * time.Millisecond)

		// Create a successful next function
		successNext := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200}, nil
		}

		// Request should succeed and circuit breaker should close
		resp, err := cb.Execute(ctx, req, successNext)
		if err == nil && resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, httpx.StateClosed, cb.State())
	})
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	t.Run("middleware name", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		config.Name = "test_cb"
		cb := httpx.NewCircuitBreakerMiddleware(config)

		assert.Equal(t, "circuit_breaker_test_cb", cb.Name())
	})

	t.Run("successful requests", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		cb := httpx.NewCircuitBreakerMiddleware(config)

		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200}, nil
		}

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		for range 5 {
			resp, err := cb.Execute(ctx, req, next)
			if err == nil && resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, httpx.StateClosed, cb.State())
		}

		counts := cb.Counts()
		assert.Equal(t, uint32(5), counts.Requests)
		assert.Equal(t, uint32(5), counts.TotalSuccesses)
		assert.Equal(t, uint32(0), counts.TotalFailures)
	})

	t.Run("integration with existing error types", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		config.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 1
		}

		cb := httpx.NewCircuitBreakerMiddleware(config)

		// Test with HTTPError
		next := func(_ context.Context, req *http.Request) (*http.Response, error) {
			return nil, httpx.NetworkError("connection failed", nil, req)
		}

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		// First request should fail and trip circuit breaker
		resp, err := cb.Execute(ctx, req, next)
		defer closeSafely(resp)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.True(t, httpx.IsNetworkError(err))
		assert.Equal(t, httpx.StateOpen, cb.State())

		// Second request should be blocked by circuit breaker
		resp, err = cb.Execute(ctx, req, next)
		defer closeSafely(resp)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.True(t, httpx.IsCircuitBreakerError(err))
	})
}

func TestCircuitBreakerWithServer(t *testing.T) {
	t.Run("circuit breaker with real HTTP server", func(t *testing.T) {
		failureCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			failureCount++
			if failureCount <= 2 { // Only first 2 requests fail
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "server error"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message": "success"}`))
		}))
		defer server.Close()

		// Configure circuit breaker to trip after 2 failures
		config := httpx.DefaultCircuitBreakerConfig()
		config.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 2
		}
		config.Timeout = 50 * time.Millisecond // Short timeout for testing

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientCircuitBreaker(config),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))

		// First request - fails (500 error)
		response1, err1 := client.Execute(*req, map[string]any{})
		require.NoError(t, err1) // HTTP errors don't cause Execute to fail
		assert.Equal(t, http.StatusInternalServerError, response1.StatusCode)

		// Second request - fails and trips circuit breaker
		response2, err2 := client.Execute(*req, map[string]any{})
		assert.NoError(t, err2)
		assert.Equal(t, http.StatusInternalServerError, response2.StatusCode)

		// Third request - should be blocked by circuit breaker
		response3, err3 := client.Execute(*req, map[string]any{})
		assert.Error(t, err3)
		assert.True(t, httpx.IsCircuitBreakerError(err3))
		assert.Nil(t, response3)

		// Wait for circuit breaker timeout
		time.Sleep(100 * time.Millisecond)

		// Fourth request - should succeed as server now returns success (failureCount=3)
		response4, err4 := client.Execute(*req, map[string]any{})
		assert.NoError(t, err4)
		assert.NotNil(t, response4)
		assert.Equal(t, http.StatusOK, response4.StatusCode)
	})
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	t.Run("concurrent requests", func(t *testing.T) {
		config := httpx.DefaultCircuitBreakerConfig()
		config.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 5
		}

		cb := httpx.NewCircuitBreakerMiddleware(config)

		requestCount := 0
		var mu sync.Mutex

		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			mu.Lock()
			requestCount++
			count := requestCount
			mu.Unlock()

			// First 5 requests fail, rest succeed
			if count <= 5 {
				return nil, errors.New("service error")
			}
			return &http.Response{StatusCode: 200}, nil
		}

		ctx := context.Background()

		// Launch 10 concurrent requests
		var wg sync.WaitGroup
		var results sync.Map

		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
				resp, err := cb.Execute(ctx, req, next)
				defer closeSafely(resp)

				results.Store(id, map[string]interface{}{
					"response": resp,
					"error":    err,
				})
			}(i)
		}

		wg.Wait()

		// Check results
		var successCount, errorCount, circuitBreakerErrorCount int

		results.Range(func(_, value interface{}) bool {
			result := value.(map[string]interface{})
			resp := result["response"]
			err := result["error"]

			if err != nil {
				errorCount++
				if httpx.IsCircuitBreakerError(err.(error)) {
					circuitBreakerErrorCount++
				}
			} else if resp != nil {
				successCount++
			}
			return true
		})

		// Should have some failures and/or circuit breaker blocks
		assert.Positive(t, errorCount, "Should have some failed requests")
		// Note: successCount might be 0 if circuit breaker trips quickly

		// Verify circuit breaker was activated
		assert.Equal(t, httpx.StateOpen, cb.State())
	})
}

func TestCircuitBreakerError(t *testing.T) {
	t.Run("circuit breaker error creation", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)
		err := httpx.CircuitBreakerError("circuit breaker is open", req)

		assert.Equal(t, httpx.ErrorTypeMiddleware, err.Type)
		assert.Contains(t, err.Message, "circuit breaker is open")
		assert.Equal(t, req, err.Request)
	})

	t.Run("is circuit breaker error detection", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)

		// Circuit breaker error
		cbErr := httpx.CircuitBreakerError("circuit breaker is open", req)
		assert.True(t, httpx.IsCircuitBreakerError(cbErr))

		// Regular error
		regularErr := errors.New("regular error")
		assert.False(t, httpx.IsCircuitBreakerError(regularErr))

		// Other HTTP error
		networkErr := httpx.NetworkError("network error", nil, req)
		assert.False(t, httpx.IsCircuitBreakerError(networkErr))
	})
}

func TestCircuitBreakerIntegrationWithRetry(t *testing.T) {
	t.Run("circuit breaker with retry policy", func(t *testing.T) {
		failureCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			failureCount++
			if failureCount == 1 {
				// First attempt fails
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Retry succeeds
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
		}))
		defer server.Close()

		// Configure both circuit breaker and retry
		cbConfig := httpx.DefaultCircuitBreakerConfig()
		cbConfig.ReadyToTrip = func(counts httpx.Counts) bool {
			return counts.TotalFailures >= 3 // Allow a few failures before tripping
		}

		retryPolicy := httpx.DefaultRetryPolicy()
		retryPolicy.MaxAttempts = 2 // First attempt + 1 retry

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientCircuitBreaker(cbConfig),
			httpx.WithClientRetryPolicy(retryPolicy),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		response, err := client.Execute(*req, map[string]any{})

		// Should succeed after retries
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		// Circuit breaker should still be closed since retry succeeded
		// Note: We can't directly access the circuit breaker state from the client,
		// but we can infer it worked correctly since the request succeeded
	})
}

func closeSafely(closable *http.Response) {
	if closable == nil || closable.Body == nil {
		return
	}

	_ = closable.Body.Close()
}
