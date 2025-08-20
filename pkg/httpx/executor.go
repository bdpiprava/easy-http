package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// execute is a function that executes the request with given client and returns the response
func execute(client *Client, request *Request, respType any) (*Response, error) {
	if respType == nil {
		return nil, errors.New("response type cannot be nil")
	}

	// Build request options to check for streaming mode
	// Use new config architecture if available, fall back to old for compatibility
	var requestOpts RequestOptions
	if client.config.Timeout != 0 || client.config.Logger != nil {
		// Client was created with new architecture
		requestOpts = buildOptsFromConfig(client.config, request)
	} else {
		// Client was created with old architecture
		requestOpts = buildOpts(client.clientOptions, request)
	}

	// Build HTTP request using appropriate architecture
	var req *http.Request
	var err error
	var logger *slog.Logger
	var logLevel slog.Level

	if client.config.Timeout != 0 || client.config.Logger != nil {
		// Client was created with new architecture
		req, err = buildRequestFromConfig(requestOpts)
		logger = client.config.Logger
		logLevel = client.config.LogLevel
	} else {
		// Client was created with old architecture
		req, err = request.ToHTTPReq(client.clientOptions)
		logger = client.clientOptions.Logger
		logLevel = client.clientOptions.LogLevel
	}

	if err != nil {
		logError(logger, "Failed to build HTTP request", err, req)
		return nil, err
	}

	// Log the outgoing request
	logRequest(logger, logLevel, req)

	start := time.Now()
	resp, err := client.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		logError(logger, "Failed to execute HTTP request", err, req)
		return nil, errors.Wrap(err, "failed to execute request")
	}

	// Log the response
	logResponse(logger, logLevel, resp, duration)

	return newResponse(resp, respType, requestOpts.Streaming)
}

// logRequest logs the outgoing HTTP request with structured logging
func logRequest(logger *slog.Logger, minLevel slog.Level, req *http.Request) {
	if logger == nil || !logger.Enabled(context.Background(), minLevel) {
		return
	}

	logger.LogAttrs(context.Background(), slog.LevelDebug, "HTTP request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("host", req.Host),
		slog.Any("headers", req.Header),
	)
}

// logResponse logs the HTTP response with structured logging
func logResponse(logger *slog.Logger, minLevel slog.Level, resp *http.Response, duration time.Duration) {
	if logger == nil || !logger.Enabled(context.Background(), minLevel) {
		return
	}

	level := slog.LevelInfo
	if resp.StatusCode >= 400 {
		level = slog.LevelWarn
	}
	if resp.StatusCode >= 500 {
		level = slog.LevelError
	}

	logger.LogAttrs(context.Background(), level, "HTTP response",
		slog.Int("status_code", resp.StatusCode),
		slog.String("status", resp.Status),
		slog.Duration("duration", duration),
		slog.String("content_length", resp.Header.Get("Content-Length")),
		slog.String("content_type", resp.Header.Get("Content-Type")),
	)
}

// buildRequestFromConfig builds an HTTP request using the new configuration architecture
func buildRequestFromConfig(opts RequestOptions) (*http.Request, error) {
	// Check for errors that occurred during option processing
	if opts.Error != nil {
		return nil, opts.Error
	}

	if _, ok := supportedMethods[strings.ToUpper(opts.Method)]; !ok {
		return nil, errors.Errorf("unsupported method: %s", opts.Method)
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.BaseURL, opts.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.URL.Path = path.Join(req.URL.Path, opts.Path)
	req.Header = opts.Headers
	req.URL.RawQuery = opts.QueryParams.Encode()

	// Apply basic auth if specified
	if opts.BasicAuth.Username != "" || opts.BasicAuth.Password != "" {
		req.SetBasicAuth(opts.BasicAuth.Username, opts.BasicAuth.Password)
	}

	return req, nil
}

// logError logs errors with structured logging and request context
func logError(logger *slog.Logger, message string, err error, req *http.Request) {
	if logger == nil {
		return
	}

	attrs := []slog.Attr{
		slog.String("error", err.Error()),
	}

	if req != nil {
		attrs = append(attrs,
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
		)
	}

	logger.LogAttrs(context.Background(), slog.LevelError, message, attrs...)
}
