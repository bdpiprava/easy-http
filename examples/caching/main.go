package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

// User represents a simple user model
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	fmt.Println("HTTP Caching Example")
	fmt.Println("===================")

	// Example 1: Enable default caching
	example1()

	// Example 2: Custom cache configuration
	example2()

	// Example 3: Cache statistics
	example3()
}

func example1() {
	fmt.Println("\nExample 1: Default HTTP Caching")
	fmt.Println("--------------------------------")

	// Create client with default caching enabled
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://jsonplaceholder.typicode.com"),
		httpx.WithClientDefaultCache(), // Enable caching with defaults
	)

	// First request - cache miss, fetches from server
	fmt.Println("Making first request to /users/1...")
	req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users/1"))
	resp1, err := client.Execute(*req1, User{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Response 1 - Status: %d, User: %+v\n", resp1.StatusCode, resp1.Body)

	// Second request - cache hit, served from cache (if server supports caching)
	fmt.Println("Making second request to /users/1...")
	req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users/1"))
	resp2, err := client.Execute(*req2, User{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("Response 2 - Status: %d, User: %+v\n", resp2.StatusCode, resp2.Body)

	fmt.Println("Note: Subsequent requests may use cached response or conditional requests")
}

func example2() {
	fmt.Println("\nExample 2: Custom Cache Configuration")
	fmt.Println("-------------------------------------")

	// Create custom cache configuration
	cacheConfig := httpx.CacheConfig{
		Backend:      httpx.NewInMemoryCache(5000), // Larger cache size
		DefaultTTL:   10 * time.Minute,             // 10 minute TTL
		MaxSizeBytes: 50 * 1024 * 1024,             // 50MB max
		SkipCacheFor: func(req *http.Request) bool {
			// Don't cache requests with Authorization header
			return req.Header.Get("Authorization") != ""
		},
	}

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://jsonplaceholder.typicode.com"),
		httpx.WithClientCache(cacheConfig),
	)

	// Make multiple requests to demonstrate caching
	for i := 1; i <= 3; i++ {
		fmt.Printf("\nRequest %d to /users/1...\n", i)
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users/1"))
		resp, err := client.Execute(*req, User{})
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("Status: %d, User Name: %s\n", resp.StatusCode, resp.Body.(User).Name)

		// Small delay between requests
		time.Sleep(500 * time.Millisecond)
	}
}

func example3() {
	fmt.Println("\nExample 3: Cache Backend with Statistics")
	fmt.Println("----------------------------------------")

	// Create cache backend to track statistics
	cacheBackend := httpx.NewInMemoryCache(100)

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://jsonplaceholder.typicode.com"),
		httpx.WithClientCache(httpx.CacheConfig{
			Backend:    cacheBackend,
			DefaultTTL: 5 * time.Minute,
		}),
	)

	// Make several requests
	urls := []string{"/users/1", "/users/2", "/users/1", "/users/3", "/users/1"}
	for i, path := range urls {
		fmt.Printf("\nRequest %d to %s...\n", i+1, path)
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath(path))
		resp, err := client.Execute(*req, User{})
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("Status: %d\n", resp.StatusCode)

		// Show cache statistics
		stats := cacheBackend.Stats()
		fmt.Printf("Cache Stats - Hits: %d, Misses: %d, Size: %d, Evictions: %d\n",
			stats.Hits, stats.Misses, stats.Size, stats.Evictions)

		time.Sleep(300 * time.Millisecond)
	}

	// Final statistics
	fmt.Println("\nFinal Cache Statistics:")
	stats := cacheBackend.Stats()
	fmt.Printf("Total Hits: %d\n", stats.Hits)
	fmt.Printf("Total Misses: %d\n", stats.Misses)
	fmt.Printf("Cache Size: %d entries\n", stats.Size)
	fmt.Printf("Total Evictions: %d\n", stats.Evictions)

	// Calculate hit rate
	total := stats.Hits + stats.Misses
	if total > 0 {
		hitRate := float64(stats.Hits) / float64(total) * 100
		fmt.Printf("Hit Rate: %.2f%%\n", hitRate)
	}
}
