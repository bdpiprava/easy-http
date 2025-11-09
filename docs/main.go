package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// serverTimeout is the timeout for the server
const serverTimeout = 3 * time.Second

type Example struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	FullDescription string         `json:"fullDescription,omitempty"`
	Code            string         `json:"code"`
	Category        string         `json:"category,omitempty"`
	CategoryIcon    string         `json:"categoryIcon,omitempty"`
	Icon            string         `json:"icon,omitempty"`
	Options         []ConfigOption `json:"options,omitempty"`
}

type ConfigOption struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type TemplateData struct {
	Examples      []*Example               `json:"examples"`
	ConfigOptions map[string]ConfigOption  `json:"configOptions"`
	Version       string                   `json:"version"`
	GithubRepo    string                   `json:"githubRepo"`
}

var generate = flag.Bool("generate", false, "Generate the static files")

func main() {
	flag.Parse()

	if *generate {
		println("Generating static files")
		buildStatic()
		return
	}

	// Serve static files from build directory for local development
	staticServer := http.FileServer(http.Dir("./build/docs"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		buildStatic()
		staticServer.ServeHTTP(w, r)
	})

	println("Starting server at http://localhost:8090")
	server := &http.Server{
		Addr:              ":8090",
		ReadHeaderTimeout: serverTimeout,
	}

	_ = server.ListenAndServe()
}

func buildStatic() {
	tmpl, err := template.New("index.html").ParseFiles("./docs/template/index.html")
	if err != nil {
		panic(err)
	}

	f, err := os.Create("./build/docs/index.html")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	exs := getExamples()

	templateData := TemplateData{
		Examples:      exs,
		ConfigOptions: getConfigOptions(),
		Version:       "v1.0.0",
		GithubRepo:    "https://github.com/bdpiprava/easy-http",
	}

	err = tmpl.Execute(f, templateData)
	if err != nil {
		panic(err)
	}
}

func readFuncBodyIgnoreError(fn reflect.Value) string {
	body, _ := readFuncBody(fn)
	return fmt.Sprintf("func example()%s}", body)
}

func readFuncBody(fn reflect.Value) (string, error) {
	p := fn.Pointer()
	fc := runtime.FuncForPC(p)
	filename, line := fc.FileLine(p)
	fset := token.NewFileSet()
	// parse file to AST tree
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}
	// walk and find the function block
	find := &FindBlockByLine{Fset: fset, Line: line}
	ast.Walk(find, node)

	if find.Block != nil {
		fp, err := os.Open(filename)
		if err != nil {
			return "", err
		}
		defer fp.Close()
		_, _ = fp.Seek(int64(find.Block.Lbrace-1), 0)
		buf := make([]byte, int64(find.Block.Rbrace-find.Block.Lbrace))
		_, err = io.ReadFull(fp, buf)
		if err != nil {
			return "", err
		}

		return string(buf), nil
	}
	return "", nil
}

// FindBlockByLine is a ast.Visitor implementation that finds a block by line.
type FindBlockByLine struct {
	Fset  *token.FileSet
	Line  int
	Block *ast.BlockStmt
}

// Visit implements the ast.Visitor interface.
func (f *FindBlockByLine) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	if blockStmt, ok := node.(*ast.BlockStmt); ok {
		stmtStartingPosition := blockStmt.Pos()
		stmtLine := f.Fset.Position(stmtStartingPosition).Line
		if stmtLine == f.Line {
			f.Block = blockStmt
			return nil
		}
	}
	return f
}

