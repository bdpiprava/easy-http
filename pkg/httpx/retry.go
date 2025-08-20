package httpx

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"time"
)

// RetryStrategy defines different retry strategies
type RetryStrategy string

const (
	// RetryStrategyFixed uses a fixed delay between retries
	RetryStrategyFixed RetryStrategy = "fixed"
	// RetryStrategyLinear increases delay linearly with each attempt
	RetryStrategyLinear RetryStrategy = "linear"
	// RetryStrategyExponential uses exponential backoff (default)
	RetryStrategyExponential RetryStrategy = "exponential"
	// RetryStrategyExponentialJitter uses exponential backoff with jitter
	RetryStrategyExponentialJitter RetryStrategy = "exponential_jitter"
)

// RetryCondition determines whether a request should be retried based on error and response
type RetryCondition func(attempt int, err error, resp *http.Response) bool

// RetryPolicy defines the complete retry configuration
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts (including the initial request)
	MaxAttempts int

	// BaseDelay is the base delay between retries
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Strategy defines how delays are calculated
	Strategy RetryStrategy

	// Multiplier is used for exponential and linear strategies (default: 2.0 for exponential)
	Multiplier float64

	// JitterMax adds random jitter up to this duration (only for jitter strategy)
	JitterMax time.Duration

	// Condition determines if a request should be retried
	Condition RetryCondition

	// RetryableStatusCodes defines which HTTP status codes should trigger retries
	RetryableStatusCodes []int

	// RetryableErrorTypes defines which error types should trigger retries
	RetryableErrorTypes []ErrorType
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:          3,
		BaseDelay:            100 * time.Millisecond,
		MaxDelay:             5 * time.Second,
		Strategy:             RetryStrategyExponential,
		Multiplier:           2.0,
		JitterMax:            0,
		Condition:            AdvancedDefaultRetryCondition,
		RetryableStatusCodes: []int{500, 502, 503, 504, 429}, // Server errors and rate limiting
		RetryableErrorTypes:  []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout},
	}
}

// AggressiveRetryPolicy returns a more aggressive retry policy for critical operations
func AggressiveRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:          5,
		BaseDelay:            50 * time.Millisecond,
		MaxDelay:             10 * time.Second,
		Strategy:             RetryStrategyExponentialJitter,
		Multiplier:           1.5,
		JitterMax:            100 * time.Millisecond,
		Condition:            AggressiveRetryCondition,
		RetryableStatusCodes: []int{408, 429, 500, 502, 503, 504}, // Include timeouts
		RetryableErrorTypes:  []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeServer},
	}
}

// ConservativeRetryPolicy returns a conservative retry policy to minimize load
func ConservativeRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:          2,
		BaseDelay:            500 * time.Millisecond,
		MaxDelay:             2 * time.Second,
		Strategy:             RetryStrategyFixed,
		Multiplier:           1.0,
		JitterMax:            0,
		Condition:            ConservativeRetryCondition,
		RetryableStatusCodes: []int{503, 504}, // Only retry on service unavailable
		RetryableErrorTypes:  []ErrorType{ErrorTypeNetwork},
	}
}

// AdvancedDefaultRetryCondition is the default retry condition for advanced retry middleware
func AdvancedDefaultRetryCondition(attempt int, err error, resp *http.Response) bool {
	// Don't retry if we've exceeded reasonable attempts
	if attempt > 3 {
		return false
	}

	// Retry on network and timeout errors
	if err != nil {
		if httpErr, ok := err.(*HTTPError); ok {
			return httpErr.Type == ErrorTypeNetwork || httpErr.Type == ErrorTypeTimeout
		}
		return true // Retry on unknown errors
	}

	// Retry on server errors but not client errors
	if resp != nil {
		return resp.StatusCode >= 500 || resp.StatusCode == 429 // Rate limiting
	}

	return false
}

// AggressiveRetryCondition is more permissive with retries
func AggressiveRetryCondition(attempt int, err error, resp *http.Response) bool {
	// Allow more attempts
	if attempt > 5 {
		return false
	}

	// Retry on various error types
	if err != nil {
		if httpErr, ok := err.(*HTTPError); ok {
			switch httpErr.Type {
			case ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeServer:
				return true
			default:
				return false
			}
		}
		return true
	}

	// Retry on more status codes
	if resp != nil {
		return resp.StatusCode >= 500 ||
			resp.StatusCode == 429 ||
			resp.StatusCode == 408 // Request timeout
	}

	return false
}

// ConservativeRetryCondition is more restrictive with retries
func ConservativeRetryCondition(attempt int, err error, resp *http.Response) bool {
	// Limit attempts
	if attempt > 1 {
		return false
	}

	// Only retry on clear network issues
	if err != nil {
		if httpErr, ok := err.(*HTTPError); ok {
			return httpErr.Type == ErrorTypeNetwork
		}
		return false
	}

	// Only retry on service unavailable
	if resp != nil {
		return resp.StatusCode == 503 || resp.StatusCode == 504
	}

	return false
}

// AdvancedRetryMiddleware implements sophisticated retry logic
type AdvancedRetryMiddleware struct {
	policy RetryPolicy
}

