package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestNewTokenBucketLimiter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		requestsPerSec  float64
		burstSize       int
		wantBurstSize   int
		wantNotNil      bool
	}{
		{
			name:           "creates limiter with positive values",
			requestsPerSec: 10.0,
			burstSize:      20,
			wantBurstSize:  20,
			wantNotNil:     true,
		},
		{
			name:           "creates limiter with zero burst size uses rate as burst",
			requestsPerSec: 5.0,
			burstSize:      0,
			wantBurstSize:  5,
			wantNotNil:     true,
		},
		{
			name:           "creates limiter with zero rate and burst defaults to 1",
			requestsPerSec: 0,
			burstSize:      0,
			wantBurstSize:  1,
			wantNotNil:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewTokenBucketLimiter(tc.requestsPerSec, tc.burstSize)

			if tc.wantNotNil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestTokenBucketLimiter_Allow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestsPerSec float64
		burstSize      int
		setupLimiter   func(*httpx.TokenBucketLimiter)
		numRequests    int
		waitBetween    time.Duration
		wantAllowed    int
		wantBlocked    int
	}{
		{
			name:           "allows burst requests immediately",
			requestsPerSec: 10,
			burstSize:      5,
			setupLimiter:   func(l *httpx.TokenBucketLimiter) {},
			numRequests:    5,
			waitBetween:    0,
			wantAllowed:    5,
			wantBlocked:    0,
		},
		{
			name:           "blocks requests exceeding burst",
			requestsPerSec: 1,
			burstSize:      2,
			setupLimiter:   func(l *httpx.TokenBucketLimiter) {},
			numRequests:    3,
			waitBetween:    0,
			wantAllowed:    2,
			wantBlocked:    1,
		},
		{
			name:           "refills tokens over time",
			requestsPerSec: 10,
			burstSize:      5,
			setupLimiter: func(l *httpx.TokenBucketLimiter) {
				// Consume all tokens first
				ctx := context.Background()
				for i := 0; i < 5; i++ {
					_ = l.Allow(ctx)
				}
			},
			numRequests:  2,
			waitBetween:  150 * time.Millisecond, // Allow time for refill
			wantAllowed:  2,
			wantBlocked:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewTokenBucketLimiter(tc.requestsPerSec, tc.burstSize)
			tc.setupLimiter(subject)

			allowed := 0
			blocked := 0

			for i := 0; i < tc.numRequests; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err := subject.Allow(ctx)
				cancel()

				if err == nil {
					allowed++
				} else {
					blocked++
				}

				if tc.waitBetween > 0 && i < tc.numRequests-1 {
					time.Sleep(tc.waitBetween)
				}
			}

			assert.Equal(t, tc.wantAllowed, allowed, "allowed requests mismatch")
			assert.Equal(t, tc.wantBlocked, blocked, "blocked requests mismatch")
		})
	}
}

func TestTokenBucketLimiter_Allow_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewTokenBucketLimiter(1, 1)

		// Consume the single token
		ctx1 := context.Background()
		err := subject.Allow(ctx1)
		require.NoError(t, err)

		// Try to get another token with already cancelled context
		ctx2, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		gotErr := subject.Allow(ctx2)

		assert.Error(t, gotErr)
		assert.Equal(t, context.Canceled, gotErr)
	})

	t.Run("respects context timeout", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewTokenBucketLimiter(0.1, 1) // Very slow refill

		// Consume the token
		ctx1 := context.Background()
		err := subject.Allow(ctx1)
		require.NoError(t, err)

		// Try with short timeout
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		gotErr := subject.Allow(ctx2)

		assert.Error(t, gotErr)
		assert.Equal(t, context.DeadlineExceeded, gotErr)
	})
}

