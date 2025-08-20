package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestRetryPolicies(t *testing.T) {
	t.Run("default retry policy", func(t *testing.T) {
		policy := httpx.DefaultRetryPolicy()

		assert.Equal(t, 3, policy.MaxAttempts)
		assert.Equal(t, 100*time.Millisecond, policy.BaseDelay)
		assert.Equal(t, 5*time.Second, policy.MaxDelay)
		assert.Equal(t, httpx.RetryStrategyExponential, policy.Strategy)
		assert.InEpsilon(t, 2.0, policy.Multiplier, 0.01)
		assert.Contains(t, policy.RetryableStatusCodes, 500)
		assert.Contains(t, policy.RetryableStatusCodes, 503)
		assert.Contains(t, policy.RetryableErrorTypes, httpx.ErrorTypeNetwork)
	})

	t.Run("aggressive retry policy", func(t *testing.T) {
		policy := httpx.AggressiveRetryPolicy()

		assert.Equal(t, 5, policy.MaxAttempts)
		assert.Equal(t, httpx.RetryStrategyExponentialJitter, policy.Strategy)
		assert.Contains(t, policy.RetryableStatusCodes, 408) // Request timeout
		assert.Contains(t, policy.RetryableErrorTypes, httpx.ErrorTypeServer)
	})

	t.Run("conservative retry policy", func(t *testing.T) {
		policy := httpx.ConservativeRetryPolicy()

		assert.Equal(t, 2, policy.MaxAttempts)
		assert.Equal(t, httpx.RetryStrategyFixed, policy.Strategy)
		assert.Equal(t, []int{503, 504}, policy.RetryableStatusCodes)
		assert.Equal(t, []httpx.ErrorType{httpx.ErrorTypeNetwork}, policy.RetryableErrorTypes)
	})
}

func TestRetryConditions(t *testing.T) {
	tests := []struct {
		name        string
		condition   httpx.RetryCondition
		attempt     int
		err         error
		resp        *http.Response
		shouldRetry bool
	}{
		{
			name:        "default condition - network error should retry",
			condition:   httpx.AdvancedDefaultRetryCondition,
			attempt:     1,
			err:         httpx.NetworkError("connection failed", nil, nil),
			shouldRetry: true,
		},
		{
			name:        "default condition - timeout error should retry",
			condition:   httpx.AdvancedDefaultRetryCondition,
			attempt:     1,
			err:         httpx.TimeoutError("request timeout", nil, nil),
			shouldRetry: true,
		},
		{
			name:        "default condition - server error should retry",
			condition:   httpx.AdvancedDefaultRetryCondition,
			attempt:     1,
			resp:        &http.Response{StatusCode: 500},
			shouldRetry: true,
		},
		{
			name:        "default condition - client error should not retry",
			condition:   httpx.AdvancedDefaultRetryCondition,
			attempt:     1,
			resp:        &http.Response{StatusCode: 404},
			shouldRetry: false,
		},
		{
			name:        "default condition - too many attempts should not retry",
			condition:   httpx.AdvancedDefaultRetryCondition,
			attempt:     5,
			err:         httpx.NetworkError("connection failed", nil, nil),
			shouldRetry: false,
		},
		{
			name:        "aggressive condition - allows more attempts",
			condition:   httpx.AggressiveRetryCondition,
			attempt:     4,
			err:         httpx.NetworkError("connection failed", nil, nil),
			shouldRetry: true,
		},
		{
			name:        "conservative condition - limits attempts",
			condition:   httpx.ConservativeRetryCondition,
			attempt:     2,
			err:         httpx.NetworkError("connection failed", nil, nil),
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.condition(tt.attempt, tt.err, tt.resp)
			assert.Equal(t, tt.shouldRetry, result)
		})
	}
}