func getConfigOptions() map[string]ConfigOption {
	return map[string]ConfigOption{
		// Client Configuration
		"WithDefaultBaseURL": {
			Name:        "WithDefaultBaseURL",
			Type:        "string",
			Description: "Sets the default base URL for all requests made by this client. The base URL will be prepended to all request paths.",
		},
		"WithDefaultTimeout": {
			Name:        "WithDefaultTimeout",
			Type:        "time.Duration",
			Description: "Sets the default timeout for all requests. Requests will fail if they don't complete within this duration.",
		},
		"WithDefaultHeader": {
			Name:        "WithDefaultHeader",
			Type:        "string, string",
			Description: "Adds a default header that will be included in all requests made by this client.",
		},
		"WithDefaultHeaders": {
			Name:        "WithDefaultHeaders",
			Type:        "http.Header",
			Description: "Sets multiple default headers at once using http.Header type.",
		},
		"WithClientLogger": {
			Name:        "WithClientLogger",
			Type:        "*slog.Logger",
			Description: "Sets a structured logger for the client. Enables detailed logging of requests, responses, and errors.",
		},
		"WithClientLogLevel": {
			Name:        "WithClientLogLevel",
			Type:        "slog.Level",
			Description: "Sets the logging level (Debug, Info, Warn, Error). Controls which log messages are output.",
		},

		// Circuit Breaker Configuration
		"WithClientDefaultCircuitBreaker": {
			Name:        "WithClientDefaultCircuitBreaker",
			Type:        "void",
			Description: "Enables circuit breaker with default configuration. Protects against cascading failures by temporarily blocking requests to failing services.",
		},
		"WithClientCircuitBreaker": {
			Name:        "WithClientCircuitBreaker",
			Type:        "CircuitBreakerConfig",
			Description: "Enables circuit breaker with custom configuration. Allows fine-tuning of thresholds, timeouts, and state change callbacks.",
		},
		"DefaultCircuitBreakerConfig": {
			Name:        "DefaultCircuitBreakerConfig",
			Type:        "CircuitBreakerConfig",
			Description: "Returns a balanced circuit breaker configuration. Trips when 50% of requests fail with at least 5 requests in the window.",
		},
		"AggressiveCircuitBreakerConfig": {
			Name:        "AggressiveCircuitBreakerConfig",
			Type:        "CircuitBreakerConfig",
			Description: "Returns an aggressive circuit breaker configuration. Trips quickly with lower thresholds, suitable for critical services.",
		},
		"ConservativeCircuitBreakerConfig": {
			Name:        "ConservativeCircuitBreakerConfig",
			Type:        "CircuitBreakerConfig",
			Description: "Returns a conservative circuit breaker configuration. Allows more failures before tripping, suitable for less critical services.",
		},

		// Retry Configuration
		"WithClientDefaultRetryPolicy": {
			Name:        "WithClientDefaultRetryPolicy",
			Type:        "void",
			Description: "Enables automatic retry with default policy. Retries failed requests up to 3 times with exponential backoff.",
		},
		"WithClientRetryPolicy": {
			Name:        "WithClientRetryPolicy",
			Type:        "RetryPolicy",
			Description: "Enables automatic retry with custom policy. Allows configuration of max attempts, backoff strategy, and retry conditions.",
		},
		"DefaultRetryPolicy": {
			Name:        "DefaultRetryPolicy",
			Type:        "RetryPolicy",
			Description: "Returns a balanced retry policy with 3 max attempts and exponential backoff.",
		},
		"AggressiveRetryPolicy": {
			Name:        "AggressiveRetryPolicy",
			Type:        "RetryPolicy",
			Description: "Returns an aggressive retry policy with 5 max attempts and shorter delays.",
		},
		"ConservativeRetryPolicy": {
			Name:        "ConservativeRetryPolicy",
			Type:        "RetryPolicy",
			Description: "Returns a conservative retry policy with 2 max attempts and longer delays.",
		},

		// Caching Configuration
		"WithClientDefaultCache": {
			Name:        "WithClientDefaultCache",
			Type:        "void",
			Description: "Enables HTTP caching with default configuration. Respects standard HTTP cache headers (Cache-Control, ETag, etc).",
		},

		// Request Configuration
		"WithBaseURL": {
			Name:        "WithBaseURL",
			Type:        "string",
			Description: "Sets the base URL for this specific request. Overrides the client's default base URL.",
		},
		"WithPath": {
			Name:        "WithPath",
			Type:        "string",
			Description: "Sets the URL path for the request. Will be appended to the base URL.",
		},
		"WithQueryParam": {
			Name:        "WithQueryParam",
			Type:        "string, string",
			Description: "Adds a single query parameter to the request URL.",
		},
		"WithQueryParams": {
			Name:        "WithQueryParams",
			Type:        "map[string]string",
			Description: "Adds multiple query parameters at once using a map.",
		},
		"WithHeader": {
			Name:        "WithHeader",
			Type:        "string, string",
			Description: "Adds a single header to the request.",
		},
		"WithHeaders": {
			Name:        "WithHeaders",
			Type:        "http.Header",
			Description: "Adds multiple headers at once using http.Header type.",
		},
		"WithJSONBody": {
			Name:        "WithJSONBody",
			Type:        "interface{}",
			Description: "Sets the request body as JSON. Automatically marshals the provided struct/map and sets Content-Type to application/json.",
		},
		"WithBody": {
			Name:        "WithBody",
			Type:        "io.Reader",
			Description: "Sets the request body from an io.Reader. Useful for streaming or custom content types.",
		},
		"WithContext": {
			Name:        "WithContext",
			Type:        "context.Context",
			Description: "Sets the context for the request. Useful for timeouts, cancellation, and passing request-scoped values.",
		},
		"WithBasicAuth": {
			Name:        "WithBasicAuth",
			Type:        "string, string",
			Description: "Adds HTTP Basic authentication to the request using username and password.",
		},
		"WithBearerToken": {
			Name:        "WithBearerToken",
			Type:        "string",
			Description: "Adds Bearer token authentication to the request. Sets the Authorization header with 'Bearer <token>'.",
		},
		"WithStreaming": {
			Name:        "WithStreaming",
			Type:        "void",
			Description: "Enables streaming mode for the response. The response body won't be automatically read, allowing streaming consumption.",
		},

		// Proxy Configuration
		"WithHTTPProxy": {
			Name:        "WithHTTPProxy",
			Type:        "string",
			Description: "Sets an HTTP proxy for the request. Format: http://proxy-host:port",
		},
		"WithHTTPSProxy": {
			Name:        "WithHTTPSProxy",
			Type:        "string",
			Description: "Sets an HTTPS proxy for the request. Format: https://proxy-host:port",
		},
		"WithSOCKS5Proxy": {
			Name:        "WithSOCKS5Proxy",
			Type:        "string",
			Description: "Sets a SOCKS5 proxy for the request. Format: socks5://proxy-host:port",
		},
		"WithProxyAuth": {
			Name:        "WithProxyAuth",
			Type:        "string, string",
			Description: "Sets authentication credentials for the proxy server (username, password).",
		},

		// Cookie Configuration
		"WithCookie": {
			Name:        "WithCookie",
			Type:        "*http.Cookie",
			Description: "Adds a single cookie to the request.",
		},
		"WithCookies": {
			Name:        "WithCookies",
			Type:        "[]*http.Cookie",
			Description: "Adds multiple cookies to the request.",
		},
		"WithCookieJar": {
			Name:        "WithCookieJar",
			Type:        "http.CookieJar",
			Description: "Sets a cookie jar for automatic cookie management across requests.",
		},

		// Middleware Configuration
		"WithClientMiddleware": {
			Name:        "WithClientMiddleware",
			Type:        "Middleware",
			Description: "Adds a custom middleware to the client. Middleware can intercept and modify requests/responses.",
		},
		"WithClientMiddlewares": {
			Name:        "WithClientMiddlewares",
			Type:        "[]Middleware",
			Description: "Adds multiple middlewares to the client. Middlewares are executed in the order they are added.",
		},

		// Tracing Configuration
		"WithTracing": {
			Name:        "WithTracing",
			Type:        "string",
			Description: "Enables OpenTelemetry distributed tracing with the specified service name.",
		},

		// Convenience Methods
		"GET": {
			Name:        "GET[T]",
			Type:        "generic function",
			Description: "Performs a GET request and automatically unmarshals the response into type T. Simplified API for common GET requests.",
		},
		"POST": {
			Name:        "POST[T]",
			Type:        "generic function",
			Description: "Performs a POST request and automatically unmarshals the response into type T. Simplified API for common POST requests.",
		},
		"PUT": {
			Name:        "PUT[T]",
			Type:        "generic function",
			Description: "Performs a PUT request and automatically unmarshals the response into type T. Simplified API for common PUT requests.",
		},
		"DELETE": {
			Name:        "DELETE[T]",
			Type:        "generic function",
			Description: "Performs a DELETE request and automatically unmarshals the response into type T. Simplified API for common DELETE requests.",
		},
		"PATCH": {
			Name:        "PATCH[T]",
			Type:        "generic function",
			Description: "Performs a PATCH request and automatically unmarshals the response into type T. Simplified API for common PATCH requests.",
		},
	}
}

func getOptions(names ...string) []ConfigOption {
	allOptions := getConfigOptions()
	var result []ConfigOption
	for _, name := range names {
		if opt, ok := allOptions[name]; ok {
			result = append(result, opt)
		}
	}
	return result
}

