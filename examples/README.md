# Easy-HTTP Examples

This directory contains comprehensive examples demonstrating how to use the `easy-http` library effectively. Each example is self-contained and runnable, focusing on specific use cases and patterns.

## 📁 Example Structure

```
examples/
├── basic/           # Basic HTTP operations
├── retry/           # Retry mechanisms and policies
├── circuit_breaker/ # Circuit breaker patterns
├── middleware/      # Custom middleware creation
├── advanced/        # Advanced configuration patterns
├── testing/         # Testing and mocking patterns
└── README.md        # This file
```

## 🚀 Getting Started

To run any example:

```bash
cd examples/<example-name>
go run main.go
```

## 📖 Example Descriptions

### 1. Basic HTTP Operations (`basic/`)

**Use Case**: Simple HTTP operations for API integration and basic web service communication.

**What you'll learn**:
- Making GET, POST, PUT, DELETE requests
- Using query parameters and custom headers
- Working with JSON request/response bodies
- Basic authentication
- Type-safe response handling
- Creating and configuring HTTP clients

**Key Features Demonstrated**:
- `httpx.GET[T]()`, `httpx.POST[T]()` convenience functions
- `httpx.NewRequest()` and `httpx.WithPath()`, `httpx.WithQueryParam()`
- `httpx.NewClient()` with default configurations
- `httpx.WithJSONBody()` for sending JSON data
- `httpx.WithBasicAuth()` for authentication

**Perfect for**: Developers new to the library who want to understand basic HTTP operations.

---

### 2. Retry Mechanisms (`retry/`)

**Use Case**: Building resilient applications that can handle transient failures and network issues.

**What you'll learn**:
- Different retry strategies (fixed, linear, exponential, jitter)
- Configuring retry policies for different scenarios
- Custom retry conditions and logic
- Retry strategy comparison and selection
- Integration with structured logging

**Key Features Demonstrated**:
- `httpx.DefaultRetryPolicy()`, `httpx.AggressiveRetryPolicy()`, `httpx.ConservativeRetryPolicy()`
- `httpx.RetryPolicy` configuration options
- `httpx.WithClientRetryPolicy()` for client-level retry
- Custom `RetryCondition` functions
- `httpx.RetryStrategy` types and behaviors

**Perfect for**: Building microservices that need to handle external API failures gracefully.

---

### 3. Circuit Breaker (`circuit_breaker/`)

**Use Case**: Implementing fault tolerance patterns to prevent cascading failures in distributed systems.

**What you'll learn**:
- Circuit breaker states (closed, open, half-open)
- Configuring failure thresholds and timeouts
- State change monitoring and alerting
- Integration with retry mechanisms
- Different circuit breaker policies

**Key Features Demonstrated**:
- `httpx.DefaultCircuitBreakerConfig()`, `httpx.AggressiveCircuitBreakerConfig()`, `httpx.ConservativeCircuitBreakerConfig()`
- `httpx.WithClientCircuitBreaker()` configuration
- `CircuitBreakerConfig.OnStateChange` callbacks
- `httpx.IsCircuitBreakerError()` error detection
- Circuit breaker + retry policy combinations

**Perfect for**: Microservices architectures requiring robust fault tolerance mechanisms.

---

### 4. Custom Middleware (`middleware/`)

**Use Case**: Extending HTTP client functionality with cross-cutting concerns like authentication, logging, rate limiting, and request transformation.

**What you'll learn**:
- Creating custom middleware from scratch
- Middleware execution order and chaining
- Request and response transformation
- Authentication middleware patterns
- Rate limiting implementation
- Built-in middleware usage

**Key Features Demonstrated**:
- `httpx.Middleware` interface implementation
- `httpx.WithClientMiddleware()` and `httpx.WithClientMiddlewares()`
- `httpx.MiddlewareFunc` and middleware chaining
- Request header manipulation
- Response transformation patterns

**Perfect for**: Advanced users who need to implement custom request/response processing logic.

---

### 5. Advanced Configuration (`advanced/`)

**Use Case**: Production-ready HTTP clients with comprehensive configuration, context handling, streaming, and error management.

**What you'll learn**:
- Production-ready client configuration
- Context-aware requests with timeouts and cancellation
- Streaming response handling
- Comprehensive error handling and classification
- Microservice communication patterns
- Service discovery integration

**Key Features Demonstrated**:
- `httpx.NewClientWithConfig()` with all configuration options
- `httpx.WithContext()` for request cancellation
- `httpx.WithStreaming()` for large response handling
- `httpx.HTTPError` classification and handling
- Structured logging integration
- Production monitoring patterns

**Perfect for**: Production applications requiring enterprise-grade HTTP client functionality.

---

### 6. Testing and Mocking (`testing/`)

**Use Case**: Testing HTTP client code effectively with mocks, stubs, and integration testing patterns.

**What you'll learn**:
- Creating testable HTTP client code
- Using `httptest.Server` for mocking
- Testing different HTTP methods and status codes
- Error scenario testing
- Integration testing patterns
- State verification in tests

**Key Features Demonstrated**:
- `httptest.NewServer()` for creating mock servers
- Testing service layers that use `httpx`
- Error condition simulation and testing
- HTTP method and status code verification
- Stateful test server patterns

**Perfect for**: Developers who want to write comprehensive tests for their HTTP client code.

---

## 🏗️ Architecture Patterns

