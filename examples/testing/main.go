package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"
)

// Example demonstrates how to use the httpx testing utilities
// to test HTTP clients and services
func main() {
	fmt.Println("=== Easy-HTTP Testing Utilities Examples ===")

	basicMockServerExample()
	requestMatchersExample()
	assertionsExample()
	errorSimulationExample()
	flakyBehaviorExample()
	fmt.Println("\n=== All Examples Completed ===")
}

// basicMockServerExample demonstrates basic mock server setup and usage
func basicMockServerExample() {
	fmt.Println("1. Basic Mock Server Example")
	fmt.Println("   Creating a mock server with simple responses...")

	// Create a new mock server
	mock := httpxtesting.NewMockServer()
	defer mock.Close()

	// Configure responses using fluent API
	mock.OnGet("/users").
		WithStatus(http.StatusOK).
		WithJSON(map[string]interface{}{
			"users": []map[string]interface{}{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
		})

	mock.OnPost("/users").
		WithStatus(http.StatusCreated).
		WithJSON(map[string]interface{}{
			"id":      3,
			"name":    "Charlie",
			"created": true,
		})

	// Make requests to the mock server
	resp1, _ := http.Get(mock.URL() + "/users")
	defer resp1.Body.Close()
	body1, _ := io.ReadAll(resp1.Body)

	resp2, _ := http.Post(mock.URL()+"/users", "application/json", nil)
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)

	fmt.Printf("   GET /users: %d - %s\n", resp1.StatusCode, string(body1))
	fmt.Printf("   POST /users: %d - %s\n\n", resp2.StatusCode, string(body2))
}