func TestAdvancedRetryMiddleware(t *testing.T) {
	t.Run("retry on network error", func(t *testing.T) {
		attemptCount := 0
		maxAttempts := 3

		middleware := httpx.NewAdvancedRetryMiddleware(httpx.RetryPolicy{
			MaxAttempts: maxAttempts,
			BaseDelay:   10 * time.Millisecond, // Short delay for testing
			MaxDelay:    100 * time.Millisecond,
			Strategy:    httpx.RetryStrategyFixed,
			Condition: func(attempt int, err error, _ *http.Response) bool {
				return attempt < maxAttempts-1 && err != nil
			},
		})

		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			attemptCount++
			if attemptCount < maxAttempts {
				return nil, errors.New("network error")
			}
			return &http.Response{StatusCode: 200}, nil
		}

		ctx := context.Background()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		resp, err := middleware.Execute(ctx, req, next)
		defer closeSafely(resp)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, maxAttempts, attemptCount)
	})

	t.Run("retry on server error status codes", func(t *testing.T) {
		attemptCount := 0

		middleware := httpx.NewAdvancedRetryMiddleware(httpx.RetryPolicy{
			MaxAttempts:          3,
			BaseDelay:            10 * time.Millisecond,
			MaxDelay:             100 * time.Millisecond,
			Strategy:             httpx.RetryStrategyFixed,
			RetryableStatusCodes: []int{503},
		})

		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			attemptCount++
			if attemptCount < 3 {
				return &http.Response{StatusCode: 503}, nil
			}
			return &http.Response{StatusCode: 200}, nil
		}

		ctx := context.Background()
		req, _ := http.NewRequest("GET", "http://example.com", nil)

		resp, err := middleware.Execute(ctx, req, next)
		defer closeSafely(resp)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 3, attemptCount)
	})

	t.Run("stop retrying after max attempts", func(t *testing.T) {
		attemptCount := 0
		maxAttempts := 2

		middleware := httpx.NewAdvancedRetryMiddleware(httpx.RetryPolicy{
			MaxAttempts: maxAttempts,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
			Strategy:    httpx.RetryStrategyFixed,
			Condition: func(_ int, err error, _ *http.Response) bool {
				return err != nil
			},
		})

		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			attemptCount++
			return nil, errors.New("persistent error")
		}

		ctx := context.Background()
		req, _ := http.NewRequest("GET", "http://example.com", nil)

		resp, err := middleware.Execute(ctx, req, next)
		defer closeSafely(resp)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, maxAttempts, attemptCount)
	})

	t.Run("context cancellation stops retries", func(t *testing.T) {
		attemptCount := 0

		middleware := httpx.NewAdvancedRetryMiddleware(httpx.RetryPolicy{
			MaxAttempts: 5,
			BaseDelay:   100 * time.Millisecond, // Longer delay to test cancellation
			Strategy:    httpx.RetryStrategyFixed,
			Condition: func(_ int, err error, _ *http.Response) bool {
				return err != nil
			},
		})

		next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
			attemptCount++
			return nil, errors.New("error")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		req, _ := http.NewRequest("GET", "http://example.com", nil)

		start := time.Now()
		resp, err := middleware.Execute(ctx, req, next)
		defer closeSafely(resp)
		duration := time.Since(start)

		require.Error(t, err)
		assert.Nil(t, resp)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, duration, 200*time.Millisecond) // Should be cancelled before second retry
		assert.Equal(t, 1, attemptCount)               // Should only attempt once before context cancellation
	})
}

func TestRetryStrategies(t *testing.T) {
	testCases := []struct {
		name            string
		strategy        httpx.RetryStrategy
		baseDelay       time.Duration
		maxDelay        time.Duration
		multiplier      float64
		attempt         int
		expectedMinimum time.Duration
		expectedMaximum time.Duration
	}{
		{
			name:            "fixed strategy",
			strategy:        httpx.RetryStrategyFixed,
			baseDelay:       100 * time.Millisecond,
			maxDelay:        1 * time.Second,
			attempt:         3,
			expectedMinimum: 100 * time.Millisecond,
			expectedMaximum: 100 * time.Millisecond,
		},
		{
			name:            "linear strategy",
			strategy:        httpx.RetryStrategyLinear,
			baseDelay:       100 * time.Millisecond,
			maxDelay:        1 * time.Second,
			multiplier:      2.0,
			attempt:         2,                      // Third attempt (0-indexed)
			expectedMinimum: 600 * time.Millisecond, // 100 * (2+1) * 2.0
			expectedMaximum: 600 * time.Millisecond,
		},
		{
			name:            "exponential strategy",
			strategy:        httpx.RetryStrategyExponential,
			baseDelay:       100 * time.Millisecond,
			maxDelay:        1 * time.Second,
			multiplier:      2.0,
			attempt:         2,                      // Third attempt
			expectedMinimum: 400 * time.Millisecond, // 100 * 2^2
			expectedMaximum: 400 * time.Millisecond,
		},
		{
			name:            "exponential with max delay cap",
			strategy:        httpx.RetryStrategyExponential,
			baseDelay:       100 * time.Millisecond,
			maxDelay:        300 * time.Millisecond,
			multiplier:      2.0,
			attempt:         4,                      // Fifth attempt would be 100 * 2^4 = 1600ms
			expectedMinimum: 300 * time.Millisecond, // Capped at maxDelay
			expectedMaximum: 300 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			middleware := httpx.NewAdvancedRetryMiddleware(httpx.RetryPolicy{
				MaxAttempts: 5,
				BaseDelay:   tc.baseDelay,
				MaxDelay:    tc.maxDelay,
				Strategy:    tc.strategy,
				Multiplier:  tc.multiplier,
			})

			// Use reflection to access the private calculateDelay method indirectly
			// by creating a test scenario that measures actual delays
			attemptCount := 0
			var measuredDelay time.Duration

			next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
				attemptCount++
				if attemptCount == 1 {
					return nil, errors.New("first error")
				}
				if attemptCount == 2 {
					// This should trigger the delay calculation for the second attempt
					return nil, errors.New("second error")
				}
				return &http.Response{StatusCode: 200}, nil
			}

			ctx := context.Background()
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			start := time.Now()
			resp, _ := middleware.Execute(ctx, req, next)
			defer closeSafely(resp)
			measuredDelay = time.Since(start)

			// For strategies other than jitter, we can verify the delay more precisely
			if tc.strategy != httpx.RetryStrategyExponentialJitter {
				// Allow generous tolerance for timing variations in tests
				tolerance := 200 * time.Millisecond
				assert.GreaterOrEqual(t, measuredDelay, tc.expectedMinimum-tolerance)
				assert.LessOrEqual(t, measuredDelay, tc.expectedMaximum+tolerance)
			}
		})
	}
}

