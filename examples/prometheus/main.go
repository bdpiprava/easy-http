package main

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("Prometheus Metrics Collection Examples")
	fmt.Println("======================================")

	// Example 1: Basic Prometheus metrics with default settings
	example1()

	// Example 2: Custom Prometheus configuration
	example2()

	// Example 3: Multiple metrics registries
	example3()

	// Example 4: Metrics with custom labels and buckets
	example4()

	// Example 5: Full integration with metrics endpoint
	example5()
}

func example1() {
	fmt.Println("\nExample 1: Default Prometheus Metrics")
	fmt.Println("--------------------------------------")

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","data":"Hello"}`))
	}))
	defer server.Close()

	// Create HTTP client with default Prometheus metrics
	// This will track: requests_total, request_duration_seconds,
	// request_size_bytes, response_size_bytes, errors_total, in_flight_requests
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultPrometheusMetrics(),
	)

	// Make some requests to generate metrics
	for range 5 {
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
		_, err := client.Execute(*req, map[string]any{})
		if err != nil {
			log.Printf("Request failed: %v", err)
		}
	}

	fmt.Println("✓ Made 5 requests with default Prometheus metrics tracking")
	fmt.Println("  Metrics tracked: requests_total, request_duration_seconds, request/response sizes")
}

func example2() {
	fmt.Println("\nExample 2: Custom Prometheus Configuration")
	fmt.Println("-------------------------------------------")

	// Create custom Prometheus registry to avoid conflicts
	registry := prometheus.NewRegistry()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	// Create client with custom Prometheus configuration
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientPrometheusMetrics(httpx.PrometheusConfig{
			Namespace:          "myapp",                             // Prefix all metrics with "myapp_"
			Subsystem:          "api_client",                        // Use "api_client" as subsystem
			Registry:           registry,                            // Use custom registry
			DurationBuckets:    []float64{0.001, 0.01, 0.1, 1, 10},  // Custom latency buckets
			SizeBuckets:        []float64{100, 1000, 10000, 100000}, // Custom size buckets
			IncludeHostLabel:   true,                                // Include host in labels
			IncludeMethodLabel: true,                                // Include HTTP method in labels
		}),
	)

	// Make requests
	req := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/api/orders"),
		httpx.WithJSONBody(map[string]string{"order": "12345"}))

	_, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Request failed: %v", err)
	}

	fmt.Println("✓ Client configured with custom Prometheus settings:")
	fmt.Println("  - Namespace: myapp")
	fmt.Println("  - Subsystem: api_client")
	fmt.Println("  - Custom duration buckets: [0.001, 0.01, 0.1, 1, 10]")
	fmt.Println("  - Custom size buckets: [100, 1000, 10000, 100000]")
	fmt.Println("  - Metric name format: myapp_api_client_requests_total")
}