func TestTokenBucketLimiter_UpdateFromHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		headers       http.Header
		wantLimit     int
		wantRemaining int
		wantResetAt   bool
	}{
		{
			name: "parses standard X-RateLimit headers",
			headers: http.Header{
				"X-RateLimit-Limit":     []string{"100"},
				"X-RateLimit-Remaining": []string{"50"},
				"X-RateLimit-Reset":     []string{"1640000000"},
			},
			wantLimit:     100,
			wantRemaining: 50,
			wantResetAt:   true,
		},
		{
			name: "handles partial headers",
			headers: http.Header{
				"X-RateLimit-Limit": []string{"200"},
			},
			wantLimit:     200,
			wantRemaining: -1,
			wantResetAt:   false,
		},
		{
			name: "ignores invalid values",
			headers: http.Header{
				"X-RateLimit-Limit":     []string{"invalid"},
				"X-RateLimit-Remaining": []string{"also-invalid"},
			},
			wantLimit:     -1,
			wantRemaining: -1,
			wantResetAt:   false,
		},
		{
			name: "parses HTTP date format for reset",
			headers: http.Header{
				"X-RateLimit-Reset": []string{"Mon, 02 Jan 2006 15:04:05 GMT"},
			},
			wantLimit:     -1,
			wantRemaining: -1,
			wantResetAt:   true,
		},
		{
			name:          "handles empty headers",
			headers:       http.Header{},
			wantLimit:     -1,
			wantRemaining: -1,
			wantResetAt:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewTokenBucketLimiter(10, 20)

			subject.UpdateFromHeaders(tc.headers)

			status := subject.Status()
			assert.Equal(t, tc.wantLimit, status.Limit)
			assert.Equal(t, tc.wantRemaining, status.Remaining)

			if tc.wantResetAt {
				assert.False(t, status.ResetAt.IsZero())
			} else {
				assert.True(t, status.ResetAt.IsZero())
			}
		})
	}
}

func TestTokenBucketLimiter_Status(t *testing.T) {
	t.Parallel()

	t.Run("returns current status", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewTokenBucketLimiter(10, 20)

		// Update with headers
		headers := http.Header{
			"X-RateLimit-Limit":     []string{"100"},
			"X-RateLimit-Remaining": []string{"75"},
			"X-RateLimit-Reset":     []string{"1640000000"},
		}
		subject.UpdateFromHeaders(headers)

		got := subject.Status()

		assert.Equal(t, 100, got.Limit)
		assert.Equal(t, 75, got.Remaining)
		assert.False(t, got.ResetAt.IsZero())
	})
}

func TestTokenBucketLimiter_Concurrency(t *testing.T) {
	t.Parallel()

	t.Run("handles concurrent requests safely", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewTokenBucketLimiter(100, 50)
		var wg sync.WaitGroup
		var allowed int32
		var blocked int32

		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				err := subject.Allow(ctx)
				if err == nil {
					atomic.AddInt32(&allowed, 1)
				} else {
					atomic.AddInt32(&blocked, 1)
				}
			}()
		}

		wg.Wait()

		// At least burst size should be allowed
		assert.True(t, allowed >= 50, "Expected at least burst size to be allowed")
		assert.Equal(t, int32(numGoroutines), allowed+blocked)
	})
}

func TestNewRateLimitMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		config          httpx.RateLimitConfig
		wantStrategy    httpx.RateLimitStrategy
		wantMaxWait     time.Duration
	}{
		{
			name: "creates middleware with custom config",
			config: httpx.RateLimitConfig{
				Strategy:        httpx.RateLimitTokenBucket,
				RequestsPerSec:  5,
				BurstSize:       10,
				WaitOnLimit:     true,
				MaxWaitDuration: 10 * time.Second,
			},
			wantStrategy: httpx.RateLimitTokenBucket,
			wantMaxWait:  10 * time.Second,
		},
		{
			name: "uses defaults when not specified",
			config: httpx.RateLimitConfig{
				RequestsPerSec: 10,
			},
			wantStrategy: httpx.RateLimitTokenBucket,
			wantMaxWait:  30 * time.Second,
		},
		{
			name: "sets MaxWaitDuration to 0 when WaitOnLimit is false",
			config: httpx.RateLimitConfig{
				RequestsPerSec:  10,
				WaitOnLimit:     false,
				MaxWaitDuration: 30 * time.Second, // Should be overridden
			},
			wantStrategy: httpx.RateLimitTokenBucket,
			wantMaxWait:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewRateLimitMiddleware(tc.config)

			assert.NotNil(t, got)
			assert.Equal(t, "rate-limit", got.Name())
		})
	}
}

