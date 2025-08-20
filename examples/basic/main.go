package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

// User represents a user in our examples
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Post represents a blog post
type Post struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	UserID int    `json:"userId"`
}

func main() {
	fmt.Println("=== Basic HTTP Operations Example ===")

	// Create a test server to demonstrate the HTTP operations
	server := createTestServer()
	defer server.Close()

	// Example 1: Simple GET request using the convenience function
	fmt.Println("\n--- Example 1: Simple GET Request ---")
	simpleGETRequest(server.URL)

	// Example 2: GET request with query parameters
	fmt.Println("\n--- Example 2: GET with Query Parameters ---")
	getWithQueryParams(server.URL)

	// Example 3: POST request with JSON body
	fmt.Println("\n--- Example 3: POST with JSON Body ---")
	postWithJSONBody(server.URL)

	// Example 4: Using a configured client
	fmt.Println("\n--- Example 4: Using Configured Client ---")
	usingConfiguredClient(server.URL)

	// Example 5: Custom headers and authentication
	fmt.Println("\n--- Example 5: Custom Headers and Authentication ---")
	customHeadersAndAuth(server.URL)

	fmt.Println("\n=== Basic Examples Complete ===")
}

// simpleGETRequest demonstrates the simplest way to make a GET request
func simpleGETRequest(baseURL string) {
	// Using the generic GET function with automatic JSON unmarshaling
	response, err := httpx.GET[map[string]any](
		httpx.WithBaseURL(baseURL),
		httpx.WithPath("/api/users/1"),
	)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Response: %+v\n", response.Body)
}

// getWithQueryParams shows how to add query parameters to requests
func getWithQueryParams(baseURL string) {
	// GET request with multiple query parameters
	response, err := httpx.GET[[]map[string]any](
		httpx.WithBaseURL(baseURL),
		httpx.WithPath("/api/users"),
		httpx.WithQueryParam("page", "1"),
		httpx.WithQueryParam("limit", "5"),
		httpx.WithQueryParam("active", "true"),
	)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	if users, ok := response.Body.([]map[string]any); ok {
		fmt.Printf("Found %d users\n", len(users))
	} else {
		fmt.Printf("Response: %+v\n", response.Body)
	}
}

// postWithJSONBody demonstrates posting JSON data
func postWithJSONBody(baseURL string) {
	// Create a new user
	newUser := User{
		Name:  "Alice Johnson",
		Email: "alice@example.com",
	}

	// POST request with JSON body using type-safe responses
	response, err := httpx.POST[User](
		httpx.WithBaseURL(baseURL),
		httpx.WithPath("/api/users"),
		httpx.WithJSONBody(newUser),
	)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Created user: %+v\n", response.Body)
}

// usingConfiguredClient shows how to create and use a configured client
func usingConfiguredClient(baseURL string) {
	// Create a client with default configuration
	client := httpx.NewClient(
		httpx.WithDefaultBaseURL(baseURL),
		httpx.WithDefaultTimeout(5*time.Second),
		httpx.WithDefaultHeader("User-Agent", "MyApp/1.0"),
		httpx.WithDefaultHeader("Accept", "application/json"),
	)

	// Now all requests will use these defaults
	request := httpx.NewRequest(http.MethodGet,
		httpx.WithPath("/api/posts/1"),
	)

	response, err := client.Execute(*request, Post{})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	if post, ok := response.Body.(Post); ok {
		fmt.Printf("Post title: %s\n", post.Title)
	}
}

// customHeadersAndAuth demonstrates adding custom headers and basic authentication
func customHeadersAndAuth(baseURL string) {
	// Request with custom headers and basic authentication
	response, err := httpx.GET[map[string]any](
		httpx.WithBaseURL(baseURL),
		httpx.WithPath("/api/secure"),
		httpx.WithHeader("X-API-Version", "v1"),
		httpx.WithHeader("X-Client-ID", "example-client"),
		httpx.WithBasicAuth("admin", "secret"),
	)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Secure data: %+v\n", response.Body)
}

// createTestServer creates a simple HTTP server for demonstration
func createTestServer() *httptest.Server {
	mux := http.NewServeMux()

	// GET /api/users/1 - Single user
	mux.HandleFunc("/api/users/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1, "name": "John Doe", "email": "john@example.com"}`))
	})

	// GET/POST /api/users - List or create users
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			// GET - List users with query parameters
			page := r.URL.Query().Get("page")
			limit := r.URL.Query().Get("limit")
			active := r.URL.Query().Get("active")

			w.WriteHeader(http.StatusOK)

			// Simple response based on query parameters
			response := `[
				{"id": 1, "name": "John Doe", "email": "john@example.com"},
				{"id": 2, "name": "Jane Smith", "email": "jane@example.com"}
			]`

			if page == "1" && limit == "5" && active == "true" {
				fmt.Fprintf(w, "Filtered results (page=%s, limit=%s, active=%s): %s", page, limit, active, response)
			} else {
				_, _ = w.Write([]byte(response))
			}

		case http.MethodPost:
			// POST - Create user
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": 3, "name": "Alice Johnson", "email": "alice@example.com"}`))

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// GET /api/posts/1 - Single post
	mux.HandleFunc("/api/posts/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1, "title": "Hello World", "body": "This is my first post", "userId": 1}`))
	})

	// GET /api/secure - Secure endpoint requiring auth
	mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		// Check basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "Unauthorized"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Access granted", "data": "sensitive information"}`))
	})

	return httptest.NewServer(mux)
}
