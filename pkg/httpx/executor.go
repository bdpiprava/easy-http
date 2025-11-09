package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"strings"

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

	// Always use middleware execution for clients with new config architecture
	// This includes both new clients and old clients converted to new architecture
	return executeWithMiddleware(client, request, requestOpts, respType)
}

// executeWithMiddleware executes the request using the new architecture with middleware support
func executeWithMiddleware(client *Client, _ *Request, requestOpts RequestOptions, respType any) (*Response, error) {
	// Build the HTTP request
	req, err := buildRequestFromConfig(requestOpts)
	if err != nil {
		// Classify the error for better context
		httpErr := ClassifyError(err, req, nil)
		if client.config.Logger != nil {
			logError(client.config.Logger, "Failed to build HTTP request", httpErr, req)
		}
		return nil, httpErr
	}

	// Create the final handler that performs the actual HTTP call
	// Handle DisableCookies by using a temporary client without cookie jar
	finalHandler := func(_ context.Context, httpReq *http.Request) (*http.Response, error) {
		if requestOpts.DisableCookies && client.client.Jar != nil {
			// Create temporary client without cookie jar for this request
			tempClient := &http.Client{
				Timeout: client.client.Timeout,
				// Copy other settings but omit Jar
				CheckRedirect: client.client.CheckRedirect,
				Transport:     client.client.Transport,
			}
			return tempClient.Do(httpReq)
		}
		return client.client.Do(httpReq)
	}

	// Create middleware chain
	chain := NewMiddlewareChain(finalHandler)
	for _, middleware := range client.config.Middlewares {
		chain.Add(middleware)
	}

	// Execute the middleware chain
	ctx := req.Context()
	resp, err := chain.Execute(ctx, req)
	if err != nil {
		// Classify and enhance the error with context
		httpErr := ClassifyError(err, req, resp)
		return nil, httpErr
	}

	return newResponse(resp, respType, requestOpts.Streaming)
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

	// Add cookies to request
	for _, cookie := range opts.Cookies {
		req.AddCookie(cookie)
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
