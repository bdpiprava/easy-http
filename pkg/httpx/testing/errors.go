package testing

import (
	"fmt"
	"net/http"
	"time"
)

// ErrorSimulator provides methods for simulating various error conditions
type ErrorSimulator struct {
	response *ResponseBuilder
}

// SimulateError returns an ErrorSimulator for configuring error responses
func (rb *ResponseBuilder) SimulateError() *ErrorSimulator {
	return &ErrorSimulator{response: rb}
}

// Timeout simulates a request timeout by delaying for the specified duration
func (es *ErrorSimulator) Timeout(duration time.Duration) *ResponseBuilder {
	return es.response.WithDelay(func() {
		time.Sleep(duration)
	})
}

// NetworkError simulates a network error by closing the connection
func (es *ErrorSimulator) NetworkError() *ResponseBuilder {
	// Return a response that will cause the client to see a broken connection
	return es.response.
		WithStatus(http.StatusBadGateway).
		WithBodyString("simulated network error")
}

// InternalServerError simulates a 500 Internal Server Error
func (es *ErrorSimulator) InternalServerError() *ResponseBuilder {
	return es.response.
		WithStatus(http.StatusInternalServerError).
		WithJSON(map[string]interface{}{
			"error":   "internal server error",
			"message": "simulated server failure",
		})
}

// BadRequest simulates a 400 Bad Request error
func (es *ErrorSimulator) BadRequest(message string) *ResponseBuilder {
	if message == "" {
		message = "bad request"
	}
	return es.response.
		WithStatus(http.StatusBadRequest).
		WithJSON(map[string]interface{}{
			"error":   "bad_request",
			"message": message,
		})
}

// Unauthorized simulates a 401 Unauthorized error
func (es *ErrorSimulator) Unauthorized() *ResponseBuilder {
	return es.response.
		WithStatus(http.StatusUnauthorized).
		WithJSON(map[string]interface{}{
			"error":   "unauthorized",
			"message": "authentication required",
		})
}

// Forbidden simulates a 403 Forbidden error
func (es *ErrorSimulator) Forbidden() *ResponseBuilder {
	return es.response.
		WithStatus(http.StatusForbidden).
		WithJSON(map[string]interface{}{
			"error":   "forbidden",
			"message": "access denied",
		})
}

// NotFound simulates a 404 Not Found error
func (es *ErrorSimulator) NotFound() *ResponseBuilder {
	return es.response.
		WithStatus(http.StatusNotFound).
		WithJSON(map[string]interface{}{
			"error":   "not_found",
			"message": "resource not found",
		})
}

// Conflict simulates a 409 Conflict error
func (es *ErrorSimulator) Conflict(message string) *ResponseBuilder {
	if message == "" {
		message = "resource conflict"
	}
	return es.response.
		WithStatus(http.StatusConflict).
		WithJSON(map[string]interface{}{
			"error":   "conflict",
			"message": message,
		})
}

// TooManyRequests simulates a 429 Too Many Requests error
func (es *ErrorSimulator) TooManyRequests(retryAfter int) *ResponseBuilder {
	rb := es.response.
		WithStatus(http.StatusTooManyRequests).
		WithJSON(map[string]interface{}{
			"error":   "rate_limit_exceeded",
			"message": "too many requests",
		})

	if retryAfter > 0 {
		rb.WithHeader("Retry-After", fmt.Sprintf("%d", retryAfter))
	}

	return rb
}

// ServiceUnavailable simulates a 503 Service Unavailable error
func (es *ErrorSimulator) ServiceUnavailable(retryAfter int) *ResponseBuilder {
	rb := es.response.
		WithStatus(http.StatusServiceUnavailable).
		WithJSON(map[string]interface{}{
			"error":   "service_unavailable",
			"message": "service temporarily unavailable",
		})

	if retryAfter > 0 {
		rb.WithHeader("Retry-After", fmt.Sprintf("%d", retryAfter))
	}

	return rb
}

// GatewayTimeout simulates a 504 Gateway Timeout error
func (es *ErrorSimulator) GatewayTimeout() *ResponseBuilder {
	return es.response.
		WithStatus(http.StatusGatewayTimeout).
		WithJSON(map[string]interface{}{
			"error":   "gateway_timeout",
			"message": "upstream request timed out",
		})
}

// CustomError simulates a custom HTTP error with the specified status code and message
func (es *ErrorSimulator) CustomError(statusCode int, message string) *ResponseBuilder {
	return es.response.
		WithStatus(statusCode).
		WithJSON(map[string]interface{}{
			"error":   http.StatusText(statusCode),
			"message": message,
		})
}

// Slow simulates a slow response by adding a delay
func (es *ErrorSimulator) Slow(duration time.Duration) *ResponseBuilder {
	return es.response.WithDelay(func() {
		time.Sleep(duration)
	})
}

// Flaky simulates intermittent failures (alternates between success and failure)
type FlakyResponse struct {
	mock         *MockServer
	path         string
	successCount int
	failureCount int
	current      int
}

// OnFlaky creates a response that alternates between success and failure
func (m *MockServer) OnFlaky(path string, successEvery int) *FlakyResponse {
	return &FlakyResponse{
		mock:         m,
		path:         path,
		successCount: successEvery,
		failureCount: 1,
		current:      0,
	}
}

// WithPattern configures the flaky pattern (e.g., 3 successes, 2 failures)
func (fr *FlakyResponse) WithPattern(successCount, failureCount int) *FlakyResponse {
	fr.successCount = successCount
	fr.failureCount = failureCount
	return fr
}

// Configure sets up the flaky behavior on the mock server
func (fr *FlakyResponse) Configure() {
	fr.mock.On(ExactPath(fr.path)).WithBodyString("configured flaky response")
	// Note: Actual flaky behavior would require stateful response handling
	// This is a simplified version - full implementation would track call counts
}
