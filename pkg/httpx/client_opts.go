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

	// Proxy configuration
	ProxyURL    string       // HTTP/HTTPS/SOCKS proxy URL (e.g., "http://proxy.company.com:8080", "socks5://localhost:1080")
	ProxyAuth   BasicAuth    // Proxy authentication credentials
	NoProxy     []string     // Domains to bypass proxy (e.g., "localhost", "*.internal.com", "192.168.0.0/16")
	ProxyConfig *ProxyConfig // Internal proxy configuration (automatically populated from ProxyURL/ProxyAuth/NoProxy)

	// Retry configuration
	RetryPolicy *RetryPolicy // Optional retry policy for all requests

	// Circuit breaker configuration
	CircuitBreakerConfig *CircuitBreakerConfig // Optional circuit breaker for fault tolerance

	// Cookie management
	CookieJar        http.CookieJar    // Automatic cookie jar for managing cookies across requests
	CookieJarManager *CookieJarManager // Optional cookie jar manager with persistence utilities

	// Middleware configuration
	Middlewares []Middleware // Ordered list of middlewares to apply to all requests
}

// ClientOptions is a struct that holds the options for the client
//
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
	Context        context.Context // Request context for cancellation/timeout
	Timeout        time.Duration   // Request timeout (overrides client default)
	Streaming      bool            // If true, response body will not be read into memory
	Cookies        []*http.Cookie  // Cookies to add to this specific request
	DisableCookies bool            // If true, disables cookie jar for this specific request

	// Proxy configuration (overrides client proxy for this specific request)
	ProxyURL     string    // Proxy URL for this request (overrides client proxy)
	ProxyAuth    BasicAuth // Proxy auth for this request
	DisableProxy bool      // If true, disables proxy for this specific request

	// Internal
	Error error // Stores errors from RequestOptions that can't return errors directly
}

// RequestOptions is a struct that holds the options for the request
//
// Deprecated: Use RequestConfig for new code. Maintained for backward compatibility.
type RequestOptions struct {
	Method         string
	BaseURL        string
	Headers        http.Header
	QueryParams    url.Values
	Body           io.Reader
	BasicAuth      BasicAuth
	Path           string
	Timeout        time.Duration
	Context        context.Context
	Error          error          // Stores errors from RequestOptions that can't return errors directly
	Streaming      bool           // If true, response body will not be read into memory
	Cookies        []*http.Cookie // Cookies to add to this specific request
	DisableCookies bool           // If true, disables cookie jar for this specific request
	ProxyURL       string         // Proxy URL for this request (overrides client proxy)
	ProxyAuth      BasicAuth      // Proxy auth for this request
	DisableProxy   bool           // If true, disables proxy for this specific request
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
		Method:         r.Method,
		BaseURL:        r.BaseURL,
		Headers:        r.Headers,
		QueryParams:    r.QueryParams,
		Body:           r.Body,
		BasicAuth:      r.BasicAuth,
		Path:           r.Path,
		Timeout:        r.Timeout,
		Context:        r.Context,
		Error:          r.Error,
		Streaming:      r.Streaming,
		Cookies:        r.Cookies,
		DisableCookies: r.DisableCookies,
		ProxyURL:       r.ProxyURL,
		ProxyAuth:      r.ProxyAuth,
		DisableProxy:   r.DisableProxy,
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
