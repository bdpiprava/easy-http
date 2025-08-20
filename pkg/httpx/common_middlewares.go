package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// LoggingMiddleware logs HTTP requests and responses
type LoggingMiddleware struct {
	logger   *slog.Logger
	logLevel slog.Level
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger *slog.Logger, level slog.Level) *LoggingMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingMiddleware{
		logger:   logger,
		logLevel: level,
	}
}

// Name returns the middleware name
func (m *LoggingMiddleware) Name() string {
	return "logging"
}

// Execute implements the Middleware interface
func (m *LoggingMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	if !m.logger.Enabled(ctx, m.logLevel) {
		return next(ctx, req)
	}

	// Log the outgoing request
	m.logger.LogAttrs(ctx, slog.LevelDebug, "HTTP request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("host", req.Host),
		slog.Any("headers", req.Header),
	)

	start := time.Now()
	resp, err := next(ctx, req)
	duration := time.Since(start)

	if err != nil {
		m.logger.LogAttrs(ctx, slog.LevelError, "Failed to execute HTTP request",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	// Determine log level based on response status
	level := slog.LevelInfo
	if resp.StatusCode >= 400 {
		level = slog.LevelWarn
	}
	if resp.StatusCode >= 500 {
		level = slog.LevelError
	}

	m.logger.LogAttrs(ctx, level, "HTTP response",
		slog.Int("status_code", resp.StatusCode),
		slog.String("status", resp.Status),
		slog.Duration("duration", duration),
		slog.String("content_length", resp.Header.Get("Content-Length")),
		slog.String("content_type", resp.Header.Get("Content-Type")),
	)

	return resp, nil
}

// RetryMiddleware implements automatic retry logic with exponential backoff
type RetryMiddleware struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	retryFunc  func(error, *http.Response) bool
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	RetryFunc  func(error, *http.Response) bool
}

// DefaultRetryConfig provides sensible retry defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		RetryFunc:  DefaultRetryCondition,
	}
}

// DefaultRetryCondition determines if a request should be retried
func DefaultRetryCondition(err error, resp *http.Response) bool {
	// Retry on network errors
	if err != nil {
		return true
	}

	// Retry on server errors (5xx) but not client errors (4xx)
	if resp != nil && resp.StatusCode >= 500 {
		return true
	}

	return false
}

// NewRetryMiddleware creates a new retry middleware
func NewRetryMiddleware(config RetryConfig) *RetryMiddleware {
	if config.RetryFunc == nil {
		config.RetryFunc = DefaultRetryCondition
	}
	if config.BaseDelay == 0 {
		config.BaseDelay = 100 * time.Millisecond
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 5 * time.Second
	}

	return &RetryMiddleware{
		maxRetries: config.MaxRetries,
		baseDelay:  config.BaseDelay,
		maxDelay:   config.MaxDelay,
		retryFunc:  config.RetryFunc,
	}
}

// Name returns the middleware name
func (m *RetryMiddleware) Name() string {
	return "retry"
}

// Execute implements the Middleware interface
func (m *RetryMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		// Clone the request for retry attempts to avoid issues with consumed request bodies
		reqClone := req.Clone(ctx)

		resp, err := next(ctx, reqClone)

		// If successful, return immediately
		if err == nil && (resp == nil || !m.retryFunc(nil, resp)) {
			return resp, nil
		}

		// Store the last error/response for potential return
		lastErr = err
		lastResp = resp

		// Don't retry if this is the last attempt
		if attempt == m.maxRetries {
			break
		}

		// Check if we should retry
		if !m.retryFunc(err, resp) {
			break
		}

		// Calculate delay with exponential backoff
		multiplier := 1 << uint(attempt)
		delay := time.Duration(float64(m.baseDelay) * float64(multiplier))
		if delay > m.maxDelay {
			delay = m.maxDelay
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// Return the last error or response
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

// MetricsMiddleware collects HTTP request metrics
type MetricsMiddleware struct {
	collector MetricsCollector
}

// MetricsCollector defines the interface for collecting HTTP metrics
type MetricsCollector interface {
	IncrementRequests(method, url string)
	IncrementErrors(method, url string, statusCode int)
	RecordDuration(method, url string, duration time.Duration)
}

// NoOpMetricsCollector is a no-op implementation for testing
type NoOpMetricsCollector struct{}

func (NoOpMetricsCollector) IncrementRequests(method, url string)                      {}
func (NoOpMetricsCollector) IncrementErrors(method, url string, statusCode int)        {}
func (NoOpMetricsCollector) RecordDuration(method, url string, duration time.Duration) {}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(collector MetricsCollector) *MetricsMiddleware {
	if collector == nil {
		collector = NoOpMetricsCollector{}
	}
	return &MetricsMiddleware{
		collector: collector,
	}
}

// Name returns the middleware name
func (m *MetricsMiddleware) Name() string {
	return "metrics"
}

// Execute implements the Middleware interface
func (m *MetricsMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	method := req.Method
	url := req.URL.String()

	m.collector.IncrementRequests(method, url)

	start := time.Now()
	resp, err := next(ctx, req)
	duration := time.Since(start)

	m.collector.RecordDuration(method, url, duration)

	if err != nil {
		m.collector.IncrementErrors(method, url, 0) // 0 indicates network error
		return nil, err
	}

	if resp.StatusCode >= 400 {
		m.collector.IncrementErrors(method, url, resp.StatusCode)
	}

	return resp, nil
}

// UserAgentMiddleware adds or modifies the User-Agent header
type UserAgentMiddleware struct {
	userAgent string
	append    bool
}

// NewUserAgentMiddleware creates a new User-Agent middleware
func NewUserAgentMiddleware(userAgent string, appendToExisting bool) *UserAgentMiddleware {
	return &UserAgentMiddleware{
		userAgent: userAgent,
		append:    appendToExisting,
	}
}

// Name returns the middleware name
func (m *UserAgentMiddleware) Name() string {
	return "user-agent"
}

// Execute implements the Middleware interface
func (m *UserAgentMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	if m.userAgent != "" {
		if m.append && req.Header.Get("User-Agent") != "" {
			existing := req.Header.Get("User-Agent")
			req.Header.Set("User-Agent", fmt.Sprintf("%s %s", existing, m.userAgent))
		} else {
			req.Header.Set("User-Agent", m.userAgent)
		}
	}

	return next(ctx, req)
}