// requestMatchersExample demonstrates advanced request matching
func requestMatchersExample() {
	fmt.Println("2. Request Matchers Example")
	fmt.Println("   Using matchers for flexible request routing...")

	mock := httpxtesting.NewMockServer()
	defer mock.Close()

	// Exact path matching
	mock.On(httpxtesting.ExactPath("/api/users")).
		WithStatus(http.StatusOK).
		WithBodyString("exact match")

	// Path prefix matching
	mock.On(httpxtesting.PathPrefix("/api/")).
		WithStatus(http.StatusOK).
		WithBodyString("prefix match")

	// Regex path matching
	mock.On(httpxtesting.PathRegex(`/users/\d+`)).
		WithStatus(http.StatusOK).
		WithBodyString("regex match")

	// Query parameter matching
	mock.On(httpxtesting.HasQueryParam("filter", "active")).
		WithStatus(http.StatusOK).
		WithBodyString("query param match")

	// Header matching
	mock.On(httpxtesting.HasHeader("Authorization", "Bearer token")).
		WithStatus(http.StatusOK).
		WithBodyString("header match")

	// Composite matchers (AND)
	mock.On(
		httpxtesting.And(
			httpxtesting.MethodIs("POST"),
			httpxtesting.ExactPath("/api/submit"),
			httpxtesting.HasHeader("Content-Type", "application/json"),
		),
	).WithStatus(http.StatusOK).WithBodyString("composite match")

	// Test the matchers
	testMatcher(mock, "GET", "/api/users", nil, "Exact path")
	testMatcher(mock, "GET", "/api/posts", nil, "Path prefix")
	testMatcher(mock, "GET", "/users/123", nil, "Regex path")
	testMatcher(mock, "GET", "/search?filter=active", nil, "Query param")

	// Test with headers
	req, _ := http.NewRequest("GET", mock.URL()+"/protected", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("   Header match: %s\n\n", string(body))
}

// assertionsExample demonstrates request verification and assertions
func assertionsExample() {
	fmt.Println("3. Assertions Example")
	fmt.Println("   Verifying requests and their properties...")

	mock := httpxtesting.NewMockServer()
	defer mock.Close()

	mock.OnGet("/api/data").WithStatus(http.StatusOK)
	mock.OnPost("/api/data").WithStatus(http.StatusCreated)

	// Make some requests
	req1, _ := http.NewRequest("GET", mock.URL()+"/api/data?id=123", nil)
	req1.Header.Set("X-Custom", "test-value")
	resp1, _ := http.DefaultClient.Do(req1)
	resp1.Body.Close()

	resp2, _ := http.Post(mock.URL()+"/api/data", "application/json", nil)
	resp2.Body.Close()

	// Use assertions to verify requests
	assert := mock.Assert()

	if err := assert.RequestCount(2); err != nil {
		fmt.Printf("   ❌ Request count: %v\n", err)
	} else {
		fmt.Println("   ✅ Request count: 2 requests received")
	}

	if err := assert.RequestTo("/api/data"); err != nil {
		fmt.Printf("   ❌ Request to /api/data: %v\n", err)
	} else {
		fmt.Println("   ✅ Request to /api/data received")
	}

	if err := assert.RequestWithMethod("POST"); err != nil {
		fmt.Printf("   ❌ POST request: %v\n", err)
	} else {
		fmt.Println("   ✅ POST request received")
	}

	if err := assert.RequestWithHeader("X-Custom", "test-value"); err != nil {
		fmt.Printf("   ❌ Custom header: %v\n", err)
	} else {
		fmt.Println("   ✅ Request with custom header received")
	}

	if err := assert.RequestWithQueryParam("id", "123"); err != nil {
		fmt.Printf("   ❌ Query param: %v\n", err)
	} else {
		fmt.Println("   ✅ Request with query param received")
	}

	// Verify request sequence
	if err := assert.VerifySequence("/api/data", "/api/data"); err != nil {
		fmt.Printf("   ❌ Sequence: %v\n", err)
	} else {
		fmt.Println("   ✅ Request sequence verified")
	}

	// Access request history
	requests := mock.Requests()
	fmt.Printf("   📝 Total requests recorded: %d\n\n", len(requests))
}

// errorSimulationExample demonstrates error simulation features
func errorSimulationExample() {
	fmt.Println("4. Error Simulation Example")
	fmt.Println("   Testing error handling scenarios...")

	mock := httpxtesting.NewMockServer()
	defer mock.Close()

	// Simulate various error conditions
	mock.OnGet("/400").SimulateError().BadRequest("invalid input")
	mock.OnGet("/401").SimulateError().Unauthorized()
	mock.OnGet("/403").SimulateError().Forbidden()
	mock.OnGet("/404").SimulateError().NotFound()
	mock.OnGet("/429").SimulateError().TooManyRequests(60)
	mock.OnGet("/500").SimulateError().InternalServerError()
	mock.OnGet("/503").SimulateError().ServiceUnavailable(120)
	mock.OnGet("/504").SimulateError().GatewayTimeout()

	// Simulate slow responses
	mock.OnGet("/slow").SimulateError().Slow(100 * time.Millisecond)

	// Test error responses
	testError(mock, "/400", "Bad Request")
	testError(mock, "/401", "Unauthorized")
	testError(mock, "/404", "Not Found")
	testError(mock, "/429", "Too Many Requests")
	testError(mock, "/500", "Internal Server Error")
	testError(mock, "/503", "Service Unavailable")

	// Test slow response
	start := time.Now()
	resp, _ := http.Get(mock.URL() + "/slow")
	resp.Body.Close()
	elapsed := time.Since(start)
	fmt.Printf("   Slow response took %v (expected ≥100ms)\n\n", elapsed)
}

// flakyBehaviorExample demonstrates flaky response simulation
func flakyBehaviorExample() {
	fmt.Println("5. Flaky Behavior Example")
	fmt.Println("   Testing intermittent failures...")

	mock := httpxtesting.NewMockServer()
	defer mock.Close()

	// Configure flaky endpoint: fail 2 times, succeed 3 times, repeat
	mock.OnFlaky("/flaky", 2).
		WithPattern(3, 2).
		Configure()

	// Make multiple requests to see the pattern
	fmt.Println("   Making 10 requests to flaky endpoint:")
	for i := 1; i <= 10; i++ {
		resp, _ := http.Get(mock.URL() + "/flaky")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		status := "✅"
		if resp.StatusCode != http.StatusOK {
			status = "❌"
		}
		fmt.Printf("   Request %2d: %s %d - %s\n", i, status, resp.StatusCode, string(body))
	}
	fmt.Println()
}

// Helper function to test request matchers
func testMatcher(mock *httpxtesting.MockServer, method, path string, headers map[string]string, description string) {
	req, _ := http.NewRequest(method, mock.URL()+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("   %s: %s\n", description, string(body))
}

// Helper function to test error responses
func testError(mock *httpxtesting.MockServer, path, description string) {
	resp, _ := http.Get(mock.URL() + path)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var errData map[string]interface{}
	if err := json.Unmarshal(body, &errData); err == nil {
		fmt.Printf("   %s (%d): %v\n", description, resp.StatusCode, errData["error"])
	} else {
		fmt.Printf("   %s (%d): %s\n", description, resp.StatusCode, string(body))
	}
}
