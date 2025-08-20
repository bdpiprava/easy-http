package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
)

// ErrorType represents the category of HTTP error
type ErrorType string

const (
	// ErrorTypeNetwork indicates network-related errors (DNS, connection failures, etc.)
	ErrorTypeNetwork ErrorType = "network"
	// ErrorTypeTimeout indicates timeout errors
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeClient indicates client errors (4xx status codes)
	ErrorTypeClient ErrorType = "client"
	// ErrorTypeServer indicates server errors (5xx status codes)
	ErrorTypeServer ErrorType = "server"
	// ErrorTypeValidation indicates validation errors (invalid URLs, headers, etc.)
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeMiddleware indicates middleware-related errors
	ErrorTypeMiddleware ErrorType = "middleware"
	// ErrorTypeUnknown indicates unclassified errors
	ErrorTypeUnknown ErrorType = "unknown"
)

// HTTPError is the base error type for all HTTP-related errors
type HTTPError struct {
	Type       ErrorType       // The category of error
	Message    string          // Human-readable error message
	Cause      error           // The underlying error that caused this error
	Request    *http.Request   // The HTTP request that caused the error (may be nil)
	Response   *http.Response  // The HTTP response if available (may be nil)
	StatusCode int             // HTTP status code if available (0 if not applicable)
	Context    context.Context // Request context for additional metadata
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	if e.Request != nil {
		return fmt.Sprintf("%s error for %s %s: %s",
			e.Type, e.Request.Method, e.Request.URL.String(), e.Message)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// Unwrap implements the unwrapper interface for error chains
func (e *HTTPError) Unwrap() error {
	return e.Cause
}

// Is implements the error equality interface
func (e *HTTPError) Is(target error) bool {
	if httpErr, ok := target.(*HTTPError); ok {
		return e.Type == httpErr.Type && e.StatusCode == httpErr.StatusCode
	}
	return false
}

// NewHTTPError creates a new HTTPError with the given parameters
func NewHTTPError(errorType ErrorType, message string, cause error, req *http.Request, resp *http.Response) *HTTPError {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	var ctx context.Context
	if req != nil {
		ctx = req.Context()
	}

	return &HTTPError{
		Type:       errorType,
		Message:    message,
		Cause:      cause,
		Request:    req,
		Response:   resp,
		StatusCode: statusCode,
		Context:    ctx,
	}
}

// NetworkError creates a network-related error
func NetworkError(message string, cause error, req *http.Request) *HTTPError {
	return NewHTTPError(ErrorTypeNetwork, message, cause, req, nil)
}

// TimeoutError creates a timeout-related error
func TimeoutError(message string, cause error, req *http.Request) *HTTPError {
	return NewHTTPError(ErrorTypeTimeout, message, cause, req, nil)
}

// ClientError creates a client error (4xx status codes)
func ClientError(message string, req *http.Request, resp *http.Response) *HTTPError {
	return NewHTTPError(ErrorTypeClient, message, nil, req, resp)
}

// ServerError creates a server error (5xx status codes)
func ServerError(message string, req *http.Request, resp *http.Response) *HTTPError {
	return NewHTTPError(ErrorTypeServer, message, nil, req, resp)
}

// ValidationError creates a validation error
func ValidationError(message string, cause error) *HTTPError {
	return NewHTTPError(ErrorTypeValidation, message, cause, nil, nil)
}

// MiddlewareError creates a middleware-related error
func MiddlewareError(message string, cause error, req *http.Request) *HTTPError {
	return NewHTTPError(ErrorTypeMiddleware, message, cause, req, nil)
}

// ClassifyError analyzes an error and returns an appropriate HTTPError
func ClassifyError(err error, req *http.Request, resp *http.Response) *HTTPError {
	// For response-related errors, check status code first
	if resp != nil {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return ClientError(fmt.Sprintf("client error %d: %s", resp.StatusCode, resp.Status), req, resp)
		}
		if resp.StatusCode >= 500 {
			return ServerError(fmt.Sprintf("server error %d: %s", resp.StatusCode, resp.Status), req, resp)
		}
	}

	// If no error and no problematic status code, return nil
	if err == nil {
		return nil
	}

	// If it's already an HTTPError, return as is
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr
	}

	// Analyze the error to determine its type
	errorType, message := classifyErrorType(err)

	return NewHTTPError(errorType, message, err, req, resp)
}

// classifyErrorType analyzes the underlying error to determine its type
func classifyErrorType(err error) (ErrorType, string) {
	errStr := err.Error()

	// Check for timeout errors
	if isTimeoutError(err) {
		return ErrorTypeTimeout, "request timeout"
	}

	// Check for network errors
	if isNetworkError(err) {
		return ErrorTypeNetwork, "network error"
	}

	// Check for URL validation errors
	urlErr := &url.Error{}
	if errors.As(err, &urlErr) {
		if strings.Contains(errStr, "invalid") || strings.Contains(errStr, "parse") {
			return ErrorTypeValidation, "invalid URL"
		}
		return ErrorTypeNetwork, "URL error"
	}

	// Check for validation-like errors
	if strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "bad") {
		return ErrorTypeValidation, "validation error"
	}

	return ErrorTypeUnknown, errStr
}

// isTimeoutError checks if an error is timeout-related
func isTimeoutError(err error) bool {
	// Check for context timeout
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for net timeout
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for syscall timeout
	opErr := &net.OpError{}
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return true
		}
		var sysErr syscall.Errno
		if errors.As(opErr.Err, &sysErr) {
			if sysErr == syscall.ETIMEDOUT {
				return true
			}
		}
	}

	// Check error message for timeout indicators
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline exceeded")
}

// isNetworkError checks if an error is network-related
func isNetworkError(err error) bool {
	// Check for network operations
	opError := &net.OpError{}
	if errors.As(err, &opError) {
		return true
	}

	// Check for DNS errors
	dNSError := &net.DNSError{}
	if errors.As(err, &dNSError) {
		return true
	}

	// Check for address errors
	addrError := &net.AddrError{}
	if errors.As(err, &addrError) {
		return true
	}

	// Check error message for network indicators
	errStr := strings.ToLower(err.Error())
	networkIndicators := []string{
		"connection refused",
		"no such host",
		"network unreachable",
		"connection reset",
		"broken pipe",
		"dial tcp",
		"dial udp",
		"dns",
	}

	for _, indicator := range networkIndicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}

	return false
}

// Helper functions for error type checking

// IsNetworkError checks if an error is network-related
func IsNetworkError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeNetwork
	}
	return false
}

// IsTimeoutError checks if an error is timeout-related
func IsTimeoutError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeTimeout
	}
	return false
}

// IsClientError checks if an error is a client error (4xx)
func IsClientError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeClient
	}
	return false
}

// IsServerError checks if an error is a server error (5xx)
func IsServerError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeServer
	}
	return false
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeValidation
	}
	return false
}

// IsMiddlewareError checks if an error is middleware-related
func IsMiddlewareError(err error) bool {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Type == ErrorTypeMiddleware
	}
	return false
}

// GetStatusCode extracts the HTTP status code from an error if available
func GetStatusCode(err error) int {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return 0
}

// GetRequestContext extracts the request context from an error if available
func GetRequestContext(err error) context.Context {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.Context
	}
	return nil
}
