package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// execute is a function that executes the request with given client and returns the response
func execute(client *Client, request *Request, respType any) (*Response, error) {
	if respType == nil {
		return nil, errors.New("response type cannot be nil")
	}

	// Build request options to check for streaming mode
	requestOpts := buildOpts(client.clientOptions, request)

	req, err := request.ToHTTPReq(client.clientOptions)
	if err != nil {
		logError(client.clientOptions.Logger, "Failed to build HTTP request", err, req)
		return nil, err
	}

	// Log the outgoing request
	logRequest(client.clientOptions.Logger, client.clientOptions.LogLevel, req)

	start := time.Now()
	resp, err := client.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		logError(client.clientOptions.Logger, "Failed to execute HTTP request", err, req)
		return nil, errors.Wrap(err, "failed to execute request")
	}

	// Log the response
	logResponse(client.clientOptions.Logger, client.clientOptions.LogLevel, resp, duration)

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