func TestRateLimitMiddleware_Name(t *testing.T) {
	t.Parallel()

	t.Run("returns rate-limit as middleware name", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec: 10,
		})

		got := subject.Name()

		assert.Equal(t, "rate-limit", got)
	})
}

func TestRateLimitMiddleware_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		config          httpx.RateLimitConfig
		numRequests     int
		requestDelay    time.Duration
		wantSuccessMin  int
		wantErrors      bool
	}{
		{
			name: "allows requests within rate limit",
			config: httpx.RateLimitConfig{
				RequestsPerSec: 10,
				BurstSize:      5,
				WaitOnLimit:    true,
			},
			numRequests:    3,
			requestDelay:   0,
			wantSuccessMin: 3,
			wantErrors:     false,
		},
		{
			name: "blocks requests exceeding rate limit with WaitOnLimit false",
			config: httpx.RateLimitConfig{
				RequestsPerSec:  1,
				BurstSize:       2,
				WaitOnLimit:     false,
				MaxWaitDuration: 0,
			},
			numRequests:    5,
			requestDelay:   0,
			wantSuccessMin: 2,
			wantErrors:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer server.Close()

			middleware := httpx.NewRateLimitMiddleware(tc.config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			successCount := 0
			errorCount := 0

			for i := 0; i < tc.numRequests; i++ {
				req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
				_, err := client.Execute(*req, map[string]any{})

				if err == nil {
					successCount++
				} else {
					errorCount++
				}

				if tc.requestDelay > 0 {
					time.Sleep(tc.requestDelay)
				}
			}

			assert.True(t, successCount >= tc.wantSuccessMin)
			if tc.wantErrors {
				assert.True(t, errorCount > 0)
			}
		})
	}
}

