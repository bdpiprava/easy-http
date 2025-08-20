package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// BasicAuth is a struct that holds the username and password for basic authentication
type BasicAuth struct {
	Username string
	Password string
}

// ClientConfig holds configuration that applies to all requests made by this client
type ClientConfig struct {
	// Client-level connection settings
	Timeout  time.Duration // Default timeout for all requests
	Logger   *slog.Logger  // Optional structured logger for all requests
	LogLevel slog.Level    // Minimum log level for HTTP operations

	// Default values that can be overridden per request
	DefaultBaseURL   string      // Default base URL for requests
	DefaultHeaders   http.Header // Default headers applied to all requests
	DefaultBasicAuth BasicAuth   // Default basic auth for all requests
}

// ClientOptions is a struct that holds the options for the client
// Deprecated: Use ClientConfig for new code. Maintained for backward compatibility.
type ClientOptions struct {
	BaseURL   string
	Headers   http.Header
	BasicAuth BasicAuth
	Timeout   time.Duration
	Logger    *slog.Logger // Optional structured logger
	LogLevel  slog.Level   // Minimum log level for HTTP operations
}

// ClientOption is a function that takes a pointer to Options and modifies it
type ClientOption func(client *ClientOptions)

// RequestConfig holds configuration specific to a single request
type RequestConfig struct {
	// Required request settings
	Method string // HTTP method (GET, POST, etc.)

	// URL components
	BaseURL string // Base URL for this request (overrides client default)
	Path    string // Path to append to base URL

	// Request modifiers
	Headers     http.Header // Headers for this request (merged with client defaults)
	QueryParams url.Values  // Query parameters for this request
	Body        io.Reader   // Request body
	BasicAuth   BasicAuth   // Basic auth for this request (overrides client default)

	// Request behavior
	Context   context.Context // Request context for cancellation/timeout
	Timeout   time.Duration   // Request timeout (overrides client default)
	Streaming bool            // If true, response body will not be read into memory

	// Internal
	Error error // Stores errors from RequestOptions that can't return errors directly
}

// RequestOptions is a struct that holds the options for the request
// Deprecated: Use RequestConfig for new code. Maintained for backward compatibility.
type RequestOptions struct {
	Method      string
	BaseURL     string
	Headers     http.Header
	QueryParams url.Values
	Body        io.Reader
	BasicAuth   BasicAuth
	Path        string
	Timeout     time.Duration
	Context     context.Context
	Error       error // Stores errors from RequestOptions that can't return errors directly
	Streaming   bool  // If true, response body will not be read into memory
}

// ClientConfigOption is a function that modifies ClientConfig
type ClientConfigOption func(*ClientConfig)

// RequestConfigOption is a function that modifies RequestConfig
type RequestConfigOption func(*RequestConfig)

// RequestOption is a function that takes a pointer to Options and modifies it
type RequestOption func(*RequestOptions)

// ToClientOptions converts ClientConfig to ClientOptions for backward compatibility
func (c ClientConfig) ToClientOptions() ClientOptions {
	return ClientOptions{
		BaseURL:   c.DefaultBaseURL,
		Headers:   c.DefaultHeaders,
		BasicAuth: c.DefaultBasicAuth,
		Timeout:   c.Timeout,
		Logger:    c.Logger,
		LogLevel:  c.LogLevel,
	}
}

// ToRequestOptions converts RequestConfig to RequestOptions for backward compatibility
func (r RequestConfig) ToRequestOptions() RequestOptions {
	return RequestOptions{
		Method:      r.Method,
		BaseURL:     r.BaseURL,
		Headers:     r.Headers,
		QueryParams: r.QueryParams,
		Body:        r.Body,
		BasicAuth:   r.BasicAuth,
		Path:        r.Path,
		Timeout:     r.Timeout,
		Context:     r.Context,
		Error:       r.Error,
		Streaming:   r.Streaming,
	}
}

// MergeWithDefaults merges client defaults with request-specific config
func (r *RequestConfig) MergeWithDefaults(clientConfig ClientConfig) {
	// Use client defaults if request doesn't specify
	if r.BaseURL == "" {
		r.BaseURL = clientConfig.DefaultBaseURL
	}

	if r.Timeout == 0 {
		r.Timeout = clientConfig.Timeout
	}

	// Merge headers: client defaults first, then request-specific
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}

	// Copy client default headers first
	for key, values := range clientConfig.DefaultHeaders {
		if _, exists := r.Headers[key]; !exists {
			r.Headers[key] = values
		}
	}

	// Use client default basic auth if request doesn't specify
	if r.BasicAuth.Username == "" && r.BasicAuth.Password == "" {
		r.BasicAuth = clientConfig.DefaultBasicAuth
	}
}
