package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

const (
	methodGet  = "GET"
	methodPost = "POST"
)

// User represents a user in our system
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserService demonstrates a service that uses httpx for API calls
type UserService struct {
	client  *httpx.Client
	baseURL string
}

// NewUserService creates a new user service
func NewUserService(baseURL string) *UserService {
	client := httpx.NewClient(
		httpx.WithDefaultBaseURL(baseURL),
		httpx.WithDefaultTimeout(30*time.Second),
		httpx.WithDefaultHeader("Content-Type", "application/json"),
	)

	return &UserService{
		client:  client,
		baseURL: baseURL,
	}
}

// GetUser fetches a user by ID
func (s *UserService) GetUser(id int) (*User, error) {
	req := httpx.NewRequest(http.MethodGet,
		httpx.WithPath(fmt.Sprintf("/users/%d", id)),
	)

	response, err := s.client.Execute(*req, User{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user %d: %w", id, err)
	}

	if user, ok := response.Body.(User); ok {
		return &user, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

// CreateUser creates a new user
func (s *UserService) CreateUser(user User) (*User, error) {
	req := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/users"),
		httpx.WithJSONBody(user),
	)

	response, err := s.client.Execute(*req, User{})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if createdUser, ok := response.Body.(User); ok {
		return &createdUser, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(id int, user User) (*User, error) {
	req := httpx.NewRequest(http.MethodPut,
		httpx.WithPath(fmt.Sprintf("/users/%d", id)),
		httpx.WithJSONBody(user),
	)

	response, err := s.client.Execute(*req, User{})
	if err != nil {
		return nil, fmt.Errorf("failed to update user %d: %w", id, err)
	}

	if updatedUser, ok := response.Body.(User); ok {
		return &updatedUser, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

// DeleteUser deletes a user by ID
func (s *UserService) DeleteUser(id int) error {
	req := httpx.NewRequest(http.MethodDelete,
		httpx.WithPath(fmt.Sprintf("/users/%d", id)),
	)

	_, err := s.client.Execute(*req, map[string]any{})
	if err != nil {
		return fmt.Errorf("failed to delete user %d: %w", id, err)
	}

	return nil
}

func main() {
	fmt.Println("=== Testing and Mocking Example ===")

	// Example 1: Basic service testing with mock server
	fmt.Println("\n--- Example 1: Basic Service Testing ---")
	basicServiceTesting()

	// Example 2: Testing error scenarios
	fmt.Println("\n--- Example 2: Error Scenario Testing ---")
	errorScenarioTesting()

	// Example 3: Testing different HTTP methods
	fmt.Println("\n--- Example 3: HTTP Methods Testing ---")
	httpMethodsTesting()

	// Example 4: Testing with different response codes
	fmt.Println("\n--- Example 4: Response Codes Testing ---")
	responseCodesTesting()

	// Example 5: Integration testing patterns
	fmt.Println("\n--- Example 5: Integration Testing Patterns ---")
	integrationTestingPatterns()

	fmt.Println("\n=== Testing Examples Complete ===")
}

// basicServiceTesting demonstrates basic service testing with httptest
func basicServiceTesting() {
	// Create a mock server that simulates a user API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == methodGet && r.URL.Path == "/users/1":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 1, "name": "John Doe", "email": "john@example.com"}`))

		case r.Method == methodPost && r.URL.Path == "/users":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": 2, "name": "Jane Smith", "email": "jane@example.com"}`))

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not found"}`))
		}
	}))
	defer mockServer.Close()

	// Create service with mock server URL
	userService := NewUserService(mockServer.URL)

	// Test GetUser
	user, err := userService.GetUser(1)
	if err != nil {
		fmt.Printf("  GetUser failed: %v\n", err)
	} else {
		fmt.Printf("  GetUser success: %+v\n", *user)
	}

	// Test CreateUser
	newUser := User{Name: "Jane Smith", Email: "jane@example.com"}
	createdUser, err := userService.CreateUser(newUser)
	if err != nil {
		fmt.Printf("  CreateUser failed: %v\n", err)
	} else {
		fmt.Printf("  CreateUser success: %+v\n", *createdUser)
	}
}

// errorScenarioTesting demonstrates testing various error scenarios
func errorScenarioTesting() {
	// Create a mock server that simulates different error conditions
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/404":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "user not found"}`))

		case "/users/500":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))

		case "/users/timeout":
			// Simulate timeout by not responding
			select {}

		case "/users/bad-json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid json`))

		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "bad request"}`))
		}
	}))
	defer mockServer.Close()

	userService := NewUserService(mockServer.URL)

	// Test 404 error
	fmt.Println("  Testing 404 error:")
	_, err := userService.GetUser(404)
	if err != nil {
		fmt.Printf("    Expected error: %v\n", err)
	}

	// Test 500 error
	fmt.Println("  Testing 500 error:")
	_, err = userService.GetUser(500)
	if err != nil {
		fmt.Printf("    Expected error: %v\n", err)
	}

	// Test network error (non-existent server)
	fmt.Println("  Testing network error:")
	networkErrorService := NewUserService("http://localhost:1") // Non-existent server
	_, err = networkErrorService.GetUser(1)
	if err != nil {
		fmt.Printf("    Expected network error: %v\n", err)
	}
}

// httpMethodsTesting demonstrates testing different HTTP methods
func httpMethodsTesting() {
	var lastMethod string
	var lastPath string
	var lastBody string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path

		// Read body for POST/PUT requests
		if r.Method == methodPost || r.Method == "PUT" {
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			lastBody = string(body)
		}

		switch r.Method {
		case methodGet:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 1, "name": "Test User", "email": "test@example.com"}`))

		case methodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": 2, "name": "Created User", "email": "created@example.com"}`))

		case "PUT":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 1, "name": "Updated User", "email": "updated@example.com"}`))

		case "DELETE":
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer mockServer.Close()

	userService := NewUserService(mockServer.URL)

	// Test GET
	fmt.Println("  Testing GET:")
	user, err := userService.GetUser(1)
	if err != nil {
		fmt.Printf("    Error: %v\n", err)
	} else {
		fmt.Printf("    Method: %s, Path: %s, User: %+v\n", lastMethod, lastPath, *user)
	}

	// Test POST
	fmt.Println("  Testing POST:")
	newUser := User{Name: "New User", Email: "new@example.com"}
	createdUser, err := userService.CreateUser(newUser)
	if err != nil {
		fmt.Printf("    Error: %v\n", err)
	} else {
		fmt.Printf("    Method: %s, Path: %s, Body: %s\n", lastMethod, lastPath, lastBody)
		fmt.Printf("    Created: %+v\n", *createdUser)
	}

	// Test PUT
	fmt.Println("  Testing PUT:")
	updateUser := User{Name: "Updated User", Email: "updated@example.com"}
	updated, err := userService.UpdateUser(1, updateUser)
	if err != nil {
		fmt.Printf("    Error: %v\n", err)
	} else {
		fmt.Printf("    Method: %s, Path: %s, Body: %s\n", lastMethod, lastPath, lastBody)
		fmt.Printf("    Updated: %+v\n", *updated)
	}

	// Test DELETE
	fmt.Println("  Testing DELETE:")
	err = userService.DeleteUser(1)
	if err != nil {
		fmt.Printf("    Error: %v\n", err)
	} else {
		fmt.Printf("    Method: %s, Path: %s - Success\n", lastMethod, lastPath)
	}
}

// responseCodesTesting demonstrates testing different HTTP response codes
func responseCodesTesting() {
	testCases := []struct {
		code        int
		path        string
		description string
	}{
		{200, "/users/200", "Success"},
		{201, "/users/201", "Created"},
		{400, "/users/400", "Bad Request"},
		{401, "/users/401", "Unauthorized"},
		{403, "/users/403", "Forbidden"},
		{404, "/users/404", "Not Found"},
		{500, "/users/500", "Internal Server Error"},
		{503, "/users/503", "Service Unavailable"},
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract expected status code from path
		for _, tc := range testCases {
			if strings.Contains(r.URL.Path, fmt.Sprintf("/%d", tc.code)) {
				w.WriteHeader(tc.code)
				if tc.code >= 200 && tc.code < 300 {
					_, _ = w.Write([]byte(`{"id": 1, "name": "Test User", "email": "test@example.com"}`))
				} else {
					fmt.Fprintf(w, `{"error": "%s", "code": %d}`, tc.description, tc.code)
				}
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	client := httpx.NewClient(httpx.WithDefaultBaseURL(mockServer.URL))

	for _, tc := range testCases {
		fmt.Printf("  Testing %d (%s):\n", tc.code, tc.description)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath(tc.path))
		response, err := client.Execute(*req, map[string]any{})

		if err != nil {
			fmt.Printf("    Error: %v\n", err)
		} else {
			fmt.Printf("    Status: %d, Body: %+v\n", response.StatusCode, response.Body)
		}
	}
}

// integrationTestingPatterns demonstrates patterns for integration testing
func integrationTestingPatterns() {
	fmt.Println("  Integration Testing Patterns:")

	// Pattern 1: Test server lifecycle management
	fmt.Println("\n    Pattern 1: Server Lifecycle Management")
	server := createTestServer()
	defer server.Close() // Always clean up

	service := NewUserService(server.URL)
	user, err := service.GetUser(1)
	if err != nil {
		fmt.Printf("      Error: %v\n", err)
	} else {
		fmt.Printf("      Retrieved user: %+v\n", *user)
	}

	// Pattern 2: State verification
	fmt.Println("\n    Pattern 2: State Verification")
	stateServer := createStatefulTestServer()
	defer stateServer.Close()

	stateService := NewUserService(stateServer.URL)

	// Create user
	newUser := User{Name: "State Test User", Email: "state@example.com"}
	created, err := stateService.CreateUser(newUser)
	if err != nil {
		fmt.Printf("      Create error: %v\n", err)
	} else {
		fmt.Printf("      Created user: %+v\n", *created)
	}

	// Verify user exists
	retrieved, err := stateService.GetUser(created.ID)
	if err != nil {
		fmt.Printf("      Retrieve error: %v\n", err)
	} else {
		fmt.Printf("      Retrieved user: %+v\n", *retrieved)
		if retrieved.Name == created.Name {
			fmt.Printf("      ✅ State verification passed\n")
		} else {
			fmt.Printf("      ❌ State verification failed\n")
		}
	}

	// Pattern 3: Error condition testing
	fmt.Println("\n    Pattern 3: Error Condition Testing")
	errorServer := createErrorTestServer()
	defer errorServer.Close()

	errorService := NewUserService(errorServer.URL)

	// Test various error conditions
	errorConditions := []int{400, 401, 403, 404, 500, 503}
	for _, code := range errorConditions {
		_, err := errorService.GetUser(code)
		if err != nil {
			fmt.Printf("      Error %d: %v\n", code, err)
		}
	}
}

// Helper functions for creating test servers

func createTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1, "name": "Integration Test User", "email": "integration@example.com"}`))
	}))
}

