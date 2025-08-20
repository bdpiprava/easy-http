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
	fmt.Println("=== Retry Mechanisms Example ===")

	// Example 1: Default retry policy
	fmt.Println("\n--- Example 1: Default Retry Policy ---")
	defaultRetryExample()

	// Example 2: Custom retry policy with exponential backoff
	fmt.Println("\n--- Example 2: Custom Retry Policy ---")
	customRetryExample()

	// Example 3: Aggressive retry policy
	fmt.Println("\n--- Example 3: Aggressive Retry Policy ---")
	aggressiveRetryExample()

	// Example 4: Conservative retry policy
	fmt.Println("\n--- Example 4: Conservative Retry Policy ---")
	conservativeRetryExample()

	// Example 5: Custom retry condition
	fmt.Println("\n--- Example 5: Custom Retry Condition ---")
	customRetryConditionExample()

	// Example 6: Retry strategies comparison
	fmt.Println("\n--- Example 6: Retry Strategies Comparison ---")
	retryStrategiesComparison()

	fmt.Println("\n=== Retry Examples Complete ===")
}

// defaultRetryExample demonstrates the default retry policy
func defaultRetryExample() {
	server := createUnstableServer(3) // Fails first 2 attempts
	defer server.Close()

	// Create client with default retry policy
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultRetryPolicy(),
		httpx.WithClientLogger(slog.Default()),
	)

	start := time.Now()
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response, err := client.Execute(*req, map[string]any{})
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error after %v: %v", duration, err)
	} else {
		fmt.Printf("Success after %v! Status: %d\n", duration, response.StatusCode)
		fmt.Printf("Response: %+v\n", response.Body)
	}
}

// customRetryExample shows how to configure custom retry behavior
func customRetryExample() {
	server := createUnstableServer(4) // Fails first 3 attempts
	defer server.Close()

	// Custom retry policy with exponential backoff
	customPolicy := httpx.RetryPolicy{
		MaxAttempts:          5,
		BaseDelay:            50 * time.Millisecond,
		MaxDelay:             2 * time.Second,
		Strategy:             httpx.RetryStrategyExponential,
		Multiplier:           2.0,
		RetryableStatusCodes: []int{500, 502, 503, 504},
	}

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientRetryPolicy(customPolicy),
		httpx.WithClientLogger(slog.Default()),
	)

	start := time.Now()
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response, err := client.Execute(*req, map[string]any{})
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error after %v: %v", duration, err)
	} else {
		fmt.Printf("Success after %v! Status: %d\n", duration, response.StatusCode)
	}
}

// aggressiveRetryExample demonstrates aggressive retry behavior
func aggressiveRetryExample() {
	server := createUnstableServer(5) // Fails first 4 attempts
	defer server.Close()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientAggressiveRetryPolicy(),
		httpx.WithClientLogger(slog.Default()),
	)

	start := time.Now()
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response, err := client.Execute(*req, map[string]any{})
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error after %v: %v", duration, err)
	} else {
		fmt.Printf("Success with aggressive policy after %v! Status: %d\n", duration, response.StatusCode)
	}
}

// conservativeRetryExample demonstrates conservative retry behavior
func conservativeRetryExample() {
	server := createUnstableServer(3) // This will likely fail with conservative policy
	defer server.Close()

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientConservativeRetryPolicy(),
		httpx.WithClientLogger(slog.Default()),
	)

	start := time.Now()
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response, err := client.Execute(*req, map[string]any{})
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Expected failure with conservative policy after %v: %v\n", duration, err)
	} else {
		fmt.Printf("Unexpected success after %v! Status: %d\n", duration, response.StatusCode)
	}
}

// customRetryConditionExample shows how to implement custom retry logic
func customRetryConditionExample() {
	server := createUnstableServer(4)
	defer server.Close()

	// Custom retry condition that only retries on 500 errors, max 3 attempts
	customConditionPolicy := httpx.RetryPolicy{
		MaxAttempts: 4,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Strategy:    httpx.RetryStrategyLinear,
		Multiplier:  1.5,
		Condition: func(attempt int, _ error, resp *http.Response) bool {
			fmt.Printf("  Custom condition check: attempt %d", attempt)

			// Max 3 retries (4 total attempts)
			if attempt >= 3 {
				fmt.Println(" - No more retries")
				return false
			}

			// Only retry on 500 Internal Server Error
			if resp != nil && resp.StatusCode == 500 {
				fmt.Println(" - Retrying due to 500 error")
				return true
			}

			// Don't retry on other conditions
			fmt.Println(" - Not retrying")
			return false
		},
	}

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientRetryPolicy(customConditionPolicy),
		httpx.WithClientLogger(slog.Default()),
	)

	start := time.Now()
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response, err := client.Execute(*req, map[string]any{})
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error after %v: %v", duration, err)
	} else {
		fmt.Printf("Success with custom condition after %v! Status: %d\n", duration, response.StatusCode)
	}
}

// retryStrategiesComparison demonstrates different retry strategies
func retryStrategiesComparison() {
	strategies := []struct {
		name     string
		strategy httpx.RetryStrategy
	}{
		{"Fixed Delay", httpx.RetryStrategyFixed},
		{"Linear Backoff", httpx.RetryStrategyLinear},
		{"Exponential Backoff", httpx.RetryStrategyExponential},
		{"Exponential with Jitter", httpx.RetryStrategyExponentialJitter},
	}

	for _, s := range strategies {
		fmt.Printf("\nTesting %s strategy:\n", s.name)

		server := createUnstableServer(3)

		policy := httpx.RetryPolicy{
			MaxAttempts:          4,
			BaseDelay:            100 * time.Millisecond,
			MaxDelay:             1 * time.Second,
			Strategy:             s.strategy,
			Multiplier:           2.0,
			JitterMax:            50 * time.Millisecond, // Only used for jitter strategy
			RetryableStatusCodes: []int{500},
		}

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientRetryPolicy(policy),
		)

		start := time.Now()
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
		response, err := client.Execute(*req, map[string]any{})
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("  Result: Failed after %v\n", duration)
		} else {
			fmt.Printf("  Result: Success after %v, Status: %d\n", duration, response.StatusCode)
		}

		server.Close()
	}
}

// createUnstableServer creates a server that fails the first N requests
func createUnstableServer(failCount int) *httptest.Server {
	var requestCount int
	var mu sync.Mutex

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requestCount++
		currentCount := requestCount
		mu.Unlock()

		fmt.Printf("Server received request #%d\n", currentCount)

		if currentCount <= failCount {
			// Fail the first N requests with 500 Internal Server Error
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "server temporarily unavailable", "attempt": ` +
				fmt.Sprintf("%d", currentCount) + `}`))
			return
		}

		// Succeed after N failures
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success!", "attempt": ` +
			fmt.Sprintf("%d", currentCount) + `, "recovered_after": ` +
			fmt.Sprintf("%d", failCount) + `}`))
	}))
}
