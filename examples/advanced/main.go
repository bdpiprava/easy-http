package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("=== Advanced Configuration Example ===")

	// Example 1: Full-featured HTTP client with all features
	fmt.Println("\n--- Example 1: Full-Featured Client ---")
	fullFeaturedClientExample()

	// Example 2: Microservice client with service discovery pattern
	fmt.Println("\n--- Example 2: Microservice Client Pattern ---")
	microserviceClientExample()

	// Example 3: Context-aware requests with timeouts
	fmt.Println("\n--- Example 3: Context-Aware Requests ---")
	contextAwareRequestsExample()

	// Example 4: Streaming response handling
	fmt.Println("\n--- Example 4: Streaming Response Handling ---")
	streamingResponseExample()

	// Example 5: Error handling and classification
	fmt.Println("\n--- Example 5: Error Handling and Classification ---")
	errorHandlingExample()

	// Example 6: Production-ready client configuration
	fmt.Println("\n--- Example 6: Production-Ready Configuration ---")
	productionReadyClientExample()

	fmt.Println("\n=== Advanced Examples Complete ===")
}

// fullFeaturedClientExample demonstrates a client with all features enabled
func fullFeaturedClientExample() {
	server := createResilentTestServer()
	defer server.Close()

	// Create a structured logger for better observability
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Configure circuit breaker for fault tolerance
	circuitBreakerConfig := httpx.DefaultCircuitBreakerConfig()
	circuitBreakerConfig.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
		logger.Info("Circuit breaker state change",
			"name", name, "from", from, "to", to)
	}

	// Configure retry policy for resilience
	retryPolicy := httpx.AggressiveRetryPolicy()

	// Create client with all features
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientTimeout(30*time.Second),
		httpx.WithClientLogger(logger),
		httpx.WithClientLogLevel(slog.LevelDebug),
		httpx.WithClientDefaultHeaders(http.Header{
			"User-Agent":    []string{"AdvancedClient/1.0"},
			"Accept":        []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		}),
		httpx.WithClientCircuitBreaker(circuitBreakerConfig),
		httpx.WithClientRetryPolicy(retryPolicy),
	)

	// Make requests to demonstrate resilience
	for i := 1; i <= 5; i++ {
		fmt.Printf("\n  Request #%d:\n", i)

		req := httpx.NewRequest(http.MethodGet,
			httpx.WithPath("/api/data"),
			httpx.WithQueryParam("id", fmt.Sprintf("%d", i)),
			httpx.WithHeader("X-Request-Attempt", fmt.Sprintf("%d", i)),
		)

		response, err := client.Execute(*req, map[string]any{})
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
		} else {
			fmt.Printf("    Success: Status %d\n", response.StatusCode)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// microserviceClientExample demonstrates patterns for microservice communication
func microserviceClientExample() {
	// Simulate multiple service endpoints
	userService := createUserServiceServer()
	orderService := createOrderServiceServer()
	defer userService.Close()
	defer orderService.Close()

	// Service registry simulation
	services := map[string]string{
		"user-service":  userService.URL,
		"order-service": orderService.URL,
	}

	// Create service-specific clients
	userClient := createServiceClient("user-service", services["user-service"])
	orderClient := createServiceClient("order-service", services["order-service"])

	// Demonstrate service-to-service communication
	fmt.Println("  Fetching user information:")
	userReq := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users/123"))
	userResp, err := userClient.Execute(*userReq, map[string]any{})
	if err != nil {
		fmt.Printf("    User service error: %v\n", err)
		return
	}
	fmt.Printf("    User data: %+v\n", userResp.Body)

	fmt.Println("\n  Fetching user orders:")
	orderReq := httpx.NewRequest(http.MethodGet,
		httpx.WithPath("/orders"),
		httpx.WithQueryParam("userId", "123"),
	)
	orderResp, err := orderClient.Execute(*orderReq, []map[string]any{})
	if err != nil {
		fmt.Printf("    Order service error: %v\n", err)
		return
	}
	fmt.Printf("    Orders: %+v\n", orderResp.Body)
}

// contextAwareRequestsExample demonstrates context usage for timeouts and cancellation
func contextAwareRequestsExample() {
	server := createSlowServer()
	defer server.Close()

	//nolint:staticcheck
	client := httpx.NewClient(
		httpx.WithDefaultBaseURL(server.URL),
		httpx.WithDefaultTimeout(10*time.Second),
	)

	// Example 1: Request with timeout
	fmt.Println("  Request with 2-second timeout:")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httpx.NewRequest(http.MethodGet,
		httpx.WithPath("/slow"),
		httpx.WithContext(ctx),
	)

	start := time.Now()
	response, err := client.Execute(*req, map[string]any{})
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("    Request timed out after %v: %v\n", duration, err)
	} else {
		fmt.Printf("    Success after %v: %+v\n", duration, response.Body)
	}

	// Example 2: Request with cancellation
	fmt.Println("\n  Request with manual cancellation:")
	ctx2, cancel2 := context.WithCancel(context.Background())

	req2 := httpx.NewRequest(http.MethodGet,
		httpx.WithPath("/slow"),
		httpx.WithContext(ctx2),
	)

	// Cancel the request after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("    Cancelling request...")
		cancel2()
	}()

	start2 := time.Now()
	response2, err2 := client.Execute(*req2, map[string]any{})
	duration2 := time.Since(start2)

	if err2 != nil {
		fmt.Printf("    Request cancelled after %v: %v\n", duration2, err2)
	} else {
		fmt.Printf("    Unexpected success after %v: %+v\n", duration2, response2.Body)
	}
}

