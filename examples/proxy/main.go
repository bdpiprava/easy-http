package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	// Example 1: HTTP Proxy
	fmt.Println("=== Example 1: HTTP Proxy ===")
	httpProxyExample()

	// Example 2: SOCKS5 Proxy
	fmt.Println("\n=== Example 2: SOCKS5 Proxy ===")
	socks5ProxyExample()

	// Example 3: Proxy with Authentication
	fmt.Println("\n=== Example 3: Proxy with Authentication ===")
	proxyWithAuthExample()

	// Example 4: Proxy with Bypass Rules
	fmt.Println("\n=== Example 4: Proxy with Bypass Rules ===")
	proxyWithBypassExample()

	// Example 5: System Proxy from Environment
	fmt.Println("\n=== Example 5: System Proxy from Environment ===")
	systemProxyExample()

	// Example 6: Per-Request Proxy Override
	fmt.Println("\n=== Example 6: Per-Request Proxy Override ===")
	perRequestProxyExample()
}

// Example 1: Configure HTTP proxy for all requests
func httpProxyExample() {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://httpbin.org"),
		httpx.WithClientProxy("http://proxy.example.com:8080"),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/ip"))
	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", resp.Body())
}

// Example 2: Configure SOCKS5 proxy
func socks5ProxyExample() {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://httpbin.org"),
		httpx.WithClientProxy("socks5://localhost:1080"),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/get"))
	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", resp.Body())
}

// Example 3: Proxy with authentication
func proxyWithAuthExample() {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://httpbin.org"),
		httpx.WithClientProxy("http://proxy.example.com:8080"),
		httpx.WithClientProxyAuth("username", "password"),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/headers"))
	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", resp.Body())
}

// Example 4: Proxy with bypass rules (no-proxy list)
func proxyWithBypassExample() {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://httpbin.org"),
		httpx.WithClientProxy("http://proxy.example.com:8080"),
		httpx.WithClientNoProxy([]string{
			"localhost",          // Bypass localhost
			"*.internal.com",     // Bypass wildcard domains
			"192.168.0.0/16",     // Bypass CIDR range
			".company.local",     // Bypass domain suffix
		}),
	)

	// This request goes through proxy (httpbin.org not in bypass list)
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/get"))
	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", resp.Body())
}

// Example 5: Use system proxy from environment variables
// Set HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables
func systemProxyExample() {
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://httpbin.org"),
		httpx.WithClientSystemProxy(),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/get"))
	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", resp.Body())
}

// Example 6: Override client proxy for specific request
func perRequestProxyExample() {
	// Client configured with default proxy
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL("https://httpbin.org"),
		httpx.WithClientProxy("http://default-proxy.example.com:8080"),
	)

	// Request 1: Use client's default proxy
	req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/get"))
	resp1, err := client.Execute(*req1, map[string]any{})
	if err != nil {
		log.Printf("Error (default proxy): %v", err)
	} else {
		fmt.Printf("Request 1 (default proxy) - Status: %d\n", resp1.StatusCode)
	}

	// Request 2: Override with different proxy
	req2 := httpx.NewRequest(
		http.MethodGet,
		httpx.WithPath("/get"),
		httpx.WithProxy("http://override-proxy.example.com:9090"),
		httpx.WithProxyAuth("user", "pass"),
	)
	resp2, err := client.Execute(*req2, map[string]any{})
	if err != nil {
		log.Printf("Error (override proxy): %v", err)
	} else {
		fmt.Printf("Request 2 (override proxy) - Status: %d\n", resp2.StatusCode)
	}

	// Request 3: Disable proxy for this request
	req3 := httpx.NewRequest(
		http.MethodGet,
		httpx.WithPath("/get"),
		httpx.WithoutProxy(),
	)
	resp3, err := client.Execute(*req3, map[string]any{})
	if err != nil {
		log.Printf("Error (no proxy): %v", err)
	} else {
		fmt.Printf("Request 3 (no proxy) - Status: %d\n", resp3.StatusCode)
	}
}
