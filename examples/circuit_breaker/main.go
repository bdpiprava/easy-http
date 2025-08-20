package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("=== Circuit Breaker Pattern Example ===")

	// Example 1: Default circuit breaker configuration
	fmt.Println("\n--- Example 1: Default Circuit Breaker ---")
	defaultCircuitBreakerExample()

	// Example 2: Aggressive circuit breaker configuration
	fmt.Println("\n--- Example 2: Aggressive Circuit Breaker ---")
	aggressiveCircuitBreakerExample()

	// Example 3: Conservative circuit breaker configuration
	fmt.Println("\n--- Example 3: Conservative Circuit Breaker ---")
	conservativeCircuitBreakerExample()

	// Example 4: Circuit breaker with state change monitoring
	fmt.Println("\n--- Example 4: Circuit Breaker with State Monitoring ---")
	circuitBreakerWithStateMonitoring()

	// Example 5: Circuit breaker with retry policy integration
	fmt.Println("\n--- Example 5: Circuit Breaker + Retry Integration ---")
	circuitBreakerWithRetryIntegration()

	fmt.Println("\n=== Circuit Breaker Examples Complete ===")
}

// defaultCircuitBreakerExample demonstrates default circuit breaker behavior
func defaultCircuitBreakerExample() {
	server := createFailingServer(7) // Service fails for 7 requests, then recovers
	defer server.Close()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultCircuitBreaker(),
		httpx.WithClientLogger(slog.Default()),
	)

	fmt.Println("Testing default circuit breaker configuration:")
	fmt.Println("- Trips when failure rate ≥ 50% with at least 5 requests")
	fmt.Println("- Half-open allows 1 request to test recovery")
	fmt.Println("- 60-second timeout before trying again")

	demonstrateCircuitBreaker(client, 10)
}

// aggressiveCircuitBreakerExample shows more aggressive circuit breaker settings
func aggressiveCircuitBreakerExample() {
	server := createFailingServer(5)
	defer server.Close()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientAggressiveCircuitBreaker(),
		httpx.WithClientLogger(slog.Default()),
	)

	fmt.Println("Testing aggressive circuit breaker configuration:")
	fmt.Println("- Trips when failure rate ≥ 30% with at least 3 requests")
	fmt.Println("- Shorter timeout periods for faster recovery detection")

	demonstrateCircuitBreaker(client, 8)
}

// conservativeCircuitBreakerExample shows conservative circuit breaker settings
func conservativeCircuitBreakerExample() {
	server := createFailingServer(12) // More failures needed to trip
	defer server.Close()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientConservativeCircuitBreaker(),
		httpx.WithClientLogger(slog.Default()),
	)

	fmt.Println("Testing conservative circuit breaker configuration:")
	fmt.Println("- Trips when failure rate ≥ 80% with at least 10 requests")
	fmt.Println("- Allows more requests in half-open state")
	fmt.Println("- Longer timeout periods to avoid premature recovery attempts")

	demonstrateCircuitBreaker(client, 15)
}