func getExamples() []*Example {
	return []*Example{
		// Getting Started
		{
			Name:            "Simple GET Request",
			Description:     `Make a basic GET request using the convenience function with automatic JSON unmarshaling`,
			FullDescription: `Demonstrates the simplest way to make a GET request using Easy-HTTP's convenience function. The response is automatically unmarshaled into your desired type using Go generics, eliminating boilerplate code. This approach is ideal for quick API integrations, prototyping, or when you don't need advanced configuration like retry policies or circuit breakers. Perfect for straightforward REST API calls where you want clean, readable code.`,
			Code:            exampleSimpleGET(),
			Category:        "Getting Started",
			CategoryIcon:    "fa-rocket",
			Icon:            "fa-play",
			Options:         getOptions("GET", "WithBaseURL", "WithPath"),
		},
		{
			Name:            "Simple POST Request",
			Description:     `Make a POST request with JSON body using type-safe generic API`,
			FullDescription: `Shows how to perform a POST request with a JSON body using type-safe generics. The library automatically marshals your struct to JSON and sets the appropriate Content-Type header. The response is then unmarshaled back into your specified type. This is essential for creating resources in REST APIs, submitting forms, or any scenario where you need to send structured data to a server. The type safety ensures compile-time checks and reduces runtime errors.`,
			Code:            exampleSimplePOST(),
			Category:        "Getting Started",
			CategoryIcon:    "fa-rocket",
			Icon:            "fa-paper-plane",
			Options:         getOptions("POST", "WithBaseURL", "WithPath", "WithJSONBody"),
		},
		{
			Name:            "Using Configured Client",
			Description:     `Create a client with default configuration that applies to all requests`,
			FullDescription: `Demonstrates creating a reusable HTTP client with default configuration that applies to all requests. This pattern is crucial for production applications where you want to set base URLs, timeouts, headers, and other settings once rather than repeating them for each request. Use this when building API clients for specific services, implementing SDK wrappers, or when you need consistent behavior across multiple HTTP calls in your application.`,
			Code:            exampleConfiguredClient(),
			Category:        "Getting Started",
			CategoryIcon:    "fa-rocket",
			Icon:            "fa-cog",
			Options:         getOptions("WithDefaultBaseURL", "WithDefaultTimeout", "WithDefaultHeader"),
		},

		// Request Configuration
		{
			Name:            "Query Parameters",
			Description:     `Add query parameters to requests for filtering, pagination, and sorting`,
			FullDescription: `Illustrates how to add query parameters to your HTTP requests for filtering, pagination, sorting, and search functionality. Query parameters are essential for REST APIs that support these operations. This example shows both single parameter addition and demonstrates how you can chain multiple parameters together. Use this pattern when implementing list endpoints with filtering capabilities, search functionality, or any API that requires URL query strings.`,
			Code:            exampleQueryParams(),
			Category:        "Request Configuration",
			CategoryIcon:    "fa-sliders-h",
			Icon:            "fa-filter",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithQueryParam"),
		},
		{
			Name:            "Custom Headers",
			Description:     `Add custom headers to requests for API versioning, client identification, etc`,
			FullDescription: `Demonstrates adding custom HTTP headers to your requests. Headers are crucial for API versioning, client identification, content negotiation, and custom metadata. This pattern is commonly used when working with APIs that require specific headers for rate limiting, tracing, authentication tokens, or custom business logic. Essential for enterprise integrations where API gateways and proxies rely on headers for routing and policy enforcement.`,
			Code:            exampleCustomHeaders(),
			Category:        "Request Configuration",
			CategoryIcon:    "fa-sliders-h",
			Icon:            "fa-heading",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithHeader"),
		},
		{
			Name:            "Request Body",
			Description:     `Send JSON request body using WithJSONBody for POST/PUT operations`,
			FullDescription: `Shows how to send structured JSON data in request bodies for POST and PUT operations. The library handles marshaling your Go structs or maps to JSON automatically, setting the correct Content-Type header. This is the standard approach for creating or updating resources in RESTful APIs. Use this for any operation that requires sending complex structured data, such as user registration, resource creation, or batch updates.`,
			Code:            exampleJSONBody(),
			Category:        "Request Configuration",
			CategoryIcon:    "fa-sliders-h",
			Icon:            "fa-file-code",
			Options:         getOptions("POST", "WithBaseURL", "WithPath", "WithJSONBody"),
		},
		{
			Name:            "Context and Timeouts",
			Description:     `Use context for request cancellation, timeouts, and passing request-scoped values`,
			FullDescription: `Demonstrates using Go's context package for controlling request lifecycle, implementing timeouts, and cancellation. Context is critical for production applications where you need to enforce SLAs, handle graceful shutdowns, or propagate deadlines through your call stack. This pattern prevents requests from hanging indefinitely and allows you to implement proper timeout behavior. Essential for microservices architectures and when integrating with external APIs that may be slow or unresponsive.`,
			Code:            exampleContextTimeout(),
			Category:        "Request Configuration",
			CategoryIcon:    "fa-sliders-h",
			Icon:            "fa-clock",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithContext"),
		},

		// Authentication
		{
			Name:            "Basic Authentication",
			Description:     `Use HTTP Basic authentication with username and password`,
			FullDescription: `Shows how to implement HTTP Basic authentication by providing username and password credentials. While not the most secure authentication method, Basic Auth is still widely used for internal APIs, development environments, and services that are secured at the network level. The library automatically encodes credentials and sets the Authorization header. Use this for legacy systems, internal tools, or when specifically required by the API you're integrating with.`,
			Code:            exampleBasicAuth(),
			Category:        "Authentication",
			CategoryIcon:    "fa-lock",
			Icon:            "fa-key",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithBasicAuth"),
		},
		{
			Name:            "Bearer Token",
			Description:     `Use Bearer token authentication for JWT or OAuth access tokens`,
			FullDescription: `Demonstrates Bearer token authentication, the standard method for JWT tokens and OAuth 2.0 access tokens. This is the most common authentication pattern in modern REST APIs and microservices. The library sets the Authorization header with the Bearer scheme automatically. Use this pattern when integrating with OAuth 2.0 providers, JWT-based authentication systems, or any modern API that issues access tokens. Essential for secure API access in production environments.`,
			Code:            exampleBearerToken(),
			Category:        "Authentication",
			CategoryIcon:    "fa-lock",
			Icon:            "fa-shield-alt",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithBearerToken"),
		},
		{
			Name:            "API Key Authentication",
			Description:     `Use custom header-based API key authentication`,
			FullDescription: `Illustrates API key authentication using custom headers, a simple and effective method for authenticating service-to-service communication. Many APIs use custom header names like X-API-Key for authentication. This pattern is common in cloud services, third-party integrations, and internal microservices where you need simple authentication without the complexity of OAuth. Use this when the API provider gives you a static API key for authentication purposes.`,
			Code:            exampleAPIKey(),
			Category:        "Authentication",
			CategoryIcon:    "fa-lock",
			Icon:            "fa-id-card",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithHeader"),
		},

		// Resilience Patterns
		{
			Name:            "Retry with Default Policy",
			Description:     `Enable automatic retry with default policy (3 attempts, exponential backoff)`,
			FullDescription: `Shows how to enable automatic retry logic with a sensible default policy that retries failed requests up to 3 times with exponential backoff. This is essential for handling transient failures like temporary network issues, server overload (503), or rate limiting (429). The exponential backoff prevents overwhelming the failing service. Use this pattern for production services that need to be resilient to temporary failures without requiring fine-tuned retry configuration.`,
			Code:            exampleDefaultRetry(),
			Category:        "Resilience",
			CategoryIcon:    "fa-sync",
			Icon:            "fa-redo",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientDefaultRetryPolicy"),
		},
		{
			Name:            "Custom Retry Policy",
			Description:     `Configure custom retry policy with specific max attempts and backoff strategy`,
			FullDescription: `Demonstrates creating a custom retry policy where you control max attempts, backoff timing, and retry conditions. This gives you fine-grained control over retry behavior for specific use cases. For example, you might want more aggressive retries for critical operations or custom logic to determine which errors are retryable. Use this when default retry behavior doesn't match your requirements, such as integrating with APIs that have specific retry recommendations or when implementing custom retry strategies based on response headers.`,
			Code:            exampleCustomRetry(),
			Category:        "Resilience",
			CategoryIcon:    "fa-sync",
			Icon:            "fa-cog",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientRetryPolicy"),
		},
		{
			Name:            "Circuit Breaker",
			Description:     `Enable circuit breaker pattern to prevent cascading failures`,
			FullDescription: `Implements the circuit breaker pattern to protect your application from cascading failures when a downstream service is failing. When the failure threshold is reached, the circuit breaker "opens" and immediately fails requests without attempting them, giving the failing service time to recover. After a timeout period, it allows test requests through to check if the service has recovered. Critical for microservices architectures and distributed systems where one failing service shouldn't bring down the entire system. Use this to implement fail-fast behavior and prevent resource exhaustion.`,
			Code:            exampleCircuitBreaker(),
			Category:        "Resilience",
			CategoryIcon:    "fa-sync",
			Icon:            "fa-power-off",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientDefaultCircuitBreaker"),
		},
		{
			Name:            "Combined Resilience",
			Description:     `Combine retry policy and circuit breaker for maximum resilience`,
			FullDescription: `Shows how to combine multiple resilience patterns for comprehensive fault tolerance. By layering retry logic with circuit breaker protection, you get automatic recovery from transient failures while still protecting against sustained outages. The retry mechanism handles temporary glitches, while the circuit breaker prevents retry storms when a service is truly down. This is the recommended production configuration for critical services. Essential for building highly available systems that gracefully handle both temporary and prolonged service disruptions while maintaining system stability.`,
			Code:            exampleCombinedResilience(),
			Category:        "Resilience",
			CategoryIcon:    "fa-sync",
			Icon:            "fa-shield-alt",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientRetryPolicy", "WithClientCircuitBreaker", "WithClientLogger", "DefaultCircuitBreakerConfig", "DefaultRetryPolicy"),
		},

		// Middleware
		{
			Name:            "Custom Middleware",
			Description:     `Create and use custom middleware to intercept and modify requests/responses`,
			FullDescription: `Demonstrates how to create custom middleware to intercept and modify requests before they're sent or responses after they're received. Middleware is a powerful pattern for implementing cross-cutting concerns like logging, metrics, authentication, request/response transformation, and custom business logic. This example shows the basic structure of middleware and how to register it with the client. Use this pattern when you need to add functionality that applies to many or all requests, such as adding custom headers, measuring performance, or implementing custom retry logic.`,
			Code:            exampleCustomMiddleware(),
			Category:        "Middleware",
			CategoryIcon:    "fa-layer-group",
			Icon:            "fa-puzzle-piece",
			Options:         getOptions("WithClientMiddleware"),
		},
		{
			Name:            "Authentication Middleware",
			Description:     `Implement middleware that automatically adds authentication to all requests`,
			FullDescription: `Shows how to implement authentication middleware that automatically adds authentication tokens to all outgoing requests. This centralizes authentication logic and eliminates the need to manually add auth headers to each request. Particularly useful when working with token-based authentication where tokens may need to be refreshed or when implementing complex authentication flows. Use this pattern for service clients that always need authentication, such as internal microservice communication or when building SDK wrappers for authenticated APIs.`,
			Code:            exampleAuthMiddleware(),
			Category:        "Middleware",
			CategoryIcon:    "fa-layer-group",
			Icon:            "fa-lock",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientMiddleware"),
		},
		{
			Name:            "Logging Middleware",
			Description:     `Use built-in logging middleware for request/response logging`,
			FullDescription: `Demonstrates using the built-in structured logging capability for detailed request and response logging. Structured logging is essential for production systems where you need to debug issues, monitor API usage, and track performance. The logger captures method, URL, headers, timing, status codes, and errors in a structured format suitable for log aggregation systems. Use this for development debugging, production monitoring, or when you need to audit API calls for compliance. The logging level controls verbosity, allowing you to reduce noise in production while keeping detailed logs in development.`,
			Code:            exampleLoggingMiddleware(),
			Category:        "Middleware",
			CategoryIcon:    "fa-layer-group",
			Icon:            "fa-file-alt",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientLogger", "WithClientLogLevel"),
		},
		{
			Name:            "Middleware Chain",
			Description:     `Chain multiple middlewares together for complex request processing`,
			FullDescription: `Illustrates chaining multiple middleware together to create a comprehensive request processing pipeline. Middlewares execute in the order they're registered, allowing you to build layered functionality like authentication, then logging, then metrics collection. This pattern is fundamental for building production-grade HTTP clients where you need multiple cross-cutting concerns applied consistently. Use this when you need to combine authentication, logging, tracing, metrics, rate limiting, or any other middleware. The execution order matters - typically auth comes first, followed by logging, then metrics, then business logic.`,
			Code:            exampleMiddlewareChain(),
			Category:        "Middleware",
			CategoryIcon:    "fa-layer-group",
			Icon:            "fa-link",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientMiddlewares"),
		},

		// Advanced Features
		{
			Name:            "HTTP Caching",
			Description:     `Enable HTTP caching that respects standard cache headers`,
			FullDescription: `Demonstrates enabling intelligent HTTP caching that automatically respects standard cache control headers (Cache-Control, ETag, Last-Modified, Expires). The cache stores responses and serves them directly when valid, reducing network traffic and improving response times. The cache automatically handles conditional requests (If-None-Match, If-Modified-Since) and cache invalidation based on server directives. Use this to improve performance for frequently accessed resources, reduce API rate limit consumption, and decrease server load. Ideal for read-heavy applications, public API data, or any scenario where data doesn't change frequently.`,
			Code:            exampleCaching(),
			Category:        "Advanced",
			CategoryIcon:    "fa-cogs",
			Icon:            "fa-database",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientDefaultCache"),
		},
		{
			Name:            "Response Streaming",
			Description:     `Handle large responses efficiently using streaming mode`,
			FullDescription: `Shows how to handle large file downloads or streaming responses efficiently without loading the entire response into memory. Streaming mode gives you direct access to the response body reader, allowing you to process data in chunks. This is critical for downloading large files, processing real-time streams, or handling responses that don't fit in memory. Use this when downloading files, processing large API responses, consuming server-sent events, or any scenario where memory efficiency is important. Essential for applications that need to handle large payloads without excessive memory consumption.`,
			Code:            exampleStreaming(),
			Category:        "Advanced",
			CategoryIcon:    "fa-cogs",
			Icon:            "fa-stream",
			Options:         getOptions("WithDefaultBaseURL", "WithPath", "WithStreaming"),
		},
		{
			Name:            "Proxy Configuration",
			Description:     `Configure HTTP, HTTPS, or SOCKS5 proxy for requests`,
			FullDescription: `Demonstrates configuring HTTP, HTTPS, and SOCKS5 proxies for routing requests through proxy servers. Proxy support is essential for corporate environments with network restrictions, testing with tools like Charles or Fiddler, accessing geo-restricted content, or implementing request routing. The library supports authenticated proxies and allows per-request proxy configuration. Use this in corporate environments where all traffic must go through a proxy, when testing with debugging proxies, or when you need to route traffic through specific network paths for compliance or security requirements.`,
			Code:            exampleProxy(),
			Category:        "Advanced",
			CategoryIcon:    "fa-cogs",
			Icon:            "fa-network-wired",
			Options:         getOptions("GET", "WithBaseURL", "WithPath", "WithHTTPProxy", "WithHTTPSProxy", "WithSOCKS5Proxy", "WithProxyAuth"),
		},
		{
			Name:            "Cookie Management",
			Description:     `Manage cookies across requests using cookie jar`,
			FullDescription: `Illustrates automatic cookie management using a cookie jar that persists cookies across multiple requests, mimicking browser behavior. The cookie jar automatically handles Set-Cookie headers from responses and includes appropriate cookies in subsequent requests based on domain and path rules. This is essential for maintaining sessions with APIs that use cookie-based authentication or state management. Use this when working with session-based APIs, implementing login flows that use cookies, or when you need to maintain state across multiple API calls like a browser would.`,
			Code:            exampleCookies(),
			Category:        "Advanced",
			CategoryIcon:    "fa-cogs",
			Icon:            "fa-cookie",
			Options:         getOptions("WithClientDefaultBaseURL", "WithCookieJar", "WithPath", "WithJSONBody"),
		},
		{
			Name:            "OpenTelemetry Tracing",
			Description:     `Enable distributed tracing with OpenTelemetry for observability`,
			FullDescription: `Shows how to enable OpenTelemetry distributed tracing for complete observability across your service architecture. Traces automatically capture HTTP request details, timing information, and propagate trace context across service boundaries. This is critical for debugging issues in microservices architectures, understanding request flows, measuring latency, and identifying bottlenecks. Use this in production systems where you need end-to-end visibility of requests across multiple services. Essential for microservices observability, performance optimization, and debugging distributed systems. Integrates seamlessly with tools like Jaeger, Zipkin, and cloud tracing platforms.`,
			Code:            exampleTracing(),
			Category:        "Advanced",
			CategoryIcon:    "fa-cogs",
			Icon:            "fa-chart-line",
			Options:         getOptions("WithClientDefaultBaseURL", "WithTracing"),
		},
		{
			Name:            "Prometheus Metrics",
			Description:     `Expose Prometheus metrics for monitoring HTTP client performance`,
			FullDescription: `Demonstrates exposing Prometheus metrics for monitoring HTTP client behavior and performance. The client automatically tracks request counts, duration histograms, request/response sizes, and error rates, all tagged with useful labels like method, status code, and path. These metrics are essential for alerting, capacity planning, SLA monitoring, and performance analysis. Use this in production systems where you need to monitor API health, track error rates, measure latency percentiles, or set up alerts based on HTTP client behavior. The metrics integrate with standard Prometheus monitoring stacks and Grafana dashboards.`,
			Code:            examplePrometheus(),
			Category:        "Advanced",
			CategoryIcon:    "fa-cogs",
			Icon:            "fa-chart-bar",
			Options:         getOptions("WithClientDefaultBaseURL"),
		},

		// Testing
		{
			Name:            "Mock Server",
			Description:     `Create mock HTTP servers for testing with fluent API`,
			FullDescription: `Demonstrates creating mock HTTP servers for testing your HTTP client code without making real network calls. The mock server provides a fluent API for defining expected requests and their responses, making it easy to test different scenarios. This is essential for unit testing, integration testing, and development environments where you need deterministic, fast, and isolated tests. Use this to test your application's HTTP integration logic, verify request formatting, test error handling, and develop against APIs that aren't available yet. Eliminates dependencies on external services during testing.`,
			Code:            exampleMockServer(),
			Category:        "Testing",
			CategoryIcon:    "fa-vial",
			Icon:            "fa-server",
			Options:         getOptions("WithDefaultBaseURL"),
		},
		{
			Name:            "Request Matchers",
			Description:     `Use request matchers to verify request details in tests`,
			FullDescription: `Shows how to use sophisticated request matchers to verify that your code is making the correct HTTP requests. Matchers allow you to assert on HTTP method, path, headers, query parameters, and request body content. This gives you confidence that your application is correctly formatting and sending requests according to API specifications. Use this in unit tests to verify integration code, ensure headers are set correctly, validate request body formatting, or test that authentication tokens are being sent. Essential for testing HTTP client wrappers and SDK implementations.`,
			Code:            exampleRequestMatchers(),
			Category:        "Testing",
			CategoryIcon:    "fa-vial",
			Icon:            "fa-search",
			Options:         getOptions(),
		},
		{
			Name:            "Test Assertions",
			Description:     `Use built-in assertions to verify request behavior`,
			FullDescription: `Demonstrates using built-in test assertions to verify that expected HTTP calls were made during test execution. Assertions allow you to check that specific endpoints were called, verify call counts, and ensure that certain requests were or weren't made. This is crucial for testing side effects, verifying integration points, and ensuring your application behaves correctly. Use this in tests to confirm API calls happen when expected, verify retry behavior, test that requests aren't duplicated, or ensure failed operations don't make unnecessary API calls.`,
			Code:            exampleAssertions(),
			Category:        "Testing",
			CategoryIcon:    "fa-vial",
			Icon:            "fa-check-circle",
			Options:         getOptions("WithDefaultBaseURL", "WithPath"),
		},
		{
			Name:            "Error Simulation",
			Description:     `Simulate network errors and timeouts for testing error handling`,
			FullDescription: `Illustrates simulating various failure scenarios including network errors, timeouts, server errors, and flaky behavior for comprehensive error handling testing. Testing error paths is critical but difficult with real APIs. The mock server makes it easy to trigger specific error conditions reliably. Use this to test retry logic, verify error handling code paths, ensure graceful degradation, test circuit breaker behavior, and validate that your application handles failures appropriately. Essential for building resilient applications that properly handle the unreliable nature of network communication.`,
			Code:            exampleErrorSimulation(),
			Category:        "Testing",
			CategoryIcon:    "fa-vial",
			Icon:            "fa-exclamation-triangle",
			Options:         getOptions(),
		},

		// Production Patterns
		{
			Name:            "Production Client",
			Description:     `Complete production-ready client configuration with all best practices`,
			FullDescription: `Shows a comprehensive production-ready HTTP client configuration incorporating all best practices: structured logging, circuit breaker, retry policy, caching, distributed tracing, and proper timeouts. This configuration provides resilience against failures, observability for debugging, performance optimization through caching, and proper resource management. Use this as a template for production services where reliability and observability are critical. This configuration ensures your service can handle failures gracefully, provides visibility into API behavior, and implements defense mechanisms against cascading failures. Essential for any production service making external HTTP calls.`,
			Code:            exampleProductionClient(),
			Category:        "Production Patterns",
			CategoryIcon:    "fa-industry",
			Icon:            "fa-star",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientLogger", "WithClientLogLevel", "WithClientDefaultHeaders", "WithClientCircuitBreaker", "WithClientRetryPolicy", "WithClientDefaultCache", "WithTracing", "ConservativeCircuitBreakerConfig", "ConservativeRetryPolicy"),
		},
		{
			Name:            "Microservice Client",
			Description:     `Configure client for microservice-to-microservice communication`,
			FullDescription: `Demonstrates configuring an HTTP client specifically for microservice-to-microservice communication within a distributed system. This includes service identification headers, correlation IDs for request tracing, appropriate timeouts for internal networks, retry policies tuned for internal services, and distributed tracing integration. Use this pattern for service mesh environments, internal API calls, or any scenario where services need to communicate within your infrastructure. The configuration balances fast failure detection with resilience, includes proper context propagation for debugging, and ensures requests can be traced across service boundaries.`,
			Code:            exampleMicroserviceClient(),
			Category:        "Production Patterns",
			CategoryIcon:    "fa-industry",
			Icon:            "fa-network-wired",
			Options:         getOptions("WithClientDefaultBaseURL", "WithClientLogger", "WithClientDefaultHeader", "WithClientDefaultRetryPolicy", "WithClientDefaultCircuitBreaker", "WithTracing"),
		},
		{
			Name:            "Error Handling",
			Description:     `Comprehensive error handling and classification for production use`,
			FullDescription: `Illustrates comprehensive error handling patterns including error classification, type checking, and appropriate recovery strategies. The library provides structured error types that distinguish between network errors, timeouts, client errors (4xx), and server errors (5xx), allowing you to implement different handling strategies for each. Use this pattern in production code to properly handle and recover from different failure modes, implement appropriate retry logic, provide meaningful error messages to users, and log errors with sufficient context for debugging. Essential for building robust applications that fail gracefully and provide good user experience even when APIs fail.`,
			Code:            exampleErrorHandling(),
			Category:        "Production Patterns",
			CategoryIcon:    "fa-industry",
			Icon:            "fa-exclamation-circle",
			Options:         getOptions("WithDefaultBaseURL", "WithPath"),
		},
	}
}