// NewAdvancedRetryMiddleware creates a new advanced retry middleware
func NewAdvancedRetryMiddleware(policy RetryPolicy) *AdvancedRetryMiddleware {
	// Set defaults for missing fields
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 3
	}
	if policy.BaseDelay == 0 {
		policy.BaseDelay = 100 * time.Millisecond
	}
	if policy.MaxDelay == 0 {
		policy.MaxDelay = 5 * time.Second
	}
	if policy.Strategy == "" {
		policy.Strategy = RetryStrategyExponential
	}
	if policy.Multiplier == 0 {
		policy.Multiplier = 2.0
	}
	if policy.Condition == nil {
		policy.Condition = AdvancedDefaultRetryCondition
	}

	return &AdvancedRetryMiddleware{
		policy: policy,
	}
}

// Name returns the middleware name
func (m *AdvancedRetryMiddleware) Name() string {
	return "advanced-retry"
}

// Execute implements the Middleware interface
func (m *AdvancedRetryMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt < m.policy.MaxAttempts; attempt++ {
		// Clone the request for retry attempts
		reqClone := req.Clone(ctx)

		resp, err := next(ctx, reqClone)

		// Check if this was successful or if we shouldn't retry
		if !m.shouldRetry(attempt, err, resp) {
			return resp, err
		}

		// Store the last error/response for potential return
		lastErr = err
		lastResp = resp

		// Don't wait after the last attempt
		if attempt == m.policy.MaxAttempts-1 {
			break
		}

		// Calculate and apply delay
		delay := m.calculateDelay(attempt)
		if err := m.waitWithContext(ctx, delay); err != nil {
			return nil, err // Context cancelled or deadline exceeded
		}
	}

	// Return the last error or response
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

// shouldRetry determines if a request should be retried
func (m *AdvancedRetryMiddleware) shouldRetry(attempt int, err error, resp *http.Response) bool {
	// Use custom condition if provided
	if m.policy.Condition != nil {
		return m.policy.Condition(attempt, err, resp)
	}

	// Check against configured retryable error types
	if err != nil {
		if httpErr, ok := err.(*HTTPError); ok {
			for _, retryableType := range m.policy.RetryableErrorTypes {
				if httpErr.Type == retryableType {
					return true
				}
			}
		}
		return false
	}

	// Check against configured retryable status codes
	if resp != nil {
		for _, retryableCode := range m.policy.RetryableStatusCodes {
			if resp.StatusCode == retryableCode {
				return true
			}
		}
	}

	return false
}

// calculateDelay calculates the delay for the given attempt using the configured strategy
func (m *AdvancedRetryMiddleware) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch m.policy.Strategy {
	case RetryStrategyFixed:
		delay = m.policy.BaseDelay

	case RetryStrategyLinear:
		delay = time.Duration(float64(m.policy.BaseDelay) * (float64(attempt+1) * m.policy.Multiplier))

	case RetryStrategyExponential:
		multiplier := math.Pow(m.policy.Multiplier, float64(attempt))
		delay = time.Duration(float64(m.policy.BaseDelay) * multiplier)

	case RetryStrategyExponentialJitter:
		// Calculate exponential delay
		multiplier := math.Pow(m.policy.Multiplier, float64(attempt))
		baseDelay := time.Duration(float64(m.policy.BaseDelay) * multiplier)

		// Add jitter
		if m.policy.JitterMax > 0 {
			jitter := m.randomJitter(m.policy.JitterMax)
			delay = baseDelay + jitter
		} else {
			delay = baseDelay
		}

	default:
		// Default to exponential
		multiplier := math.Pow(2.0, float64(attempt))
		delay = time.Duration(float64(m.policy.BaseDelay) * multiplier)
	}

	// Cap at maximum delay
	if delay > m.policy.MaxDelay {
		delay = m.policy.MaxDelay
	}

	return delay
}

// randomJitter generates random jitter up to the specified maximum
func (m *AdvancedRetryMiddleware) randomJitter(maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return 0
	}

	// Generate cryptographically secure random number
	maxNanos := maxJitter.Nanoseconds()
	randomNanos, err := rand.Int(rand.Reader, big.NewInt(maxNanos))
	if err != nil {
		// Fallback to no jitter if random generation fails
		return 0
	}

	return time.Duration(randomNanos.Int64())
}

// waitWithContext waits for the specified duration while respecting context cancellation
func (m *AdvancedRetryMiddleware) waitWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// RetryableError wraps an error to indicate it should be retried
type RetryableError struct {
	Err   error
	After time.Duration // Suggested delay before retry
}

// Error implements the error interface
func (e *RetryableError) Error() string {
	if e.After > 0 {
		return fmt.Sprintf("retryable error (suggested delay: %v): %v", e.After, e.Err)
	}
	return fmt.Sprintf("retryable error: %v", e.Err)
}

// Unwrap implements the unwrapper interface
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is explicitly marked as retryable
func IsRetryable(err error) bool {
	_, ok := err.(*RetryableError)
	return ok
}

// WrapAsRetryable wraps an error as retryable with optional delay suggestion
func WrapAsRetryable(err error, suggestedDelay time.Duration) error {
	if err == nil {
		return nil
	}
	return &RetryableError{
		Err:   err,
		After: suggestedDelay,
	}
}