func TestRetryWithJitter(t *testing.T) {
	t.Run("exponential jitter adds randomness", func(t *testing.T) {
		policy := httpx.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			Strategy:    httpx.RetryStrategyExponentialJitter,
			Multiplier:  2.0,
			JitterMax:   50 * time.Millisecond,
		}

		middleware := httpx.NewAdvancedRetryMiddleware(policy)

		// Run multiple times to verify jitter creates different delays
		delays := make([]time.Duration, 5)

		for i := range 5 {
			attemptCount := 0
			next := func(_ context.Context, _ *http.Request) (*http.Response, error) {
				attemptCount++
				if attemptCount < 2 {
					return nil, errors.New("error")
				}
				return &http.Response{StatusCode: 200}, nil
			}

			ctx := context.Background()
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			start := time.Now()
			resp, _ := middleware.Execute(ctx, req, next)
			defer closeSafely(resp)
			delays[i] = time.Since(start)
		}

		// Verify that we get different delays (jitter is working)
		// Note: There's a small chance all delays could be identical, but it's extremely unlikely
		uniqueDelays := make(map[time.Duration]bool)
		for _, delay := range delays {
			uniqueDelays[delay] = true
		}

		// We should have at least 2 different delays out of 5 runs
		assert.GreaterOrEqual(t, len(uniqueDelays), 2, "Jitter should create different delays")
	})
}

func TestRetryableError(t *testing.T) {
	t.Run("retryable error wrapping", func(t *testing.T) {
		originalErr := errors.New("original error")
		retryableErr := httpx.WrapAsRetryable(originalErr, 100*time.Millisecond)

		assert.True(t, httpx.IsRetryable(retryableErr))
		assert.ErrorIs(t, retryableErr, originalErr)
		assert.Contains(t, retryableErr.Error(), "retryable error")
		assert.Contains(t, retryableErr.Error(), "suggested delay: 100ms")
	})

	t.Run("nil error wrapping", func(t *testing.T) {
		retryableErr := httpx.WrapAsRetryable(nil, 100*time.Millisecond)
		assert.NoError(t, retryableErr)
	})

	t.Run("regular error is not retryable", func(t *testing.T) {
		regularErr := errors.New("regular error")
		assert.False(t, httpx.IsRetryable(regularErr))
	})
}

func TestClientWithRetryPolicy(t *testing.T) {
	t.Run("client with default retry policy", func(t *testing.T) {
		// Create a server that fails twice then succeeds
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "server error"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message": "success"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultRetryPolicy(),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		response, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, 3, attemptCount) // Should have retried twice

		body, ok := response.Body.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "success", body["message"])
	})

	t.Run("client with custom retry policy", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": "service unavailable"}`))
		}))
		defer server.Close()

		customPolicy := httpx.RetryPolicy{
			MaxAttempts:          2, // Only try twice
			BaseDelay:            10 * time.Millisecond,
			MaxDelay:             100 * time.Millisecond,
			Strategy:             httpx.RetryStrategyFixed,
			RetryableStatusCodes: []int{503},
		}

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientRetryPolicy(customPolicy),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		response, err := client.Execute(*req, map[string]any{})

		// Should succeed but return the error response
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
		assert.Equal(t, 2, attemptCount) // Should have tried exactly twice
	})

	t.Run("retry policy with custom middlewares", func(t *testing.T) {
		var middlewareExecuted = false
		customMiddleware := &testMiddleware{
			name: "custom",
			execute: func(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
				middlewareExecuted = true
				return next(ctx, req)
			},
		}

		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			if attemptCount < 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultRetryPolicy(),
			httpx.WithClientMiddleware(customMiddleware), // Custom middleware should run after retry
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		response, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, 2, attemptCount)   // Should have retried once
		assert.True(t, middlewareExecuted) // Custom middleware should have executed
	})
}

// testMiddleware is a simple test middleware for retry tests
type testMiddleware struct {
	name    string
	execute func(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error)
}

func (m *testMiddleware) Name() string {
	return m.name
}

func (m *testMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	return m.execute(ctx, req, next)
}