// Example code snippets
func exampleSimpleGET() string {
	return strings.TrimSpace(`
// Make a simple GET request with automatic JSON unmarshaling
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/users/1"),
)

if err != nil {
	log.Fatal(err)
}

fmt.Printf("Status: %d\n", response.StatusCode)
fmt.Printf("Body: %+v\n", response.Body)
`)
}

func exampleSimplePOST() string {
	return strings.TrimSpace(`
type User struct {
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

newUser := User{
	Name:  "Alice Johnson",
	Email: "alice@example.com",
}

response, err := httpx.POST[User](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/users"),
	httpx.WithJSONBody(newUser),
)

if err != nil {
	log.Fatal(err)
}

fmt.Printf("Created: %+v\n", response.Body)
`)
}

func exampleConfiguredClient() string {
	return strings.TrimSpace(`
// Create a client with default configuration
client := httpx.NewClient(
	httpx.WithDefaultBaseURL("https://api.example.com"),
	httpx.WithDefaultTimeout(10 * time.Second),
	httpx.WithDefaultHeader("User-Agent", "MyApp/1.0"),
)

// All requests will use these defaults
req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/users/1"),
)

response, err := client.Execute(*req, map[string]any{})
`)
}

func exampleQueryParams() string {
	return strings.TrimSpace(`
response, err := httpx.GET[[]map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/users"),
	httpx.WithQueryParam("page", "1"),
	httpx.WithQueryParam("limit", "10"),
	httpx.WithQueryParam("sort", "name"),
)
`)
}

