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
	fmt.Println("=== Advanced Retry Logic Example ===")
	
	// Create a test server that fails the first few requests
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		fmt.Printf("Server received attempt #%d\n", attemptCount)
		
		if attemptCount < 3 {
			// Fail the first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server temporarily unavailable"}`))
			return
		}
		
		// Succeed on the 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success!", "attempt": ` + fmt.Sprintf("%d", attemptCount) + `}`))
	}))
	defer server.Close()

	// Example 1: Default Retry Policy
	fmt.Println("\n--- Example 1: Default Retry Policy ---")
	attemptCount = 0
	client1 := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultRetryPolicy(),
		httpx.WithClientLogger(slog.Default()),
	)

	req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response1, err := client1.Execute(*req1, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success! Status: %d, Body: %+v\n", response1.StatusCode, response1.Body)
	}

	// Example 2: Custom Retry Policy with Exponential Backoff
	fmt.Println("\n--- Example 2: Custom Retry Policy ---")
	attemptCount = 0
	customPolicy := httpx.RetryPolicy{
		MaxAttempts:          4,
		BaseDelay:           100 * time.Millisecond,
		MaxDelay:            2 * time.Second,
		Strategy:            httpx.RetryStrategyExponential,
		Multiplier:          2.0,
		RetryableStatusCodes: []int{500, 502, 503, 504},
	}

	client2 := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientRetryPolicy(customPolicy),
		httpx.WithClientLogger(slog.Default()),
	)

	req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response2, err := client2.Execute(*req2, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success! Status: %d, Body: %+v\n", response2.StatusCode, response2.Body)
	}

	// Example 3: Aggressive Retry Policy with Jitter
	fmt.Println("\n--- Example 3: Aggressive Retry with Jitter ---")
	attemptCount = 0
	client3 := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientAggressiveRetryPolicy(),
		httpx.WithClientLogger(slog.Default()),
	)

	req3 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response3, err := client3.Execute(*req3, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success! Status: %d, Body: %+v\n", response3.StatusCode, response3.Body)
	}

	// Example 4: Conservative Retry Policy
	fmt.Println("\n--- Example 4: Conservative Retry Policy (will fail) ---")
	attemptCount = 0
	client4 := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientConservativeRetryPolicy(),
		httpx.WithClientLogger(slog.Default()),
	)

	req4 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response4, err := client4.Execute(*req4, map[string]any{})
	if err != nil {
		log.Printf("Expected failure with conservative policy: %v", err)
	} else {
		fmt.Printf("Unexpected success! Status: %d, Body: %+v\n", response4.StatusCode, response4.Body)
	}

	// Example 5: Custom Retry Condition
	fmt.Println("\n--- Example 5: Custom Retry Condition ---")
	attemptCount = 0
	customConditionPolicy := httpx.RetryPolicy{
		MaxAttempts: 5,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Strategy:    httpx.RetryStrategyLinear,
		Multiplier:  1.5,
		Condition: func(attempt int, err error, resp *http.Response) bool {
			// Custom logic: only retry on 500 errors, max 3 attempts
			if attempt >= 3 {
				return false
			}
			if resp != nil && resp.StatusCode == 500 {
				fmt.Printf("Custom condition: Retrying attempt %d due to 500 error\n", attempt+1)
				return true
			}
			return false
		},
	}

	client5 := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientRetryPolicy(customConditionPolicy),
		httpx.WithClientLogger(slog.Default()),
	)

	req5 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
	response5, err := client5.Execute(*req5, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success! Status: %d, Body: %+v\n", response5.StatusCode, response5.Body)
	}

	// Example 6: Retry Strategies Comparison
	fmt.Println("\n--- Example 6: Retry Strategies Comparison ---")
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
		attemptCount = 0
		
		policy := httpx.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			Strategy:    s.strategy,
			Multiplier:  2.0,
			JitterMax:   50 * time.Millisecond, // Only used for jitter strategy
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
	}

	fmt.Println("\n=== Example completed ===")
}