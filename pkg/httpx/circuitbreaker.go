package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState string

const (
	// StateClosed indicates the circuit breaker is closed and allowing requests through
	StateClosed CircuitBreakerState = "closed"
	// StateOpen indicates the circuit breaker is open and blocking requests
	StateOpen CircuitBreakerState = "open"
	// StateHalfOpen indicates the circuit breaker is allowing limited requests to test if the service has recovered
	StateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig defines the configuration for circuit breaker behavior
type CircuitBreakerConfig struct {
	// Name is a unique identifier for this circuit breaker instance
	Name string

	// MaxRequests is the maximum number of requests allowed to pass through when the circuit breaker is half-open
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for the circuit breaker to clear the internal Counts
	Interval time.Duration

	// Timeout is the period of the open state, after which the circuit breaker switches to half-open
	Timeout time.Duration

	// ReadyToTrip returns true if the circuit breaker should trip to open state
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of the circuit breaker changes
	OnStateChange func(name string, from CircuitBreakerState, to CircuitBreakerState)

	// IsSuccessful determines whether a request is successful or not
	IsSuccessful func(err error, statusCode int) bool
}

// DefaultCircuitBreakerConfig returns a circuit breaker configuration with sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:        "default",
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// Trip if failure rate is 50% or higher with at least 5 requests
			return counts.Requests >= 5 && counts.TotalFailures >= counts.Requests/2
		},
		OnStateChange: func(_ string, _ CircuitBreakerState, _ CircuitBreakerState) {
			// Default no-op state change handler
		},
		IsSuccessful: func(err error, statusCode int) bool {
			// Consider 5xx server errors and network/timeout errors as failures
			if err != nil {
				httpErr := &HTTPError{}
				if errors.As(err, &httpErr) {
					return httpErr.Type != ErrorTypeServer &&
						httpErr.Type != ErrorTypeNetwork &&
						httpErr.Type != ErrorTypeTimeout
				}
				return false
			}
			return statusCode < 500
		},
	}
}

// AggressiveCircuitBreakerConfig returns a more aggressive circuit breaker configuration
func AggressiveCircuitBreakerConfig() CircuitBreakerConfig {
	config := DefaultCircuitBreakerConfig()
	config.Name = "aggressive"
	config.MaxRequests = 1
	config.Interval = 30 * time.Second
	config.Timeout = 30 * time.Second
	config.ReadyToTrip = func(counts Counts) bool {
		// Trip if failure rate is 30% or higher with at least 3 requests
		return counts.Requests >= 3 && counts.TotalFailures >= counts.Requests*3/10
	}
	return config
}

// ConservativeCircuitBreakerConfig returns a more conservative circuit breaker configuration
func ConservativeCircuitBreakerConfig() CircuitBreakerConfig {
	config := DefaultCircuitBreakerConfig()
	config.Name = "conservative"
	config.MaxRequests = 3
	config.Interval = 120 * time.Second
	config.Timeout = 120 * time.Second
	config.ReadyToTrip = func(counts Counts) bool {
		// Trip if failure rate is 80% or higher with at least 10 requests
		return counts.Requests >= 10 && counts.TotalFailures >= counts.Requests*4/5
	}
	return config
}

// Counts holds the numbers of requests and their successes/failures
type Counts struct {
	Requests             uint32 // Total number of requests
	TotalSuccesses       uint32 // Total number of successful requests
	TotalFailures        uint32 // Total number of failed requests
	ConsecutiveSuccesses uint32 // Number of consecutive successful requests
	ConsecutiveFailures  uint32 // Number of consecutive failed requests
}

// OnRequest increments the request count
func (c *Counts) OnRequest() {
	c.Requests++
}