func exampleCustomHeaders() string {
	return strings.TrimSpace(`
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/api/data"),
	httpx.WithHeader("X-API-Version", "v2"),
	httpx.WithHeader("X-Client-ID", "my-client"),
	httpx.WithHeader("Accept-Language", "en-US"),
)
`)
}

func exampleJSONBody() string {
	return strings.TrimSpace(`
data := map[string]any{
	"title": "New Post",
	"body":  "This is the content",
	"tags":  []string{"go", "http", "client"},
}

response, err := httpx.POST[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/posts"),
	httpx.WithJSONBody(data),
)
`)
}

func exampleContextTimeout() string {
	return strings.TrimSpace(`
// Create context with 5 second timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/slow-endpoint"),
	httpx.WithContext(ctx),
)

if err != nil {
	// Check if timeout
	if ctx.Err() == context.DeadlineExceeded {
		log.Println("Request timed out")
	}
}
`)
}

func exampleBasicAuth() string {
	return strings.TrimSpace(`
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/secure"),
	httpx.WithBasicAuth("username", "password"),
)
`)
}

func exampleBearerToken() string {
	return strings.TrimSpace(`
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/api/protected"),
	httpx.WithBearerToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."),
)
`)
}

func exampleAPIKey() string {
	return strings.TrimSpace(`
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/api/data"),
	httpx.WithHeader("X-API-Key", "your-api-key-here"),
)
`)
}