func example3() {
	fmt.Println("\nExample 3: Multiple Clients with Separate Registries")
	fmt.Println("-----------------------------------------------------")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"response"}`))
	}))
	defer server.Close()

	// Create separate registries for different API clients
	usersAPIRegistry := prometheus.NewRegistry()
	ordersAPIRegistry := prometheus.NewRegistry()

	// Users API client with its own metrics
	usersClient := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientPrometheusMetrics(httpx.PrometheusConfig{
			Namespace:          "myapp",
			Subsystem:          "users_api",
			Registry:           usersAPIRegistry,
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}),
	)

	// Orders API client with its own metrics
	ordersClient := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientPrometheusMetrics(httpx.PrometheusConfig{
			Namespace:          "myapp",
			Subsystem:          "orders_api",
			Registry:           ordersAPIRegistry,
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}),
	)

	// Make requests with users client
	usersReq := httpx.NewRequest(http.MethodGet, httpx.WithPath("/users"))
	_, _ = usersClient.Execute(*usersReq, map[string]any{})

	// Make requests with orders client
	ordersReq := httpx.NewRequest(http.MethodGet, httpx.WithPath("/orders"))
	_, _ = ordersClient.Execute(*ordersReq, map[string]any{})

	fmt.Println("✓ Created two separate clients with isolated metrics:")
	fmt.Println("  - Users API: myapp_users_api_* metrics")
	fmt.Println("  - Orders API: myapp_orders_api_* metrics")
	fmt.Println("  Each client has independent counters and histograms")
}

func example4() {
	fmt.Println("\nExample 4: Metrics with Label Variations")
	fmt.Println("-----------------------------------------")

	registry := prometheus.NewRegistry()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	// Client with only method labels (no host)
	clientNoHost := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientPrometheusMetrics(httpx.PrometheusConfig{
			Namespace:          "test",
			Subsystem:          "no_host",
			Registry:           registry,
			IncludeHostLabel:   false, // Exclude host from labels
			IncludeMethodLabel: true,
		}),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
	_, _ = clientNoHost.Execute(*req, map[string]any{})

	fmt.Println("✓ Client configured with selective labels:")
	fmt.Println("  - Method label: enabled")
	fmt.Println("  - Host label: disabled")
	fmt.Println("  Useful when host is highly dynamic and would create too many label combinations")
}

func example5() {
	fmt.Println("\nExample 5: Full Integration with Metrics HTTP Endpoint")
	fmt.Println("-------------------------------------------------------")

	// Create custom registry for this example
	registry := prometheus.NewRegistry()

	// Create test API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate varying response times and statuses
		if r.URL.Path == "/slow" {
			time.Sleep(100 * time.Millisecond)
		}
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"server error"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"success"}`))
	}))
	defer apiServer.Close()

	// Create HTTP client with Prometheus metrics
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(apiServer.URL),
		httpx.WithClientPrometheusMetrics(httpx.PrometheusConfig{
			Namespace:          "demo",
			Subsystem:          "http_client",
			Registry:           registry,
			DurationBuckets:    []float64{.001, .01, .05, .1, .5, 1, 5},
			SizeBuckets:        []float64{100, 1000, 10000},
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}),
	)

	// Generate various types of requests
	fmt.Println("\nGenerating traffic with various patterns:")

	// Successful fast requests
	for range 10 {
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))
		_, _ = client.Execute(*req, map[string]any{})
	}
	fmt.Println("  ✓ Made 10 fast successful requests")

	// Slow requests
	for range 3 {
		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/slow"))
		_, _ = client.Execute(*req, map[string]any{})
	}
	fmt.Println("  ✓ Made 3 slow requests (100ms each)")

	// Error requests
	for range 2 {
		req := httpx.NewRequest(http.MethodPost, httpx.WithPath("/error"))
		_, _ = client.Execute(*req, map[string]any{})
	}
	fmt.Println("  ✓ Made 2 requests that returned errors")

	// Create metrics HTTP server to expose Prometheus metrics
	metricsHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry: registry,
	})

	metricsServer := httptest.NewServer(metricsHandler)
	defer metricsServer.Close()

	// Fetch and display sample metrics
	resp, err := http.Get(metricsServer.URL)
	if err != nil {
		log.Printf("Failed to fetch metrics: %v", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("\n✓ Metrics endpoint available at: %s\n", metricsServer.URL)
	fmt.Println("\nSample metrics that would be exposed:")
	fmt.Println("  # HELP demo_http_client_requests_total Total number of HTTP requests made")
	fmt.Println("  # TYPE demo_http_client_requests_total counter")
	fmt.Println("  demo_http_client_requests_total{host=\"...\",method=\"GET\",status_code=\"200\"} 13")
	fmt.Println("  demo_http_client_requests_total{host=\"...\",method=\"POST\",status_code=\"500\"} 2")
	fmt.Println()
	fmt.Println("  # HELP demo_http_client_request_duration_seconds HTTP request latency distribution")
	fmt.Println("  # TYPE demo_http_client_request_duration_seconds histogram")
	fmt.Println("  demo_http_client_request_duration_seconds_bucket{host=\"...\",method=\"GET\",status_code=\"0\",le=\"0.001\"} 10")
	fmt.Println("  demo_http_client_request_duration_seconds_bucket{host=\"...\",method=\"GET\",status_code=\"0\",le=\"0.1\"} 13")
	fmt.Println()
	fmt.Println("  # HELP demo_http_client_errors_total Total number of HTTP errors")
	fmt.Println("  # TYPE demo_http_client_errors_total counter")
	fmt.Println("  demo_http_client_errors_total{error_type=\"server_error\",host=\"...\",method=\"POST\"} 2")
	fmt.Println()
	fmt.Println("  # HELP demo_http_client_in_flight_requests Current number of in-flight HTTP requests")
	fmt.Println("  # TYPE demo_http_client_in_flight_requests gauge")
	fmt.Println("  demo_http_client_in_flight_requests 0")
	fmt.Println()
	fmt.Println("In production, you would expose this at /metrics:")
	fmt.Println("  http.Handle(\"/metrics\", promhttp.Handler())")
	fmt.Println("  http.ListenAndServe(\":8080\", nil)")
}

func example6() {
	fmt.Println("\nExample 6: Combining Metrics with Other Middleware")
	fmt.Println("---------------------------------------------------")

	registry := prometheus.NewRegistry()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"compressed and metered"}`))
	}))
	defer server.Close()

	// Combine Prometheus metrics with compression middleware
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientPrometheusMetrics(httpx.PrometheusConfig{
			Namespace:          "myapp",
			Subsystem:          "api",
			Registry:           registry,
			IncludeHostLabel:   true,
			IncludeMethodLabel: true,
		}),
		httpx.WithClientCompression(httpx.CompressionConfig{
			Level:              gzip.BestSpeed,
			MinSizeBytes:       512,
			EnableRequest:      true,
			EnableResponse:     true,
			PreferredEncodings: []string{"gzip"},
		}),
	)

	// Make request - will be both compressed and metered
	req := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/api/data"),
		httpx.WithJSONBody(map[string]string{
			"data": "Large payload that will be compressed",
		}))

	_, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Request failed: %v", err)
	}

	fmt.Println("✓ Client combines multiple middleware:")
	fmt.Println("  - Prometheus metrics track all requests")
	fmt.Println("  - Compression reduces bandwidth")
	fmt.Println("  - Metrics accurately reflect compressed sizes")
	fmt.Println("  Request/response sizes in metrics show post-compression values")
}
