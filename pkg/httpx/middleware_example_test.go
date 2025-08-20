package httpx_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

// ExampleMiddleware demonstrates how to create a complete custom middleware
func ExampleMiddleware() {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Server received: %s %s\n", r.Method, r.URL.Path)
		if r.Header.Get("X-Custom") != "" {
			fmt.Printf("Custom header: %s\n", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "success"}`))
	}))
	defer server.Close()

	// Create logger
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create custom middleware that adds headers
	customMiddleware := &CustomHeaderMiddleware{headerName: "X-Custom", headerValue: "middleware-added"}

	// Create retry configuration
	retryConfig := httpx.DefaultRetryConfig()
	retryConfig.MaxRetries = 2
	retryConfig.BaseDelay = 10 * time.Millisecond

	// Create client with multiple middlewares
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddlewares(
			// Logging middleware (will be first in chain)
			httpx.NewLoggingMiddleware(logger, slog.LevelDebug),
			// Custom middleware
			customMiddleware,
			// Retry middleware (will be last in chain, closest to HTTP call)
			httpx.NewRetryMiddleware(retryConfig),
			// User-Agent middleware
			httpx.NewUserAgentMiddleware("ExampleClient/1.0", false),
		),
	)

	// Make a request
	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/example"))
	response, err := client.Execute(*req, map[string]any{})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response status: %d\n", response.StatusCode)

	// Show logs
	fmt.Printf("Logs contain request info: %v\n",
		bytes.Contains(logBuffer.Bytes(), []byte("HTTP request")))
	fmt.Printf("Logs contain response info: %v\n",
		bytes.Contains(logBuffer.Bytes(), []byte("HTTP response")))

	// Output:
	// Server received: GET /example
	// Custom header: middleware-added
	// Response status: 200
	// Logs contain request info: true
	// Logs contain response info: true
}

// CustomHeaderMiddleware is an example of a custom middleware
type CustomHeaderMiddleware struct {
	headerName  string
	headerValue string
}

func (m *CustomHeaderMiddleware) Name() string {
	return "custom-header"
}

func (m *CustomHeaderMiddleware) Execute(ctx context.Context, req *http.Request, next httpx.MiddlewareFunc) (*http.Response, error) {
	// Add custom header to request
	req.Header.Set(m.headerName, m.headerValue)

	// Call next middleware in chain
	resp, err := next(ctx, req)

	// Could also modify response here if needed
	return resp, err
}

// DemoInterceptors demonstrates how to use interceptors
func DemoInterceptors() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"intercepted": true}`))
	}))
	defer server.Close()

	// Create request interceptor
	requestInterceptor := &ExampleRequestInterceptor{}

	// Create response interceptor
	responseInterceptor := &ExampleResponseInterceptor{}

	// Create interceptor middleware
	interceptorMiddleware := httpx.NewInterceptorMiddleware("example-interceptors",
		requestInterceptor, responseInterceptor)

	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientMiddleware(interceptorMiddleware),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/intercepted"))
	response, err := client.Execute(*req, map[string]any{})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request was intercepted: %v\n", requestInterceptor.intercepted)
	fmt.Printf("Response was intercepted: %v\n", responseInterceptor.intercepted)
	fmt.Printf("Response status: %d\n", response.StatusCode)

	// Output:
	// Request was intercepted: true
	// Response was intercepted: true
	// Response status: 200
}

type ExampleRequestInterceptor struct {
	intercepted bool
}

func (i *ExampleRequestInterceptor) BeforeRequest(ctx context.Context, req *http.Request) error {
	i.intercepted = true
	req.Header.Set("X-Intercepted-Request", "true")
	return nil
}

type ExampleResponseInterceptor struct {
	intercepted bool
}

func (i *ExampleResponseInterceptor) AfterResponse(ctx context.Context, req *http.Request, resp *http.Response) (*http.Response, error) {
	i.intercepted = true
	resp.Header.Set("X-Intercepted-Response", "true")
	return resp, nil
}

// Run the examples in a test to ensure they work
func TestMiddlewareExamples(t *testing.T) {
	t.Run("example middleware", func(t *testing.T) {
		ExampleMiddleware()
	})

	t.Run("demo interceptors", func(t *testing.T) {
		DemoInterceptors()
	})
}