func exampleDefaultRetry() string {
	return strings.TrimSpace(`
client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientDefaultRetryPolicy(), // 3 retries, exponential backoff
)

req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/api/data"),
)

// Will automatically retry on failure
response, err := client.Execute(*req, map[string]any{})
`)
}

func exampleCustomRetry() string {
	return strings.TrimSpace(`
retryPolicy := httpx.RetryPolicy{
	MaxAttempts: 5,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay: 10 * time.Second,
	Multiplier: 2.0,
	ShouldRetry: func(resp *http.Response, err error) bool {
		// Custom retry logic
		return err != nil || resp.StatusCode >= 500
	},
}

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientRetryPolicy(retryPolicy),
)
`)
}

func exampleCircuitBreaker() string {
	return strings.TrimSpace(`
client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientDefaultCircuitBreaker(),
)

// Circuit breaker will open if service is failing
// and prevent further requests to give it time to recover
req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/api/data"),
)

response, err := client.Execute(*req, map[string]any{})
`)
}

func exampleCombinedResilience() string {
	return strings.TrimSpace(`
logger := slog.Default()

cbConfig := httpx.DefaultCircuitBreakerConfig()
cbConfig.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
	logger.Info("Circuit breaker state change",
		"from", from, "to", to)
}

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientRetryPolicy(httpx.DefaultRetryPolicy()),
	httpx.WithClientCircuitBreaker(cbConfig),
	httpx.WithClientLogger(logger),
)
`)
}