// circuitBreakerWithStateMonitoring demonstrates monitoring circuit breaker state changes
func circuitBreakerWithStateMonitoring() {
	server := createFailingServer(4)
	defer server.Close()

	// Custom circuit breaker with state change monitoring
	config := httpx.DefaultCircuitBreakerConfig()
	config.Name = "monitored"
	config.ReadyToTrip = func(counts httpx.Counts) bool {
		return counts.TotalFailures >= 3 // Trip after 3 failures
	}
	config.Timeout = 2 * time.Second // 2-second timeout
	config.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
		fmt.Printf("🔄 Circuit Breaker '%s' state changed: %s → %s\n", name, from, to)
	}

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientCircuitBreaker(config),
	)

	fmt.Println("Testing circuit breaker with state change monitoring:")
	fmt.Println("- Trip threshold: 3 failures")
	fmt.Println("- Timeout: 2 seconds")
	fmt.Println("- State changes will be logged")

	// Make several requests to trigger state changes
	for i := 1; i <= 8; i++ {
		fmt.Printf("\nRequest #%d: ", i)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/health"))
		response, err := client.Execute(*req, map[string]any{})

		if err != nil {
			if httpx.IsCircuitBreakerError(err) {
				fmt.Printf("❌ BLOCKED by circuit breaker: %v", err)
			} else {
				fmt.Printf("❌ ERROR: %v", err)
			}
		} else {
			fmt.Printf("✅ Status %d", response.StatusCode)
			if response.StatusCode == 200 {
				fmt.Printf(" - Service recovered!")
			}
		}

		// Wait for timeout after circuit breaker opens
		if i == 4 {
			fmt.Printf("\n⏳ Waiting for circuit breaker timeout (2s)...")
			time.Sleep(2*time.Second + 100*time.Millisecond)
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// circuitBreakerWithRetryIntegration shows how circuit breaker works with retry policies
func circuitBreakerWithRetryIntegration() {
	server := createFailingServer(6)
	defer server.Close()

	// Configure both circuit breaker and retry policy
	cbConfig := httpx.DefaultCircuitBreakerConfig()
	cbConfig.ReadyToTrip = func(counts httpx.Counts) bool {
		return counts.TotalFailures >= 5 // Allow more failures before tripping
	}
	cbConfig.OnStateChange = func(_ string, from, to httpx.CircuitBreakerState) {
		fmt.Printf("🔄 Circuit Breaker state: %s → %s\n", from, to)
	}

	retryPolicy := httpx.DefaultRetryPolicy()
	retryPolicy.MaxAttempts = 2 // Only 1 retry attempt
	retryPolicy.BaseDelay = 100 * time.Millisecond

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientCircuitBreaker(cbConfig),
		httpx.WithClientRetryPolicy(retryPolicy),
	)

	fmt.Println("Testing Circuit Breaker + Retry integration:")
	fmt.Println("- Circuit Breaker trips after 5 failures")
	fmt.Println("- Retry policy: max 2 attempts")
	fmt.Println("- Note: Each retry attempt counts towards circuit breaker failure count")

	for i := 1; i <= 6; i++ {
		fmt.Printf("\nRequest #%d: ", i)

		start := time.Now()
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/health"))
		response, err := client.Execute(*req, map[string]any{})
		duration := time.Since(start)

		if err != nil {
			if httpx.IsCircuitBreakerError(err) {
				fmt.Printf("❌ BLOCKED by circuit breaker after %v: %v", duration, err)
			} else {
				fmt.Printf("❌ FAILED after %v: %v", duration, err)
			}
		} else {
			fmt.Printf("✅ SUCCESS after %v: Status %d", duration, response.StatusCode)
			if body, ok := response.Body.(map[string]any); ok {
				if msg, exists := body["message"]; exists {
					fmt.Printf(" - %v", msg)
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// demonstrateCircuitBreaker makes a series of requests to show circuit breaker behavior
func demonstrateCircuitBreaker(client *httpx.Client, numRequests int) {
	for i := 1; i <= numRequests; i++ {
		fmt.Printf("\nRequest #%d: ", i)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/health"))
		response, err := client.Execute(*req, map[string]any{})

		if err != nil {
			if httpx.IsCircuitBreakerError(err) {
				fmt.Printf("❌ BLOCKED by circuit breaker")
			} else {
				fmt.Printf("❌ ERROR: %v", err)
			}
		} else {
			fmt.Printf("✅ SUCCESS: Status %d", response.StatusCode)
			if response.StatusCode == 200 {
				fmt.Printf(" - Service recovered!")
			}
		}

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}
}

// createFailingServer creates a server that fails for the first N requests, then succeeds
func createFailingServer(failureCount int) *httptest.Server {
	var requestCount int
	var mu sync.Mutex

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requestCount++
		currentCount := requestCount
		mu.Unlock()

		fmt.Printf("Server received request #%d\n", currentCount)

		// Simulate service failure for first several requests
		if currentCount <= failureCount {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "service temporarily unavailable"}`))
			return
		}

		// Service recovers and returns success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "service is healthy", "request": ` +
			fmt.Sprintf("%d", currentCount) + `}`))
	}))
}
