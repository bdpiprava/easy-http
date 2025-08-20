package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientConfig(t *testing.T) {
	tests := []struct {
		name   string
		opts   []httpx.ClientConfigOption
		verify func(t *testing.T, client *httpx.Client)
	}{
		{
			name: "default configuration",
			opts: nil,
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
				// Default values are set internally and not exposed
			},
		},
		{
			name: "with timeout configuration",
			opts: []httpx.ClientConfigOption{
				httpx.WithClientTimeout(30 * time.Second),
			},
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with logger configuration",
			opts: []httpx.ClientConfigOption{
				httpx.WithClientLogger(slog.Default()),
				httpx.WithClientLogLevel(slog.LevelDebug),
			},
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with base URL configuration",
			opts: []httpx.ClientConfigOption{
				httpx.WithClientDefaultBaseURL("https://api.example.com"),
			},
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with default headers configuration",
			opts: []httpx.ClientConfigOption{
				httpx.WithClientDefaultHeader("User-Agent", "test-client/1.0"),
				httpx.WithClientDefaultHeaders(http.Header{
					"Authorization": []string{"Bearer token123"},
				}),
			},
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "with basic auth configuration",
			opts: []httpx.ClientConfigOption{
				httpx.WithClientDefaultBasicAuth("user", "pass"),
			},
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "complete configuration",
			opts: []httpx.ClientConfigOption{
				httpx.WithClientTimeout(45 * time.Second),
				httpx.WithClientLogger(slog.Default()),
				httpx.WithClientLogLevel(slog.LevelWarn),
				httpx.WithClientDefaultBaseURL("https://api.test.com"),
				httpx.WithClientDefaultHeader("Content-Type", "application/json"),
				httpx.WithClientDefaultBasicAuth("testuser", "testpass"),
			},
			verify: func(t *testing.T, client *httpx.Client) {
				assert.NotNil(t, client)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := httpx.NewClientWithConfig(tt.opts...)
			tt.verify(t, client)
		})
	}
}

func TestClientConfigArchitectureIntegration(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check basic auth
		username, password, ok := r.BasicAuth()
		if ok {
			w.Header().Set("X-Auth-User", username)
			w.Header().Set("X-Auth-Pass", password)
		}

		// Check headers
		if ua := r.Header.Get("User-Agent"); ua != "" {
			w.Header().Set("X-User-Agent", ua)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success", "path": "` + r.URL.Path + `"}`))
	}))
	defer server.Close()

	// Test new architecture
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := httpx.NewClientWithConfig(
		httpx.WithClientTimeout(10*time.Second),
		httpx.WithClientLogger(logger),
		httpx.WithClientLogLevel(slog.LevelDebug),
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultHeader("User-Agent", "new-arch-test/1.0"),
		httpx.WithClientDefaultBasicAuth("testuser", "testpass"),
	)

	type TestResponse struct {
		Message string `json:"message"`
		Path    string `json:"path"`
	}

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
	response, err := client.Execute(*req, TestResponse{})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, http.StatusOK, response.StatusCode)

	result, ok := response.Body.(TestResponse)
	require.True(t, ok, "response body should be of type TestResponse")
	assert.Equal(t, "success", result.Message)
	assert.Equal(t, "/test", result.Path)

	// Verify logs were written
	assert.Contains(t, logBuffer.String(), "HTTP request")
	assert.Contains(t, logBuffer.String(), "HTTP response")
}

func TestBackwardCompatibilityArchitecture(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check basic auth
		username, password, ok := r.BasicAuth()
		if ok {
			w.Header().Set("X-Auth-User", username)
			w.Header().Set("X-Auth-Pass", password)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "legacy", "method": "` + r.Method + `"}`))
	}))
	defer server.Close()

	// Test old architecture still works
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := httpx.NewClient(
		httpx.WithDefaultTimeout(10*time.Second),
		httpx.WithLogger(logger),
		httpx.WithLogLevel(slog.LevelDebug),
		httpx.WithDefaultBaseURL(server.URL),
		httpx.WithDefaultHeader("User-Agent", "legacy-test/1.0"),
	)

	type TestResponse struct {
		Message string `json:"message"`
		Method  string `json:"method"`
	}

	req := httpx.NewRequest(http.MethodPost, httpx.WithPath("/legacy"))
	response, err := client.Execute(*req, TestResponse{})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, http.StatusOK, response.StatusCode)

	result, ok := response.Body.(TestResponse)
	require.True(t, ok, "response body should be of type TestResponse")
	assert.Equal(t, "legacy", result.Message)
	assert.Equal(t, "POST", result.Method)

	// Verify logs were written
	assert.Contains(t, logBuffer.String(), "HTTP request")
	assert.Contains(t, logBuffer.String(), "HTTP response")
}

func TestBasicAuthSupport(t *testing.T) {
	// Setup test server that requires basic auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		_ = password // Use password variable to avoid unused variable error
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"user": "` + username + `", "authenticated": true}`))
	}))
	defer server.Close()

	tests := []struct {
		name           string
		setupClient    func() *httpx.Client
		expectedStatus int
		expectedUser   string
	}{
		{
			name: "new architecture with basic auth",
			setupClient: func() *httpx.Client {
				return httpx.NewClientWithConfig(
					httpx.WithClientDefaultBaseURL(server.URL),
					httpx.WithClientDefaultBasicAuth("newuser", "newpass"),
				)
			},
			expectedStatus: http.StatusOK,
			expectedUser:   "newuser",
		},
		{
			name: "old architecture with basic auth",
			setupClient: func() *httpx.Client {
				return httpx.NewClient(
					httpx.WithDefaultBaseURL(server.URL),
					httpx.WithDefaultBasicAuth("olduser", "oldpass"),
				)
			},
			expectedStatus: http.StatusOK,
			expectedUser:   "olduser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()

			type AuthResponse struct {
				User          string `json:"user,omitempty"`
				Authenticated bool   `json:"authenticated,omitempty"`
				Error         string `json:"error,omitempty"`
			}

			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/protected"))
			response, err := client.Execute(*req, AuthResponse{})

			require.NoError(t, err)
			require.NotNil(t, response)
			assert.Equal(t, tt.expectedStatus, response.StatusCode)

			if tt.expectedStatus == http.StatusOK {
				result, ok := response.Body.(AuthResponse)
				require.True(t, ok, "response body should be of type AuthResponse")
				assert.Equal(t, tt.expectedUser, result.User)
				assert.True(t, result.Authenticated)
			}
		})
	}
}
