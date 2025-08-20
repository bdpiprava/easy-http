package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("=== Custom Middleware Example ===")

	// Example 1: Authentication middleware
	fmt.Println("\n--- Example 1: Authentication Middleware ---")
	authenticationMiddlewareExample()

	// Example 2: Request ID and correlation middleware
	fmt.Println("\n--- Example 2: Request ID Middleware ---")
	requestIDMiddlewareExample()

	// Example 3: Rate limiting middleware
	fmt.Println("\n--- Example 3: Rate Limiting Middleware ---")
	rateLimitingMiddlewareExample()

	// Example 4: Response transformation middleware
	fmt.Println("\n--- Example 4: Response Transformation Middleware ---")
	responseTransformationExample()

	// Example 5: Multiple middlewares in chain
	fmt.Println("\n--- Example 5: Multiple Middlewares Chain ---")
	middlewareChainExample()

	// Example 6: Built-in middlewares
	fmt.Println("\n--- Example 6: Built-in Middlewares ---")
	builtInMiddlewaresExample()

	fmt.Println("\n=== Middleware Examples Complete ===")
}

// AuthMiddleware adds authentication headers to requests
type AuthMiddleware struct {
	token string
}

func NewAuthMiddleware(token string) *AuthMiddleware {
	return &AuthMiddleware{token: token}
}

func (m *AuthMiddleware) Name() string {
	return "auth"
}

func (m *AuthMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	// Add Authorization header to all requests
	req.Header.Set("Authorization", "Bearer "+m.token)

	fmt.Printf("  🔑 Auth middleware: Added Bearer token to request\n")

	return next(ctx, req)
}

// RequestIDMiddleware adds a unique request ID to each request
type RequestIDMiddleware struct{}

func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{}
}

func (m *RequestIDMiddleware) Name() string {
	return "request-id"
}

func (m *RequestIDMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	// Generate a simple request ID (in real apps, use UUID)
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	req.Header.Set("X-Request-ID", requestID)

	fmt.Printf("  🏷️  Request ID middleware: Added ID %s\n", requestID)

	resp, err := next(ctx, req)

	if resp != nil {
		fmt.Printf("  🏷️  Request ID middleware: Request %s completed with status %d\n",
			requestID, resp.StatusCode)
	}

	return resp, err
}

// RateLimitMiddleware simulates rate limiting
type RateLimitMiddleware struct {
	requestCounts map[string]int
	limit         int
	windowStart   time.Time
}

func NewRateLimitMiddleware(limit int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		requestCounts: make(map[string]int),
		limit:         limit,
		windowStart:   time.Now(),
	}
}

func (m *RateLimitMiddleware) Name() string {
	return "rate-limit"
}

func (m *RateLimitMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	// Simple rate limiting by client (in real apps, use proper rate limiting algorithms)
	clientIP := req.Header.Get("X-Client-IP")
	if clientIP == "" {
		clientIP = "default"
	}

	// Reset counts every minute
	if time.Since(m.windowStart) > time.Minute {
		m.requestCounts = make(map[string]int)
		m.windowStart = time.Now()
	}

	m.requestCounts[clientIP]++

	if m.requestCounts[clientIP] > m.limit {
		fmt.Printf("  🚫 Rate limit middleware: Client %s exceeded limit (%d/%d)\n",
			clientIP, m.requestCounts[clientIP], m.limit)

		// Return rate limit error instead of making the request
		return nil, &httpx.HTTPError{
			Type:    httpx.ErrorTypeClient,
			Message: "Rate limit exceeded",
		}
	}

	fmt.Printf("  📊 Rate limit middleware: Client %s (%d/%d requests)\n",
		clientIP, m.requestCounts[clientIP], m.limit)

	return next(ctx, req)
}

// ResponseTransformMiddleware modifies responses
type ResponseTransformMiddleware struct{}

func NewResponseTransformMiddleware() *ResponseTransformMiddleware {
	return &ResponseTransformMiddleware{}
}

func (m *ResponseTransformMiddleware) Name() string {
	return "response-transform"
}

func (m *ResponseTransformMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	resp, err := next(ctx, req)
	if err != nil {
		return resp, err
	}

	// Add custom headers to response
	if resp != nil {
		resp.Header.Set("X-Processed-By", "CustomMiddleware")
		resp.Header.Set("X-Processing-Time", time.Now().Format(time.RFC3339))
		fmt.Printf("  🔄 Response transform middleware: Added custom headers\n")
	}

	return resp, err
}

// authenticationMiddlewareExample demonstrates authentication middleware
func authenticationMiddlewareExample() {
	server := createAuthServer()
	defer server.Close()

	// Create client with authentication middleware
	authMiddleware := NewAuthMiddleware("secret-api-token")

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddleware(authMiddleware),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/protected"))
	response, err := client.Execute(*req, map[string]any{})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success! Status: %d, Body: %+v\n", response.StatusCode, response.Body)
	}
}