// streamingResponseExample demonstrates handling streaming responses
func streamingResponseExample() {
	server := createStreamingServer()
	defer server.Close()

	//nolint:staticcheck
	client := httpx.NewClient(httpx.WithDefaultBaseURL(server.URL))

	fmt.Println("  Making streaming request:")
	req := httpx.NewRequest(http.MethodGet,
		httpx.WithPath("/stream"),
		httpx.WithStreaming(), // Enable streaming mode
	)

	response, err := client.Execute(*req, []byte{})
	if err != nil {
		fmt.Printf("    Error: %v\n", err)
		return
	}

	fmt.Printf("    Status: %d\n", response.StatusCode)
	fmt.Printf("    Content-Type: %s\n", response.Header().Get("Content-Type"))

	// In streaming mode, response.StreamBody is available for reading
	// Note: In a real application, you would read from response.StreamBody
	// and close it when done. For this example, we'll just show the concept.
	if response.StreamBody != nil {
		fmt.Println("    Streaming body is available for reading")
		defer response.StreamBody.Close()

		// Read first few bytes as demonstration
		buffer := make([]byte, 100)
		n, readErr := response.StreamBody.Read(buffer)
		if readErr == nil {
			fmt.Printf("    First %d bytes: %s\n", n, string(buffer[:n]))
		}
	}
}

// errorHandlingExample demonstrates comprehensive error handling
func errorHandlingExample() {
	// Create servers that return different types of errors
	networkErrorServer := createNetworkErrorServer()
	httpErrorServer := createHTTPErrorServer()

	defer networkErrorServer.Close()
	defer httpErrorServer.Close()

	//nolint:staticcheck
	client := httpx.NewClient()

	fmt.Println("  Testing different error types:")

	// Test network error
	fmt.Println("\n    Network error test:")
	req1 := httpx.NewRequest(http.MethodGet,
		httpx.WithBaseURL("http://localhost:1"), // Non-existent server
		httpx.WithPath("/test"),
	)

	_, err1 := client.Execute(*req1, map[string]any{})
	if err1 != nil {
		httpErr := &httpx.HTTPError{}
		if errors.As(err1, &httpErr) {
			fmt.Printf("      Error Type: %s\n", httpErr.Type)
			fmt.Printf("      Message: %s\n", httpErr.Message)
		} else {
			fmt.Printf("      Unknown error: %v\n", err1)
		}
	}

	// Test HTTP error
	fmt.Println("\n    HTTP error test:")
	req2 := httpx.NewRequest(http.MethodGet,
		httpx.WithBaseURL(httpErrorServer.URL),
		httpx.WithPath("/error"),
	)

	_, err2 := client.Execute(*req2, map[string]any{})
	if err2 != nil {
		httpErr := &httpx.HTTPError{}
		if errors.As(err2, &httpErr) {
			fmt.Printf("      Error Type: %s\n", httpErr.Type)
			fmt.Printf("      Status Code: %d\n", httpErr.StatusCode)
			fmt.Printf("      Message: %s\n", httpErr.Message)
		} else {
			fmt.Printf("      Unknown error: %v\n", err2)
		}
	}
}

