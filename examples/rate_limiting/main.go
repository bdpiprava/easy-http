package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("HTTP Rate Limiting Example")
	fmt.Println("===========================")

	// Example 1: Basic rate limiting with default settings
	example1()

	// Example 2: Custom rate limit configuration
	example2()

	// Example 3: Per-host rate limiting
	example3()

	// Example 4: Handling rate limit exceeded
	example4()
}

func example1() {
	fmt.Println("\nExample 1: Default Rate Limiting")
	fmt.Println("----------------------------------")

	// Create a test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"request":%d}`, requestCount)
	}))
	defer server.Close()

	// Create client with default rate limiting (10 req/sec with burst of 20)
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultRateLimit(),
	)

	// Make 5 rapid requests - all should succeed within burst
	fmt.Println("Making 5 rapid requests...")
	for i := 1; i <= 5; i++ {
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
		resp, err := client.Execute(*req, map[string]any{})
		if err != nil {
			log.Printf("Request %d failed: %v", i, err)
			continue
		}
		fmt.Printf("Request %d: Status %d\n", i, resp.StatusCode)
	}

	fmt.Printf("Total server requests: %d\n", requestCount)
}

func example2() {
	fmt.Println("\nExample 2: Custom Rate Limit Configuration")
	fmt.Println("------------------------------------------")

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "99")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"success"}`))
	}))
	defer server.Close()

	// Create client with custom rate limiting: 2 req/sec, burst of 3
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientRateLimit(httpx.RateLimitConfig{
			Strategy:        httpx.RateLimitTokenBucket,
			RequestsPerSec:  2.0,
			BurstSize:       3,
			WaitOnLimit:     true,            // Wait when limit reached
			MaxWaitDuration: 2 * time.Second, // Max wait 2 seconds
		}),
	)

	// Make requests with timing
	fmt.Println("Making 4 requests with 2 req/sec limit (burst=3)...")
	start := time.Now()
	for i := 1; i <= 4; i++ {
		reqStart := time.Now()
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
		resp, err := client.Execute(*req, map[string]any{})
		elapsed := time.Since(reqStart)

		if err != nil {
			log.Printf("Request %d failed after %.3fs: %v", i, elapsed.Seconds(), err)
			continue
		}
		fmt.Printf("Request %d: Status %d (took %.3fs)\n", i, resp.StatusCode, elapsed.Seconds())
	}
	totalElapsed := time.Since(start)
	fmt.Printf("Total time: %.3fs\n", totalElapsed.Seconds())
}

func example3() {
	fmt.Println("\nExample 3: Per-Host Rate Limiting")
	fmt.Println("----------------------------------")

	// Create two test servers
	server1Calls := 0
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		server1Calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"1"}`))
	}))
	defer server1.Close()

	server2Calls := 0
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		server2Calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"2"}`))
	}))
	defer server2.Close()

	// Create client with per-host rate limiting
	client := httpx.NewClientWithConfig(
		httpx.WithClientRateLimit(httpx.RateLimitConfig{
			RequestsPerSec: 3,
			BurstSize:      3,
			PerHost:        true, // Different rate limits per host
			WaitOnLimit:    false,
		}),
	)

	// Make 3 requests to each server - all should succeed
	fmt.Println("Making 3 requests to server1 and 3 to server2...")

	var wg sync.WaitGroup
	// Server 1 requests
	for i := range 3 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httpx.NewRequest(http.MethodGet,
				httpx.WithBaseURL(server1.URL),
				httpx.WithPath("/api"))
			_, err := client.Execute(*req, map[string]any{})
			if err != nil {
				fmt.Printf("Server1 request %d failed: %v\n", idx+1, err)
			} else {
				fmt.Printf("Server1 request %d: Success\n", idx+1)
			}
		}(i)
	}

	// Server 2 requests
	for i := range 3 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httpx.NewRequest(http.MethodGet,
				httpx.WithBaseURL(server2.URL),
				httpx.WithPath("/api"))
			_, err := client.Execute(*req, map[string]any{})
			if err != nil {
				fmt.Printf("Server2 request %d failed: %v\n", idx+1, err)
			} else {
				fmt.Printf("Server2 request %d: Success\n", idx+1)
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("\nServer1 received %d calls\n", server1Calls)
	fmt.Printf("Server2 received %d calls\n", server2Calls)
	fmt.Println("Both servers have independent rate limits!")
}

func example4() {
	fmt.Println("\nExample 4: Handling Rate Limit Exceeded")
	fmt.Println("----------------------------------------")

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	// Create client with aggressive limits and NO waiting
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientRateLimit(httpx.RateLimitConfig{
			RequestsPerSec:  1,
			BurstSize:       2,
			WaitOnLimit:     false, // Don't wait - fail immediately
			MaxWaitDuration: 0,
		}),
	)

	// Make 5 rapid requests - expect some to fail
	fmt.Println("Making 5 rapid requests (limit: 2/burst, no waiting)...")
	successCount := 0
	failCount := 0

	for i := 1; i <= 5; i++ {
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
		_, err := client.Execute(*req, map[string]any{})
		if err != nil {
			failCount++
			fmt.Printf("Request %d: ❌ Rate limited - %v\n", i, err)
		} else {
			successCount++
			fmt.Printf("Request %d: ✅ Success\n", i)
		}
	}

	fmt.Printf("\nResults:\n")
	fmt.Printf("  Successful: %d\n", successCount)
	fmt.Printf("  Rate limited: %d\n", failCount)
	fmt.Printf("  Server calls: %d\n", callCount)
	fmt.Println("\nThis prevents overwhelming the server with too many requests!")
}