// requestIDMiddlewareExample demonstrates request ID tracking
func requestIDMiddlewareExample() {
	server := createEchoServer()
	defer server.Close()

	requestIDMiddleware := NewRequestIDMiddleware()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddleware(requestIDMiddleware),
	)

	// Make multiple requests to see different IDs
	for i := 1; i <= 3; i++ {
		fmt.Printf("\n  Making request #%d:\n", i)
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/echo"))
		response, err := client.Execute(*req, map[string]any{})

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  Response: %+v\n", response.Body)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// rateLimitingMiddlewareExample demonstrates rate limiting
func rateLimitingMiddlewareExample() {
	server := createEchoServer()
	defer server.Close()

	rateLimitMiddleware := NewRateLimitMiddleware(3) // Limit to 3 requests

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddleware(rateLimitMiddleware),
		httpx.WithClientDefaultHeader("X-Client-IP", "192.168.1.100"),
	)

	// Make 5 requests to trigger rate limiting
	for i := 1; i <= 5; i++ {
		fmt.Printf("\n  Making request #%d:\n", i)
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/echo"))
		response, err := client.Execute(*req, map[string]any{})

		if err != nil {
			fmt.Printf("  ❌ Error: %v\n", err)
		} else {
			fmt.Printf("  ✅ Success: Status %d\n", response.StatusCode)
		}
	}
}

// responseTransformationExample demonstrates response transformation
func responseTransformationExample() {
	server := createEchoServer()
	defer server.Close()

	transformMiddleware := NewResponseTransformMiddleware()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddleware(transformMiddleware),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/echo"))
	response, err := client.Execute(*req, map[string]any{})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success! Status: %d\n", response.StatusCode)
		fmt.Printf("Custom headers added:\n")
		fmt.Printf("  X-Processed-By: %s\n", response.Header().Get("X-Processed-By"))
		fmt.Printf("  X-Processing-Time: %s\n", response.Header().Get("X-Processing-Time"))
	}
}

// middlewareChainExample demonstrates multiple middlewares working together
func middlewareChainExample() {
	server := createAuthServer()
	defer server.Close()

	// Create multiple middlewares
	authMiddleware := NewAuthMiddleware("api-key-12345")
	requestIDMiddleware := NewRequestIDMiddleware()
	rateLimitMiddleware := NewRateLimitMiddleware(10)
	transformMiddleware := NewResponseTransformMiddleware()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddlewares(
			authMiddleware,
			requestIDMiddleware,
			rateLimitMiddleware,
			transformMiddleware,
		),
		httpx.WithClientDefaultHeader("X-Client-IP", "192.168.1.200"),
	)

	fmt.Println("  Making request with full middleware chain:")
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/protected"))
	response, err := client.Execute(*req, map[string]any{})

	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Success! Status: %d, Body: %+v\n", response.StatusCode, response.Body)
	}
}

// builtInMiddlewaresExample demonstrates built-in middlewares
func builtInMiddlewaresExample() {
	server := createEchoServer()
	defer server.Close()

	// Create client with built-in middlewares
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientLogger(slog.Default()),
		httpx.WithClientLogLevel(slog.LevelDebug),
		// Built-in middlewares are automatically added based on configuration
	)

	fmt.Println("  Using built-in logging middleware:")
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/echo"))
	response, err := client.Execute(*req, map[string]any{})

	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Success! Status: %d\n", response.StatusCode)
	}
}

// createAuthServer creates a server that requires authentication
func createAuthServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "Missing or invalid authorization"}`))
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		fmt.Printf("    Server: Received token: %s\n", token)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Access granted", "token": "` + token + `"}`))
	}))
}

// createEchoServer creates a server that echoes request information
func createEchoServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		clientIP := r.Header.Get("X-Client-IP")

		response := map[string]string{
			"message": "Echo response",
			"method":  r.Method,
			"path":    r.URL.Path,
		}

		if requestID != "" {
			response["request_id"] = requestID
			fmt.Printf("    Server: Processing request %s\n", requestID)
		}

		if clientIP != "" {
			response["client_ip"] = clientIP
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple JSON encoding
		jsonResponse := `{"message": "` + response["message"] + `", "method": "` +
			response["method"] + `", "path": "` + response["path"] + `"`

		if requestID != "" {
			jsonResponse += `, "request_id": "` + requestID + `"`
		}
		if clientIP != "" {
			jsonResponse += `, "client_ip": "` + clientIP + `"`
		}

		jsonResponse += `}`
		_, _ = w.Write([]byte(jsonResponse))
	}))
}