func exampleCustomMiddleware() string {
	return strings.TrimSpace(`
type LoggingMiddleware struct{}

func (m *LoggingMiddleware) Name() string {
	return "logging"
}

func (m *LoggingMiddleware) Execute(
	ctx context.Context,
	req *http.Request,
	next httpx.MiddlewareFunc,
) (*http.Response, error) {
	start := time.Now()
	log.Printf("Request: %s %s", req.Method, req.URL)

	resp, err := next(ctx, req)

	log.Printf("Response: %d in %v", resp.StatusCode, time.Since(start))
	return resp, err
}

client := httpx.NewClientWithConfig(
	httpx.WithClientMiddleware(&LoggingMiddleware{}),
)
`)
}

func exampleAuthMiddleware() string {
	return strings.TrimSpace(`
type AuthMiddleware struct {
	token string
}

func (m *AuthMiddleware) Name() string {
	return "auth"
}

func (m *AuthMiddleware) Execute(
	ctx context.Context,
	req *http.Request,
	next httpx.MiddlewareFunc,
) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+m.token)
	return next(ctx, req)
}

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientMiddleware(&AuthMiddleware{token: "secret-token"}),
)
`)
}

func exampleLoggingMiddleware() string {
	return strings.TrimSpace(`
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientLogger(logger),
	httpx.WithClientLogLevel(slog.LevelDebug),
)

// All requests will be logged with details
req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/api/users"),
)

response, err := client.Execute(*req, []map[string]any{})
`)
}

func exampleMiddlewareChain() string {
	return strings.TrimSpace(`
authMiddleware := &AuthMiddleware{token: "secret"}
loggingMiddleware := &LoggingMiddleware{}
metricsMiddleware := &MetricsMiddleware{}

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientMiddlewares(
		authMiddleware,
		loggingMiddleware,
		metricsMiddleware,
	),
)

// Middlewares execute in order: auth -> logging -> metrics -> request
`)
}

func exampleCaching() string {
	return strings.TrimSpace(`
client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientDefaultCache(), // Respects Cache-Control headers
)

// First request fetches from server
req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/api/users/1"),
)
response1, _ := client.Execute(*req, map[string]any{})

// Second request may be served from cache if server allows it
response2, _ := client.Execute(*req, map[string]any{})
`)
}

func exampleStreaming() string {
	return strings.TrimSpace(`
client := httpx.NewClient(
	httpx.WithDefaultBaseURL("https://api.example.com"),
)

req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/large-file"),
	httpx.WithStreaming(), // Enable streaming mode
)

response, err := client.Execute(*req, []byte{})
if err != nil {
	log.Fatal(err)
}
defer response.StreamBody.Close()

// Read response in chunks
buffer := make([]byte, 4096)
for {
	n, err := response.StreamBody.Read(buffer)
	if err == io.EOF {
		break
	}
	// Process chunk
	processChunk(buffer[:n])
}
`)
}

func exampleProxy() string {
	return strings.TrimSpace(`
// HTTP Proxy
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/api/data"),
	httpx.WithHTTPProxy("http://proxy.example.com:8080"),
	httpx.WithProxyAuth("proxy-user", "proxy-pass"),
)

// SOCKS5 Proxy
response, err := httpx.GET[map[string]any](
	httpx.WithBaseURL("https://api.example.com"),
	httpx.WithPath("/api/data"),
	httpx.WithSOCKS5Proxy("socks5://proxy.example.com:1080"),
)
`)
}

func exampleCookies() string {
	return strings.TrimSpace(`
// Create cookie jar for automatic cookie management
jar, _ := cookiejar.New(nil)

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientCookieJar(jar),
)

// First request sets cookies
req1 := httpx.NewRequest(http.MethodPost,
	httpx.WithPath("/login"),
	httpx.WithJSONBody(map[string]string{
		"username": "user",
		"password": "pass",
	}),
)
client.Execute(*req1, map[string]any{})

// Second request automatically includes cookies
req2 := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/profile"),
)
response, _ := client.Execute(*req2, map[string]any{})
`)
}

func exampleTracing() string {
	return strings.TrimSpace(`
import "go.opentelemetry.io/otel"

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithTracing("my-service"), // Enable OpenTelemetry tracing
)

// Requests will automatically create spans
req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/api/users"),
)

response, err := client.Execute(*req, []map[string]any{})
// Trace data will be exported to configured backend
`)
}

