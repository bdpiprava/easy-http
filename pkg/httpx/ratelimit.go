package httpx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting strategies
type RateLimiter interface {
	// Allow checks if a request is allowed and waits if necessary
	Allow(ctx context.Context) error
	// UpdateFromHeaders updates rate limit state from response headers
	UpdateFromHeaders(headers http.Header)
	// Status returns current rate limit status
	Status() RateLimitStatus
}

// RateLimitStatus provides information about current rate limit state
type RateLimitStatus struct {
	Limit     int       // Total requests allowed
	Remaining int       // Remaining requests in current window
	ResetAt   time.Time // When the rate limit resets
}

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
	Strategy        RateLimitStrategy // Rate limiting algorithm
	RequestsPerSec  float64           // Requests per second (0 = unlimited)
	BurstSize       int               // Max burst size for token bucket
	PerHost         bool              // Apply rate limit per host vs globally
	WaitOnLimit     bool              // Wait when limit reached vs return error
	MaxWaitDuration time.Duration     // Maximum time to wait for rate limit
}

// RateLimitStrategy defines the rate limiting algorithm
type RateLimitStrategy string

const (
	RateLimitTokenBucket   RateLimitStrategy = "token_bucket"
	RateLimitLeakyBucket   RateLimitStrategy = "leaky_bucket"
	RateLimitFixedWindow   RateLimitStrategy = "fixed_window"
	RateLimitSlidingWindow RateLimitStrategy = "sliding_window"
)

// TokenBucketLimiter implements the token bucket algorithm
type TokenBucketLimiter struct {
	mu         sync.Mutex
	rate       float64   // Tokens per second
	capacity   int       // Maximum tokens
	tokens     float64   // Current tokens
	lastRefill time.Time // Last refill time
	limit      int       // Rate limit from server
	remaining  int       // Remaining requests from server
	resetAt    time.Time // Rate limit reset time
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
func NewTokenBucketLimiter(requestsPerSec float64, burstSize int) *TokenBucketLimiter {
	if burstSize == 0 {
		burstSize = int(requestsPerSec)
		if burstSize == 0 {
			burstSize = 1
		}
	}
	return &TokenBucketLimiter{
		rate:       requestsPerSec,
		capacity:   burstSize,
		tokens:     float64(burstSize),
		lastRefill: time.Now(),
		limit:      -1,
		remaining:  -1,
	}
}

// Allow implements RateLimiter interface
func (r *TokenBucketLimiter) Allow(ctx context.Context) error {
	r.mu.Lock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens = math.Min(float64(r.capacity), r.tokens+elapsed*r.rate)
	r.lastRefill = now

	// Check if token is available
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		r.mu.Unlock()
		return nil
	}

	// Calculate wait time
	waitTime := time.Duration((1.0-r.tokens)/r.rate*1000) * time.Millisecond
	r.mu.Unlock()

	// Wait for token
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		r.mu.Lock()
		r.tokens = math.Max(0, r.tokens-1.0)
		r.mu.Unlock()
		return nil
	}
}

// UpdateFromHeaders implements RateLimiter interface
func (r *TokenBucketLimiter) UpdateFromHeaders(headers http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Parse X-RateLimit-* headers (common format used by many APIs)
	if limitStr := headers.Get("X-RateLimit-Limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			r.limit = limit
		}
	}

	if remainingStr := headers.Get("X-RateLimit-Remaining"); remainingStr != "" {
		if remaining, err := strconv.Atoi(remainingStr); err == nil {
			r.remaining = remaining
		}
	}

	if resetStr := headers.Get("X-RateLimit-Reset"); resetStr != "" {
		// Try Unix timestamp first
		if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			r.resetAt = time.Unix(resetUnix, 0)
		} else {
			// Try HTTP date format
			if resetTime, err := time.Parse(time.RFC1123, resetStr); err == nil {
				r.resetAt = resetTime
			}
		}
	}
}

// Status implements RateLimiter interface
func (r *TokenBucketLimiter) Status() RateLimitStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	return RateLimitStatus{
		Limit:     r.limit,
		Remaining: r.remaining,
		ResetAt:   r.resetAt,
	}
}

// RateLimitMiddleware implements rate limiting for HTTP requests
type RateLimitMiddleware struct {
	config   RateLimitConfig
	limiters map[string]RateLimiter // Per-host limiters
	mu       sync.RWMutex
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(config RateLimitConfig) *RateLimitMiddleware {
	if config.Strategy == "" {
		config.Strategy = RateLimitTokenBucket
	}
	if config.MaxWaitDuration == 0 {
		config.MaxWaitDuration = 30 * time.Second
	}
	if !config.WaitOnLimit {
		config.MaxWaitDuration = 0
	}

	return &RateLimitMiddleware{
		config:   config,
		limiters: make(map[string]RateLimiter),
	}
}

// Name returns the middleware name
func (m *RateLimitMiddleware) Name() string {
	return "rate-limit"
}

// Execute implements the Middleware interface
func (m *RateLimitMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	limiter := m.getLimiter(req.URL)

	// Apply rate limit
	waitCtx := ctx
	var cancel context.CancelFunc
	if m.config.MaxWaitDuration > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, m.config.MaxWaitDuration)
		defer cancel()
	} else if m.config.MaxWaitDuration == 0 {
		// MaxWaitDuration of 0 means no waiting - fail immediately if no token available
		waitCtx, cancel = context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()
	}

	if err := limiter.Allow(waitCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &HTTPError{
				Type:    ErrorTypeMiddleware,
				Message: fmt.Sprintf("rate limit wait timeout exceeded: %v", m.config.MaxWaitDuration),
				Cause:   err,
				Request: req,
			}
		}
		return nil, err
	}

	// Execute request
	resp, err := next(ctx, req)
	if err != nil {
		return nil, err
	}

	// Update rate limiter from response headers
	limiter.UpdateFromHeaders(resp.Header)

	// Check for rate limit errors and retry after
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			// Parse Retry-After header (can be seconds or HTTP date)
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				waitDuration := time.Duration(seconds) * time.Second
				if m.config.WaitOnLimit && waitDuration <= m.config.MaxWaitDuration {
					time.Sleep(waitDuration)
					// Retry the request
					return m.Execute(ctx, req, next)
				}
			}
		}
	}

	return resp, nil
}

// getLimiter gets or creates a rate limiter for the given URL
func (m *RateLimitMiddleware) getLimiter(u *url.URL) RateLimiter {
	key := "global"
	if m.config.PerHost {
		key = u.Host
	}

	m.mu.RLock()
	limiter, exists := m.limiters[key]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := m.limiters[key]; exists {
		return limiter
	}

	// Create new limiter based on strategy
	switch m.config.Strategy {
	case RateLimitTokenBucket:
		limiter = NewTokenBucketLimiter(m.config.RequestsPerSec, m.config.BurstSize)
	default:
		limiter = NewTokenBucketLimiter(m.config.RequestsPerSec, m.config.BurstSize)
	}

	m.limiters[key] = limiter
	return limiter
}

// GetStatus returns the current rate limit status for a URL
func (m *RateLimitMiddleware) GetStatus(u *url.URL) RateLimitStatus {
	limiter := m.getLimiter(u)
	return limiter.Status()
}
