package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("=== Circuit Breaker Pattern Example ===")

	// Create a test server that simulates service failure and recovery
	failureCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount++
		fmt.Printf("Server received request #%d\n", failureCount)

		// Simulate service failure for first several requests
		if failureCount <= 5 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "service temporarily unavailable"}`))
			return
		}

		// Service recovers and returns success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "service is healthy", "request": ` + fmt.Sprintf("%d", failureCount) + `}`))
	}))
	defer server.Close()

	// Example 1: Default Circuit Breaker Configuration
	fmt.Println("\n--- Example 1: Default Circuit Breaker ---")
	demonstrateCircuitBreaker(server.URL, httpx.DefaultCircuitBreakerConfig(), "Default")

	// Reset server state
	failureCount = 0

	// Example 2: Aggressive Circuit Breaker Configuration
	fmt.Println("\n--- Example 2: Aggressive Circuit Breaker ---")
	demonstrateCircuitBreaker(server.URL, httpx.AggressiveCircuitBreakerConfig(), "Aggressive")

	// Reset server state
	failureCount = 0

	// Example 3: Conservative Circuit Breaker Configuration
	fmt.Println("\n--- Example 3: Conservative Circuit Breaker ---")
	demonstrateCircuitBreaker(server.URL, httpx.ConservativeCircuitBreakerConfig(), "Conservative")

	// Reset server state
	failureCount = 0

	// Example 4: Custom Circuit Breaker with State Change Monitoring
	fmt.Println("\n--- Example 4: Circuit Breaker with State Monitoring ---")
	customConfig := httpx.DefaultCircuitBreakerConfig()
	customConfig.Name = "monitored"
	customConfig.ReadyToTrip = func(counts httpx.Counts) bool {
		return counts.TotalFailures >= 3 // Trip after 3 failures
	}
	customConfig.Timeout = 2 * time.Second // 2-second timeout
	customConfig.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
		fmt.Printf("🔄 Circuit Breaker '%s' state changed: %s → %s\n", name, from, to)
	}

	demonstrateCircuitBreakerWithStateMonitoring(server.URL, customConfig)

	// Reset server state
	failureCount = 0

	// Example 5: Circuit Breaker with Retry Policy Integration
	fmt.Println("\n--- Example 5: Circuit Breaker + Retry Integration ---")
	demonstrateCircuitBreakerWithRetry(server.URL)

	fmt.Println("\n=== Circuit Breaker Examples Complete ===")
}

func demonstrateCircuitBreaker(serverURL string, config httpx.CircuitBreakerConfig, configName string) {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(serverURL),
		httpx.WithClientCircuitBreaker(config),
		httpx.WithClientLogger(slog.Default()),
	)

	fmt.Printf("Testing %s circuit breaker configuration:\n", configName)
	fmt.Printf("- ReadyToTrip threshold: varies by config\n")
	fmt.Printf("- Timeout: %v\n", config.Timeout)
	fmt.Printf("- MaxRequests (half-open): %d\n", config.MaxRequests)

	for i := 1; i <= 10; i++ {
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
			fmt.Printf("✅ SUCCESS: Status %d", response.StatusCode)
			if response.StatusCode == 200 {
				fmt.Printf(" - Service recovered!")
			}
		}

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}
}

func demonstrateCircuitBreakerWithStateMonitoring(serverURL string, config httpx.CircuitBreakerConfig) {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(serverURL),
		httpx.WithClientCircuitBreaker(config),
	)

	fmt.Printf("Testing circuit breaker with state change monitoring:\n")
	fmt.Printf("- Trip threshold: 3 failures\n")
	fmt.Printf("- Timeout: %v\n", config.Timeout)

	// Make several requests to trigger state changes
	for i := 1; i <= 8; i++ {
		fmt.Printf("\nRequest #%d: ", i)
		
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/health"))
		response, err := client.Execute(*req, map[string]any{})

		if err != nil {
			fmt.Printf("❌ BLOCKED: %v", err)
		} else {
			fmt.Printf("✅ Status %d", response.StatusCode)
		}

		// Wait for timeout after circuit breaker opens
		if i == 4 {
			fmt.Printf("\n⏳ Waiting for circuit breaker timeout (%v)...", config.Timeout)
			time.Sleep(config.Timeout + 100*time.Millisecond)
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func demonstrateCircuitBreakerWithRetry(serverURL string) {
	// Configure both circuit breaker and retry policy
	cbConfig := httpx.DefaultCircuitBreakerConfig()
	cbConfig.ReadyToTrip = func(counts httpx.Counts) bool {
		return counts.TotalFailures >= 4 // Allow more failures before tripping
	}
	cbConfig.OnStateChange = func(name string, from, to httpx.CircuitBreakerState) {
		fmt.Printf("🔄 Circuit Breaker state: %s → %s\n", from, to)
	}

	retryPolicy := httpx.DefaultRetryPolicy()
	retryPolicy.MaxAttempts = 2 // Only 1 retry
	retryPolicy.BaseDelay = 100 * time.Millisecond

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(serverURL),
		httpx.WithClientCircuitBreaker(cbConfig),
		httpx.WithClientRetryPolicy(retryPolicy),
	)

	fmt.Printf("Testing Circuit Breaker + Retry integration:\n")
	fmt.Printf("- Circuit Breaker trips after 4 failures\n")
	fmt.Printf("- Retry policy: max 2 attempts with exponential backoff\n")
	fmt.Printf("- Note: Each retry attempt counts towards circuit breaker failure count\n")

	for i := 1; i <= 6; i++ {
		fmt.Printf("\nRequest #%d: ", i)
		
		start := time.Now()
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/health"))
		response, err := client.Execute(*req, map[string]any{})
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ FAILED after %v: %v", duration, err)
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