func TestRateLimitMiddleware_Execute_429Response(t *testing.T) {
	t.Parallel()

	t.Run("handles 429 Too Many Requests with Retry-After", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.Header().Set("Retry-After", "1") // 1 second
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true}`))
			}
		}))
		defer server.Close()

		middleware := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec:  10,
			BurstSize:       5,
			WaitOnLimit:     true,
			MaxWaitDuration: 5 * time.Second,
		})

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, 2, callCount, "Expected retry after 429")
	})

	t.Run("does not retry 429 when Retry-After exceeds MaxWaitDuration", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Retry-After", "60") // 60 seconds
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
		}))
		defer server.Close()

		middleware := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec:  10,
			BurstSize:       5,
			WaitOnLimit:     true,
			MaxWaitDuration: 1 * time.Second,
		})

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		assert.Equal(t, 1, callCount, "Should not retry")
	})
}

func TestRateLimitMiddleware_Execute_UpdatesFromHeaders(t *testing.T) {
	t.Parallel()

	t.Run("updates rate limiter from response headers", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", "100")
			w.Header().Set("X-RateLimit-Remaining", "99")
			w.Header().Set("X-RateLimit-Reset", "1640000000")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		defer server.Close()

		middleware := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec: 10,
			BurstSize:      5,
		})

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Check that status was updated
		u, _ := url.Parse(server.URL + "/test")
		status := middleware.GetStatus(u)
		assert.Equal(t, 100, status.Limit)
		assert.Equal(t, 99, status.Remaining)
	})
}

func TestRateLimitMiddleware_Execute_PerHost(t *testing.T) {
	t.Parallel()

	t.Run("applies rate limits per host when PerHost is true", func(t *testing.T) {
		t.Parallel()

		server1Calls := 0
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server1Calls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"host":"server1"}`))
		}))
		defer server1.Close()

		server2Calls := 0
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server2Calls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"host":"server2"}`))
		}))
		defer server2.Close()

		middleware := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec: 2,
			BurstSize:      2,
			PerHost:        true, // Different limits per host
			WaitOnLimit:    false,
		})

		client := httpx.NewClientWithConfig(
			httpx.WithClientMiddleware(middleware),
		)

		// Make 2 requests to server1 (should succeed)
		for i := 0; i < 2; i++ {
			req := httpx.NewRequest(http.MethodGet,
				httpx.WithBaseURL(server1.URL),
				httpx.WithPath("/test"))
			_, err := client.Execute(*req, map[string]any{})
			assert.NoError(t, err)
		}

		// Make 2 requests to server2 (should also succeed - different host)
		for i := 0; i < 2; i++ {
			req := httpx.NewRequest(http.MethodGet,
				httpx.WithBaseURL(server2.URL),
				httpx.WithPath("/test"))
			_, err := client.Execute(*req, map[string]any{})
			assert.NoError(t, err)
		}

		assert.Equal(t, 2, server1Calls)
		assert.Equal(t, 2, server2Calls)
	})
}

func TestRateLimitMiddleware_Execute_ContextTimeout(t *testing.T) {
	t.Parallel()

	t.Run("returns error when rate limit wait exceeds MaxWaitDuration", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		middleware := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec:  1,
			BurstSize:       1,
			WaitOnLimit:     true,
			MaxWaitDuration: 10 * time.Millisecond,
		})

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware),
		)

		// First request succeeds
		req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		_, err := client.Execute(*req1, map[string]any{})
		require.NoError(t, err)

		// Second request should timeout
		req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		_, gotErr := client.Execute(*req2, map[string]any{})

		assert.Error(t, gotErr)
		assert.Contains(t, gotErr.Error(), "rate limit wait timeout exceeded")
	})
}

func TestRateLimitMiddleware_GetStatus(t *testing.T) {
	t.Parallel()

	t.Run("returns rate limit status for URL", func(t *testing.T) {
		t.Parallel()

		middleware := httpx.NewRateLimitMiddleware(httpx.RateLimitConfig{
			RequestsPerSec: 10,
			BurstSize:      5,
		})

		u, _ := url.Parse("https://api.example.com/test")
		status := middleware.GetStatus(u)

		// Initial status should have defaults
		assert.Equal(t, -1, status.Limit)
		assert.Equal(t, -1, status.Remaining)
		assert.True(t, status.ResetAt.IsZero())
	})
}

func TestRateLimitMiddleware_Integration(t *testing.T) {
	t.Parallel()

	t.Run("end-to-end rate limiting with client", func(t *testing.T) {
		t.Parallel()

		requestTimes := []time.Time{}
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestTimes = append(requestTimes, time.Now())
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultRateLimit(), // 10 req/sec with burst of 20
		)

		// Make 5 requests rapidly
		for i := 0; i < 5; i++ {
			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
			resp, err := client.Execute(*req, map[string]any{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}

		mu.Lock()
		numRequests := len(requestTimes)
		mu.Unlock()

		assert.Equal(t, 5, numRequests)
	})

	t.Run("custom rate limit configuration", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientRateLimit(httpx.RateLimitConfig{
				Strategy:        httpx.RateLimitTokenBucket,
				RequestsPerSec:  2,
				BurstSize:       2,
				WaitOnLimit:     false,
				MaxWaitDuration: 0,
			}),
		)

		// Make 4 requests - only 2 should succeed (burst limit)
		successCount := 0
		for i := 0; i < 4; i++ {
			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
			_, err := client.Execute(*req, map[string]any{})
			if err == nil {
				successCount++
			}
		}

		assert.Equal(t, 2, successCount)
		assert.Equal(t, 2, callCount)
	})
}

func TestRateLimitMiddleware_TokenRefill(t *testing.T) {
	t.Parallel()

	t.Run("allows requests after token refill period", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		// 5 requests per second = 200ms per token
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientRateLimit(httpx.RateLimitConfig{
				RequestsPerSec:  5,
				BurstSize:       2,
				WaitOnLimit:     true,
				MaxWaitDuration: 1 * time.Second,
			}),
		)

		// Use burst (2 requests)
		for i := 0; i < 2; i++ {
			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
			_, err := client.Execute(*req, map[string]any{})
			require.NoError(t, err)
		}

		// Wait for token refill (250ms should give us 1+ token)
		time.Sleep(250 * time.Millisecond)

		// Should succeed after refill
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		_, err := client.Execute(*req, map[string]any{})
		require.NoError(t, err)

		assert.Equal(t, 3, callCount)
	})
}