// OnSuccess increments the success count and resets consecutive failures
func (c *Counts) OnSuccess() {
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

// OnFailure increments the failure count and resets consecutive successes
func (c *Counts) OnFailure() {
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

// Clear resets all counts to zero
func (c *Counts) Clear() {
	c.Requests = 0
	c.TotalSuccesses = 0
	c.TotalFailures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	config     CircuitBreakerConfig
	state      CircuitBreakerState
	generation uint64
	counts     Counts
	expiry     time.Time
	mutex      sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	// Set defaults for missing fields
	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}
	if config.Interval == 0 {
		config.Interval = 60 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.ReadyToTrip == nil {
		config.ReadyToTrip = DefaultCircuitBreakerConfig().ReadyToTrip
	}
	if config.OnStateChange == nil {
		config.OnStateChange = DefaultCircuitBreakerConfig().OnStateChange
	}
	if config.IsSuccessful == nil {
		config.IsSuccessful = DefaultCircuitBreakerConfig().IsSuccessful
	}
	if config.Name == "" {
		config.Name = "circuit_breaker"
	}

	cb := &CircuitBreaker{
		config:     config,
		state:      StateClosed,
		generation: 0,
		expiry:     time.Now().Add(config.Interval),
	}

	return cb
}

// Name returns the circuit breaker name
func (cb *CircuitBreaker) Name() string {
	return fmt.Sprintf("circuit_breaker_%s", cb.config.Name)
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns a copy of the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return cb.counts
}

// Execute implements the Middleware interface
func (cb *CircuitBreaker) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	resp, err := next(ctx, req)

	cb.afterRequest(generation, cb.config.IsSuccessful(err, cb.getStatusCode(resp)))

	return resp, err
}

// getStatusCode safely extracts status code from response
func (cb *CircuitBreaker) getStatusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

// beforeRequest checks if the request should be allowed and increments counters
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, &HTTPError{
			Type:    ErrorTypeMiddleware,
			Message: fmt.Sprintf("circuit breaker '%s' is open", cb.config.Name),
		}
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.config.MaxRequests {
		return generation, &HTTPError{
			Type:    ErrorTypeMiddleware,
			Message: fmt.Sprintf("circuit breaker '%s' is half-open and max requests exceeded", cb.config.Name),
		}
	}

	cb.counts.OnRequest()
	return generation, nil
}

// afterRequest updates the circuit breaker state based on the request result
func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return // Circuit breaker state changed during request, ignore this result
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess handles successful requests
func (cb *CircuitBreaker) onSuccess(state CircuitBreakerState, now time.Time) {
	cb.counts.OnSuccess()

	if state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.config.MaxRequests {
		cb.setState(StateClosed, now)
	}
}

// onFailure handles failed requests
func (cb *CircuitBreaker) onFailure(state CircuitBreakerState, now time.Time) {
	cb.counts.OnFailure()

	switch state {
	case StateClosed:
		if cb.config.ReadyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

// currentState returns the current state and generation
func (cb *CircuitBreaker) currentState(now time.Time) (CircuitBreakerState, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

// setState changes the circuit breaker state and calls the state change callback
func (cb *CircuitBreaker) setState(state CircuitBreakerState, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.config.Name, prev, state)
	}
}

// toNewGeneration resets the generation and sets appropriate expiry time
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts.Clear()

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.config.Interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.config.Interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.config.Timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}

// CircuitBreakerMiddleware wraps a CircuitBreaker to implement the Middleware interface
type CircuitBreakerMiddleware struct {
	circuitBreaker *CircuitBreaker
}

// NewCircuitBreakerMiddleware creates a new circuit breaker middleware
func NewCircuitBreakerMiddleware(config CircuitBreakerConfig) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		circuitBreaker: NewCircuitBreaker(config),
	}
}

// Name returns the middleware name
func (cbm *CircuitBreakerMiddleware) Name() string {
	return cbm.circuitBreaker.Name()
}

// Execute implements the Middleware interface
func (cbm *CircuitBreakerMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	return cbm.circuitBreaker.Execute(ctx, req, next)
}

// State returns the current state of the circuit breaker
func (cbm *CircuitBreakerMiddleware) State() CircuitBreakerState {
	return cbm.circuitBreaker.State()
}

// Counts returns the current counts
func (cbm *CircuitBreakerMiddleware) Counts() Counts {
	return cbm.circuitBreaker.Counts()
}

// CircuitBreakerError creates a circuit breaker specific error
func CircuitBreakerError(message string, req *http.Request) *HTTPError {
	return &HTTPError{
		Type:    ErrorTypeMiddleware,
		Message: message,
		Request: req,
	}
}

// IsCircuitBreakerError checks if an error is a circuit breaker error
func IsCircuitBreakerError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeMiddleware &&
			(contains(httpErr.Message, "circuit breaker") || contains(httpErr.Message, "open") || contains(httpErr.Message, "half-open"))
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				indexOf(s, substr) >= 0)))
}

// indexOf finds the index of substring in string
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
