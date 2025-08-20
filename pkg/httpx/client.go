package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

const (
	// defaultTimeout is the default timeout for the client
	defaultTimeout = 10 * time.Second
)

// Client is a struct that holds the options and base URL for the client
type Client struct {
	config        ClientConfig  // New structured configuration
	clientOptions ClientOptions // Deprecated: kept for backward compatibility
	client        *http.Client
}

// NewClientWithConfig creates a new client with the improved configuration architecture
func NewClientWithConfig(opts ...ClientConfigOption) *Client {
	config := ClientConfig{
		Timeout:          defaultTimeout,
		LogLevel:         slog.LevelInfo,
		DefaultHeaders:   make(http.Header),
		DefaultBaseURL:   "",
		DefaultBasicAuth: BasicAuth{},
	}

	for _, opt := range opts {
		opt(&config)
	}

	return &Client{
		config:        config,
		clientOptions: config.ToClientOptions(), // For backward compatibility
		client:        &http.Client{Timeout: config.Timeout},
	}
}

// NewClient is a function that returns a new client with the given options and base URL
// Deprecated: Use NewClientWithConfig for better separation of concerns
func NewClient(opts ...ClientOption) *Client {
	cOpts := ClientOptions{
		Timeout:  defaultTimeout,
		Headers:  http.Header{},
		BaseURL:  "",
		LogLevel: slog.LevelInfo, // Default log level
	}
	for _, opt := range opts {
		opt(&cOpts)
	}

	// Convert to new config for consistency
	config := ClientConfig{
		Timeout:          cOpts.Timeout,
		Logger:           cOpts.Logger,
		LogLevel:         cOpts.LogLevel,
		DefaultBaseURL:   cOpts.BaseURL,
		DefaultHeaders:   cOpts.Headers,
		DefaultBasicAuth: cOpts.BasicAuth,
	}

	return &Client{
		config:        config,
		clientOptions: cOpts,
		client:        &http.Client{Timeout: cOpts.Timeout},
	}
}

// Execute executes the request and returns the response or an error
func (c Client) Execute(req Request, respType any) (*Response, error) {
	return execute(&c, &req, respType)
}

// WithDefaultTimeout is a function that sets the timeout for the client
func WithDefaultTimeout(timeout time.Duration) ClientOption {
	return func(c *ClientOptions) {
		c.Timeout = timeout
	}
}

// WithDefaultBaseURL is a function that sets the base URL for the client
func WithDefaultBaseURL(baseURL string) ClientOption {
	return func(c *ClientOptions) {
		c.BaseURL = baseURL
	}
}

// WithDefaultHeaders is a function that sets the headers for the client
func WithDefaultHeaders(headers http.Header) ClientOption {
	return func(c *ClientOptions) {
		if headers == nil {
			return
		}

		for k, v := range headers {
			c.Headers[k] = v
		}
	}
}

// WithDefaultHeader is a function that sets the headers for the client
func WithDefaultHeader(key string, values ...string) ClientOption {
	return func(c *ClientOptions) {
		if c.Headers == nil {
			c.Headers = http.Header{}
		}

		if cur, ok := c.Headers[key]; ok {
			c.Headers[key] = append(cur, values...)
			return
		}

		c.Headers[key] = values
	}
}

// WithLogger is a function that sets a structured logger for the client
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *ClientOptions) {
		c.Logger = logger
	}
}

// WithLogLevel is a function that sets the minimum log level for HTTP operations
func WithLogLevel(level slog.Level) ClientOption {
	return func(c *ClientOptions) {
		c.LogLevel = level
	}
}

// New ClientConfigOption functions for improved architecture

// WithClientTimeout sets the default timeout for all requests
func WithClientTimeout(timeout time.Duration) ClientConfigOption {
	return func(c *ClientConfig) {
		c.Timeout = timeout
	}
}

// WithClientLogger sets the structured logger for the client
func WithClientLogger(logger *slog.Logger) ClientConfigOption {
	return func(c *ClientConfig) {
		c.Logger = logger
	}
}

// WithClientLogLevel sets the minimum log level for HTTP operations
func WithClientLogLevel(level slog.Level) ClientConfigOption {
	return func(c *ClientConfig) {
		c.LogLevel = level
	}
}

// WithClientDefaultBaseURL sets the default base URL for all requests
func WithClientDefaultBaseURL(baseURL string) ClientConfigOption {
	return func(c *ClientConfig) {
		c.DefaultBaseURL = baseURL
	}
}

// WithClientDefaultHeaders sets default headers that will be applied to all requests
func WithClientDefaultHeaders(headers http.Header) ClientConfigOption {
	return func(c *ClientConfig) {
		if c.DefaultHeaders == nil {
			c.DefaultHeaders = make(http.Header)
		}
		for key, values := range headers {
			c.DefaultHeaders[key] = values
		}
	}
}

// WithClientDefaultHeader sets a single default header that will be applied to all requests
func WithClientDefaultHeader(key string, values ...string) ClientConfigOption {
	return func(c *ClientConfig) {
		if c.DefaultHeaders == nil {
			c.DefaultHeaders = make(http.Header)
		}
		c.DefaultHeaders[key] = values
	}
}

// WithClientDefaultBasicAuth sets default basic authentication for all requests
func WithClientDefaultBasicAuth(username, password string) ClientConfigOption {
	return func(c *ClientConfig) {
		c.DefaultBasicAuth = BasicAuth{
			Username: username,
			Password: password,
		}
	}
}