### Basic Pattern
```go
// Simple request
response, err := httpx.GET[MyType](
    httpx.WithBaseURL("https://api.example.com"),
    httpx.WithPath("/users/1"),
)
```

### Client Configuration Pattern
```go
// Configured client
client := httpx.NewClientWithConfig(
    httpx.WithClientDefaultBaseURL("https://api.example.com"),
    httpx.WithClientDefaultRetryPolicy(),
    httpx.WithClientDefaultCircuitBreaker(),
)

req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users/1"))
response, err := client.Execute(*req, User{})
```

### Service Layer Pattern
```go
type UserService struct {
    client *httpx.Client
}

func (s *UserService) GetUser(id int) (*User, error) {
    req := httpx.NewRequest(http.MethodGet,
        httpx.WithPath(fmt.Sprintf("/users/%d", id)),
    )
    
    response, err := s.client.Execute(*req, User{})
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    
    if user, ok := response.Body.(User); ok {
        return &user, nil
    }
    
    return nil, fmt.Errorf("unexpected response format")
}
```

## 🎯 Choosing the Right Example

| **If you want to...** | **Start with** | **Then explore** |
|------------------------|----------------|------------------|
| Learn basic HTTP operations | `basic/` | `testing/` |
| Build resilient services | `retry/` | `circuit_breaker/` |
| Handle service failures | `circuit_breaker/` | `advanced/` |
| Add custom functionality | `middleware/` | `advanced/` |
| Build production services | `advanced/` | All others for specific features |
| Test your HTTP code | `testing/` | `basic/` for simple test cases |

## 🔧 Configuration Quick Reference

### Client Configuration Options

```go
httpx.NewClientWithConfig(
    // Basic settings
    httpx.WithClientDefaultBaseURL("https://api.example.com"),
    httpx.WithClientTimeout(30*time.Second),
    
    // Headers and authentication
    httpx.WithClientDefaultHeader("User-Agent", "MyApp/1.0"),
    httpx.WithClientDefaultBasicAuth("username", "password"),
    
    // Resilience patterns
    httpx.WithClientDefaultRetryPolicy(),        // or custom policy
    httpx.WithClientDefaultCircuitBreaker(),     // or custom config
    
    // Observability
    httpx.WithClientLogger(slog.Default()),
    httpx.WithClientLogLevel(slog.LevelInfo),
    
    // Custom middleware
    httpx.WithClientMiddleware(myCustomMiddleware),
)
```

### Request Options

```go
httpx.NewRequest(http.MethodPost,
    // URL components
    httpx.WithBaseURL("https://api.example.com"),  // Override client default
    httpx.WithPath("/users", "123", "profile"),    // Builds: /users/123/profile
    
    // Parameters and headers
    httpx.WithQueryParam("include", "posts"),
    httpx.WithHeader("Content-Type", "application/json"),
    
    // Body and authentication
    httpx.WithJSONBody(userData),
    httpx.WithBasicAuth("user", "pass"),           // Override client default
    
    // Request behavior
    httpx.WithContext(ctx),                        // For timeouts/cancellation
    httpx.WithStreaming(),                         // For large responses
)
```

## 🚦 Best Practices

### 1. **Always Configure Timeouts**
```go
client := httpx.NewClientWithConfig(
    httpx.WithClientTimeout(30*time.Second),  // Prevent hanging requests
)
```

### 2. **Use Structured Logging**
```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
client := httpx.NewClientWithConfig(
    httpx.WithClientLogger(logger),
    httpx.WithClientLogLevel(slog.LevelInfo),
)
```

### 3. **Implement Retry for Resilience**
```go
client := httpx.NewClientWithConfig(
    httpx.WithClientDefaultRetryPolicy(),  // or custom policy
)
```

### 4. **Use Circuit Breakers for Fault Tolerance**
```go
client := httpx.NewClientWithConfig(
    httpx.WithClientDefaultCircuitBreaker(),  // or custom config
)
```

### 5. **Handle Errors Properly**
```go
response, err := client.Execute(*req, ResponseType{})
if err != nil {
    if httpErr, ok := err.(*httpx.HTTPError); ok {
        // Handle specific HTTP error types
        switch httpErr.Type {
        case httpx.ErrorTypeNetwork:
            // Handle network errors
        case httpx.ErrorTypeTimeout:
            // Handle timeout errors
        // ... other error types
        }
    }
    return fmt.Errorf("request failed: %w", err)
}
```

### 6. **Test with Mock Servers**
```go
func TestUserService(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock server logic
    }))
    defer server.Close()
    
    service := NewUserService(server.URL)
    // Test service methods
}
```

## 📚 Additional Resources

- **Library Documentation**: See the main README.md for API reference
- **Go HTTP Client Best Practices**: [Effective Go HTTP Clients](https://golang.org/doc/effective_go.html)
- **Testing Patterns**: [Go Testing Best Practices](https://golang.org/doc/tutorial/add-a-test)
- **Circuit Breaker Pattern**: [Martin Fowler's Circuit Breaker](https://martinfowler.com/bliki/CircuitBreaker.html)

## 🤝 Contributing

Found an issue with an example or have a suggestion for a new one? Please open an issue or submit a pull request!

---

*These examples are designed to be educational and demonstrate real-world usage patterns. Each example includes comprehensive comments explaining the concepts and can be run independently.*