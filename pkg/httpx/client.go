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

	// Auto-configure middlewares based on configuration
	if len(config.Middlewares) == 0 {
		var middlewares []Middleware

		// Add circuit breaker middleware if circuit breaker config is provided
		// Circuit breaker should be first to fail fast before retry attempts
		if config.CircuitBreakerConfig != nil {
			circuitBreakerMiddleware := NewCircuitBreakerMiddleware(*config.CircuitBreakerConfig)
			middlewares = append(middlewares, circuitBreakerMiddleware)
		}

		// Add retry middleware if retry policy is configured
		if config.RetryPolicy != nil {
			retryMiddleware := NewAdvancedRetryMiddleware(*config.RetryPolicy)
			middlewares = append(middlewares, retryMiddleware)
		}

		// Add logging middleware if logger is provided
		if config.Logger != nil {
			loggingMiddleware := NewLoggingMiddleware(config.Logger, config.LogLevel)
			middlewares = append(middlewares, loggingMiddleware)
		}

		config.Middlewares = middlewares
	} else {
		// If custom middlewares are provided but circuit breaker/retry policies are configured,
		// prepend them to ensure they run first (circuit breaker before retry)
		var prependMiddlewares []Middleware

		if config.CircuitBreakerConfig != nil {
			circuitBreakerMiddleware := NewCircuitBreakerMiddleware(*config.CircuitBreakerConfig)
			prependMiddlewares = append(prependMiddlewares, circuitBreakerMiddleware)
		}

		if config.RetryPolicy != nil {
			retryMiddleware := NewAdvancedRetryMiddleware(*config.RetryPolicy)
			prependMiddlewares = append(prependMiddlewares, retryMiddleware)
		}

		if len(prependMiddlewares) > 0 {
			config.Middlewares = append(prependMiddlewares, config.Middlewares...)
		}
	}

	// Create HTTP client with timeout and optional cookie jar
	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	// Wire up cookie jar if configured
	if config.CookieJar != nil {
		httpClient.Jar = config.CookieJar
	}

	return &Client{
		config:        config,
		clientOptions: config.ToClientOptions(), // For backward compatibility
		client:        httpClient,
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

	// Add default logging middleware if logger is provided
	if config.Logger != nil {
		loggingMiddleware := NewLoggingMiddleware(config.Logger, config.LogLevel)
		config.Middlewares = []Middleware{loggingMiddleware}
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

// WithDefaultBasicAuth is a function that sets default basic authentication for the client
func WithDefaultBasicAuth(username, password string) ClientOption {
	return func(c *ClientOptions) {
		c.BasicAuth = BasicAuth{
			Username: username,
			Password: password,
		}
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

// WithClientMiddleware adds middleware to the client's middleware chain
func WithClientMiddleware(middleware Middleware) ClientConfigOption {
	return func(c *ClientConfig) {
		if c.Middlewares == nil {
			c.Middlewares = make([]Middleware, 0)
		}
		c.Middlewares = append(c.Middlewares, middleware)
	}
}

// WithClientMiddlewares sets the complete middleware chain for the client
func WithClientMiddlewares(middlewares ...Middleware) ClientConfigOption {
	return func(c *ClientConfig) {
		c.Middlewares = middlewares
	}
}

// WithClientRetryPolicy sets the retry policy for all requests made by this client
func WithClientRetryPolicy(policy RetryPolicy) ClientConfigOption {
	return func(c *ClientConfig) {
		c.RetryPolicy = &policy
	}
}

// WithClientDefaultRetryPolicy enables default retry behavior for all requests
func WithClientDefaultRetryPolicy() ClientConfigOption {
	policy := DefaultRetryPolicy()
	return func(c *ClientConfig) {
		c.RetryPolicy = &policy
	}
}

// WithClientAggressiveRetryPolicy enables aggressive retry behavior for all requests
func WithClientAggressiveRetryPolicy() ClientConfigOption {
	policy := AggressiveRetryPolicy()
	return func(c *ClientConfig) {
		c.RetryPolicy = &policy
	}
}

// WithClientConservativeRetryPolicy enables conservative retry behavior for all requests
func WithClientConservativeRetryPolicy() ClientConfigOption {
	policy := ConservativeRetryPolicy()
	return func(c *ClientConfig) {
		c.RetryPolicy = &policy
	}
}

// WithClientCircuitBreaker sets the circuit breaker configuration for all requests made by this client
func WithClientCircuitBreaker(config CircuitBreakerConfig) ClientConfigOption {
	return func(c *ClientConfig) {
		c.CircuitBreakerConfig = &config
	}
}

// WithClientDefaultCircuitBreaker enables default circuit breaker behavior for all requests
func WithClientDefaultCircuitBreaker() ClientConfigOption {
	config := DefaultCircuitBreakerConfig()
	return func(c *ClientConfig) {
		c.CircuitBreakerConfig = &config
	}
}

// WithClientAggressiveCircuitBreaker enables aggressive circuit breaker behavior for all requests
func WithClientAggressiveCircuitBreaker() ClientConfigOption {
	config := AggressiveCircuitBreakerConfig()
	return func(c *ClientConfig) {
		c.CircuitBreakerConfig = &config
	}
}

// WithClientConservativeCircuitBreaker enables conservative circuit breaker behavior for all requests
func WithClientConservativeCircuitBreaker() ClientConfigOption {
	config := ConservativeCircuitBreakerConfig()
	return func(c *ClientConfig) {
		c.CircuitBreakerConfig = &config
	}
}

// WithClientCache enables HTTP caching with the specified configuration
func WithClientCache(config CacheConfig) ClientConfigOption {
	return func(c *ClientConfig) {
		cacheMiddleware := NewCacheMiddleware(config)
		c.Middlewares = append(c.Middlewares, cacheMiddleware)
	}
}

// WithClientDefaultCache enables HTTP caching with default settings
func WithClientDefaultCache() ClientConfigOption {
	return WithClientCache(CacheConfig{
		Backend:      NewInMemoryCache(1000),
		DefaultTTL:   5 * time.Minute,
		MaxSizeBytes: 10 * 1024 * 1024, // 10MB
	})
}

// WithClientRateLimit adds rate limiting to all requests
func WithClientRateLimit(config RateLimitConfig) ClientConfigOption {
	return func(c *ClientConfig) {
		rateLimitMiddleware := NewRateLimitMiddleware(config)
		c.Middlewares = append(c.Middlewares, rateLimitMiddleware)
	}
}

// WithClientDefaultRateLimit adds default rate limiting (10 req/sec with burst of 20)
func WithClientDefaultRateLimit() ClientConfigOption {
	return WithClientRateLimit(RateLimitConfig{
		Strategy:        RateLimitTokenBucket,
		RequestsPerSec:  10,
		BurstSize:       20,
		WaitOnLimit:     true,
		MaxWaitDuration: 30 * time.Second,
	})
}

// WithClientCompression enables automatic compression/decompression
func WithClientCompression(config CompressionConfig) ClientConfigOption {
	return func(c *ClientConfig) {
		compressionMiddleware := NewCompressionMiddleware(config)
		c.Middlewares = append(c.Middlewares, compressionMiddleware)
	}
}

// WithClientDefaultCompression enables compression with default settings
func WithClientDefaultCompression() ClientConfigOption {
	return WithClientCompression(DefaultCompressionConfig())
}

// WithClientPrometheusMetrics enables Prometheus metrics collection
func WithClientPrometheusMetrics(config PrometheusConfig) ClientConfigOption {
	return func(c *ClientConfig) {
		collector, err := NewPrometheusCollector(config)
		if err != nil {
			// Log error but don't fail client creation
			return
		}
		metricsMiddleware := NewMetricsMiddleware(collector)
		c.Middlewares = append(c.Middlewares, metricsMiddleware)
	}
}

// WithClientDefaultPrometheusMetrics enables Prometheus metrics with default settings
func WithClientDefaultPrometheusMetrics() ClientConfigOption {
	return WithClientPrometheusMetrics(DefaultPrometheusConfig())
}

// WithClientTracing enables OpenTelemetry distributed tracing
func WithClientTracing(config TracingConfig) ClientConfigOption {
	return func(c *ClientConfig) {
		tracingMiddleware := NewTracingMiddleware(config)
		// Tracing should be first middleware to capture all operations
		c.Middlewares = append([]Middleware{tracingMiddleware}, c.Middlewares...)
	}
}

// WithClientDefaultTracing enables OpenTelemetry tracing with default settings
func WithClientDefaultTracing() ClientConfigOption {
	return WithClientTracing(TracingConfig{})
}

// WithClientCookieJar enables automatic cookie management with a standard cookie jar
func WithClientCookieJar() ClientConfigOption {
	return func(c *ClientConfig) {
		manager, err := NewCookieJarManager()
		if err != nil {
			// Log error but don't fail client creation
			return
		}
		c.CookieJar = manager.Jar()
		c.CookieJarManager = manager
	}
}

// WithClientCookieJarManager enables cookie management with a custom cookie jar manager
// This allows for advanced cookie persistence and management utilities
func WithClientCookieJarManager(manager *CookieJarManager) ClientConfigOption {
	return func(c *ClientConfig) {
		if manager == nil {
			return
		}
		c.CookieJar = manager.Jar()
		c.CookieJarManager = manager
	}
}