// productionReadyClientExample shows a production-ready configuration
func productionReadyClientExample() {
	server := createProductionServer()
	defer server.Close()

	// Production logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Production circuit breaker config
	cbConfig := httpx.ConservativeCircuitBreakerConfig()
	cbConfig.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
		logger.Error("Circuit breaker state change",
			"service", name,
			"from_state", from,
			"to_state", to,
		)
	}

	// Production retry policy
	retryPolicy := httpx.ConservativeRetryPolicy()

	// Production-ready client
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientTimeout(10*time.Second),
		httpx.WithClientLogger(logger),
		httpx.WithClientLogLevel(slog.LevelInfo),
		httpx.WithClientDefaultHeaders(http.Header{
			"User-Agent":   []string{"ProductionApp/1.2.3"},
			"Accept":       []string{"application/json"},
			"Content-Type": []string{"application/json"},
		}),
		httpx.WithClientCircuitBreaker(cbConfig),
		httpx.WithClientRetryPolicy(retryPolicy),
	)

	fmt.Println("  Production client making API calls:")

	// Make some typical production API calls
	endpoints := []string{"/api/health", "/api/metrics", "/api/users"}

	for _, endpoint := range endpoints {
		req := httpx.NewRequest(http.MethodGet,
			httpx.WithPath(endpoint),
			httpx.WithHeader("X-Trace-ID", generateTraceID()),
		)

		response, err := client.Execute(*req, map[string]any{})
		if err != nil {
			logger.Error("API call failed",
				"endpoint", endpoint,
				"error", err.Error(),
			)
		} else {
			logger.Info("API call successful",
				"endpoint", endpoint,
				"status_code", response.StatusCode,
			)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Helper functions to create various test servers

func createServiceClient(serviceName, baseURL string) *httpx.Client {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(baseURL),
		httpx.WithClientTimeout(5*time.Second),
		httpx.WithClientLogger(logger),
		httpx.WithClientDefaultHeader("X-Service-Name", serviceName),
		httpx.WithClientDefaultRetryPolicy(),
	)
}

func createResilentTestServer() *httptest.Server {
	var requestCount int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++

		// Simulate intermittent failures
		if requestCount%3 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "simulated server error"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "success", "request": %d}`, requestCount)
	}))
}

func createUserServiceServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "users") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 123, "name": "John Doe", "email": "john@example.com"}`))
		}
	}))
}

func createOrderServiceServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "orders") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": 1, "userId": 123, "amount": 99.99}, {"id": 2, "userId": 123, "amount": 49.99}]`))
		}
	}))
}

func createSlowServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "slow") {
			// Simulate slow response (3 seconds)
			time.Sleep(3 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message": "slow response"}`))
		}
	}))
}

func createStreamingServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "stream") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)

			// Simulate streaming data
			for i := range 5 {
				fmt.Fprintf(w, "data chunk %d\n", i+1)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}))
}

func createNetworkErrorServer() *httptest.Server {
	// This server immediately closes connections to simulate network errors
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Force connection close to simulate network error
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
}

func createHTTPErrorServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error", "code": "E001"}`))
	}))
}

func createProductionServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "healthy", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
		case "/api/metrics":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"requests": 1234, "errors": 5, "uptime": "2h30m"}`))
		case "/api/users":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "endpoint not found"}`))
		}
	}))
}

func generateTraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}