func createStatefulTestServer() *httptest.Server {
	users := make(map[int]User)
	nextID := 1

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case methodGet:
			// Parse user ID from path
			var userID int
			_, _ = fmt.Sscanf(r.URL.Path, "/users/%d", &userID)

			if user, exists := users[userID]; exists {
				w.WriteHeader(http.StatusOK)
				response := fmt.Sprintf(`{"id": %d, "name": "%s", "email": "%s"}`,
					user.ID, user.Name, user.Email)
				_, _ = w.Write([]byte(response))
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error": "user not found"}`))
			}

		case methodPost:
			// Create new user
			user := User{ID: nextID, Name: "State Test User", Email: "state@example.com"}
			users[nextID] = user
			nextID++

			w.WriteHeader(http.StatusCreated)
			response := fmt.Sprintf(`{"id": %d, "name": "%s", "email": "%s"}`,
				user.ID, user.Name, user.Email)
			_, _ = w.Write([]byte(response))

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

func createErrorTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract expected error code from user ID in path
		var code int
		_, _ = fmt.Sscanf(r.URL.Path, "/users/%d", &code)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		switch code {
		case 400:
			_, _ = w.Write([]byte(`{"error": "bad request"}`))
		case 401:
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
		case 403:
			_, _ = w.Write([]byte(`{"error": "forbidden"}`))
		case 404:
			_, _ = w.Write([]byte(`{"error": "not found"}`))
		case 500:
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
		case 503:
			_, _ = w.Write([]byte(`{"error": "service unavailable"}`))
		default:
			_, _ = w.Write([]byte(`{"error": "unknown error"}`))
		}
	}))
}