func examplePrometheus() string {
	return strings.TrimSpace(`
import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Create client with Prometheus metrics enabled
client := httpx.NewClientWithPrometheus(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	"my_service", // Metrics namespace
)

// Expose metrics endpoint
http.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":2112", nil)

// Make requests - metrics will be automatically recorded
// Metrics include:
// - http_client_requests_total (counter)
// - http_client_request_duration_seconds (histogram)
// - http_client_request_size_bytes (histogram)
// - http_client_response_size_bytes (histogram)
`)
}

func exampleMockServer() string {
	return strings.TrimSpace(`
import httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"

// Create mock server
mock := httpxtesting.NewMockServer()
defer mock.Close()

// Configure responses
mock.OnGet("/users").
	WithStatus(http.StatusOK).
	WithJSON(map[string]interface{}{
		"users": []map[string]interface{}{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
	})

mock.OnPost("/users").
	WithStatus(http.StatusCreated).
	WithJSON(map[string]interface{}{
		"id":   3,
		"name": "Charlie",
	})

// Use in tests
client := httpx.NewClient(httpx.WithDefaultBaseURL(mock.URL()))
`)
}

func exampleRequestMatchers() string {
	return strings.TrimSpace(`
import httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"

mock := httpxtesting.NewMockServer()
defer mock.Close()

// Match requests with specific criteria
mock.OnRequest().
	WithMethod("POST").
	WithPath("/users").
	WithHeader("Content-Type", "application/json").
	WithJSONBody(map[string]interface{}{
		"name": "Alice",
	}).
	Respond().
	WithStatus(http.StatusCreated).
	WithJSON(map[string]interface{}{
		"id": 123,
		"name": "Alice",
	})
`)
}

func exampleAssertions() string {
	return strings.TrimSpace(`
import httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"

mock := httpxtesting.NewMockServer()
defer mock.Close()

mock.OnGet("/users").
	WithStatus(http.StatusOK).
	WithJSON([]map[string]interface{}{
		{"id": 1, "name": "Alice"},
	})

// Make request
client := httpx.NewClient(httpx.WithDefaultBaseURL(mock.URL()))
req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users"))
client.Execute(*req, []map[string]any{})

// Assert request was made
mock.AssertCalled(t, "GET", "/users")
mock.AssertNotCalled(t, "POST", "/users")
mock.AssertCallCount(t, "GET", "/users", 1)
`)
}

func exampleErrorSimulation() string {
	return strings.TrimSpace(`
import httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"

mock := httpxtesting.NewMockServer()
defer mock.Close()

// Simulate network error
mock.OnGet("/error").
	WithNetworkError(errors.New("connection refused"))

// Simulate timeout
mock.OnGet("/slow").
	WithDelay(5 * time.Second)

// Simulate server error
mock.OnGet("/server-error").
	WithStatus(http.StatusInternalServerError).
	WithJSON(map[string]interface{}{
		"error": "internal server error",
	})

// Simulate flaky behavior (fails 2 times, then succeeds)
mock.OnGet("/flaky").
	WithFlakyBehavior(2).
	WithStatus(http.StatusOK).
	WithJSON(map[string]interface{}{
		"message": "success",
	})
`)
}

func exampleProductionClient() string {
	return strings.TrimSpace(`
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

cbConfig := httpx.ConservativeCircuitBreakerConfig()
cbConfig.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
	logger.Error("Circuit breaker state change",
		"service", name,
		"from", from,
		"to", to,
	)
}

client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("https://api.example.com"),
	httpx.WithClientTimeout(10*time.Second),
	httpx.WithClientLogger(logger),
	httpx.WithClientLogLevel(slog.LevelInfo),
	httpx.WithClientDefaultHeaders(http.Header{
		"User-Agent": []string{"ProductionApp/1.0.0"},
		"Accept":     []string{"application/json"},
	}),
	httpx.WithClientCircuitBreaker(cbConfig),
	httpx.WithClientRetryPolicy(httpx.ConservativeRetryPolicy()),
	httpx.WithClientDefaultCache(),
	httpx.WithTracing("my-service"),
)
`)
}

func exampleMicroserviceClient() string {
	return strings.TrimSpace(`
// Service-to-service client with tracing and metrics
client := httpx.NewClientWithConfig(
	httpx.WithClientDefaultBaseURL("http://user-service:8080"),
	httpx.WithClientTimeout(5*time.Second),
	httpx.WithClientLogger(slog.Default()),
	httpx.WithClientDefaultHeader("X-Service-Name", "order-service"),
	httpx.WithClientDefaultHeader("X-Service-Version", "v1.2.3"),
	httpx.WithClientDefaultRetryPolicy(),
	httpx.WithClientDefaultCircuitBreaker(),
	httpx.WithTracing("order-service"),
)

// Add correlation ID middleware
type CorrelationIDMiddleware struct{}

func (m *CorrelationIDMiddleware) Name() string {
	return "correlation-id"
}

func (m *CorrelationIDMiddleware) Execute(
	ctx context.Context,
	req *http.Request,
	next httpx.MiddlewareFunc,
) (*http.Response, error) {
	req.Header.Set("X-Correlation-ID", generateCorrelationID())
	return next(ctx, req)
}
`)
}

func exampleErrorHandling() string {
	return strings.TrimSpace(`
client := httpx.NewClient(
	httpx.WithDefaultBaseURL("https://api.example.com"),
)

req := httpx.NewRequest(http.MethodGet,
	httpx.WithPath("/api/data"),
)

response, err := client.Execute(*req, map[string]any{})
if err != nil {
	// Check error type
	httpErr := &httpx.HTTPError{}
	if errors.As(err, &httpErr) {
		switch httpErr.Type {
		case httpx.ErrorTypeNetwork:
			log.Printf("Network error: %s", httpErr.Message)
		case httpx.ErrorTypeTimeout:
			log.Printf("Request timeout: %s", httpErr.Message)
		case httpx.ErrorTypeClient:
			log.Printf("Client error %d: %s", httpErr.StatusCode, httpErr.Message)
		case httpx.ErrorTypeServer:
			log.Printf("Server error %d: %s", httpErr.StatusCode, httpErr.Message)
		default:
			log.Printf("Unknown error: %s", httpErr.Message)
		}
		return
	}
}

// Check response status
if response.StatusCode >= 400 {
	log.Printf("HTTP error: %d", response.StatusCode)
}
`)
}
