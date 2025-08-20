package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bdpiprava/easy-http/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicAuthFunctionality(t *testing.T) {
	// Setup test server that echoes back auth info
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "no auth provided"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"username": "` + username + `", "password": "` + password + `", "authenticated": true}`))
	}))
	defer server.Close()

	type AuthResponse struct {
		Username      string `json:"username,omitempty"`
		Password      string `json:"password,omitempty"`
		Authenticated bool   `json:"authenticated,omitempty"`
		Error         string `json:"error,omitempty"`
	}

	t.Run("request-level basic auth with new architecture", func(t *testing.T) {
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
		)

		req := httpx.NewRequest(http.MethodGet,
			httpx.WithPath("/protected"),
			httpx.WithBasicAuth("requestuser", "requestpass"),
		)
		response, err := client.Execute(*req, AuthResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(AuthResponse)
		require.True(t, ok)
		assert.Equal(t, "requestuser", result.Username)
		assert.Equal(t, "requestpass", result.Password)
		assert.True(t, result.Authenticated)
	})

	t.Run("request-level basic auth with old architecture", func(t *testing.T) {
		client := httpx.NewClient(
			httpx.WithDefaultBaseURL(server.URL),
		)

		req := httpx.NewRequest(http.MethodGet,
			httpx.WithPath("/protected"),
			httpx.WithBasicAuth("oldrequestuser", "oldrequestpass"),
		)
		response, err := client.Execute(*req, AuthResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(AuthResponse)
		require.True(t, ok)
		assert.Equal(t, "oldrequestuser", result.Username)
		assert.Equal(t, "oldrequestpass", result.Password)
		assert.True(t, result.Authenticated)
	})

	t.Run("client-level basic auth with new architecture", func(t *testing.T) {
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultBasicAuth("clientuser", "clientpass"),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/protected"))
		response, err := client.Execute(*req, AuthResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(AuthResponse)
		require.True(t, ok)
		assert.Equal(t, "clientuser", result.Username)
		assert.Equal(t, "clientpass", result.Password)
		assert.True(t, result.Authenticated)
	})

	t.Run("client-level basic auth with old architecture", func(t *testing.T) {
		client := httpx.NewClient(
			httpx.WithDefaultBaseURL(server.URL),
			httpx.WithDefaultBasicAuth("oldclientuser", "oldclientpass"),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/protected"))
		response, err := client.Execute(*req, AuthResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(AuthResponse)
		require.True(t, ok)
		assert.Equal(t, "oldclientuser", result.Username)
		assert.Equal(t, "oldclientpass", result.Password)
		assert.True(t, result.Authenticated)
	})

	t.Run("request-level auth overrides client-level auth", func(t *testing.T) {
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultBasicAuth("defaultuser", "defaultpass"),
		)

		req := httpx.NewRequest(http.MethodGet,
			httpx.WithPath("/protected"),
			httpx.WithBasicAuth("overrideuser", "overridepass"),
		)
		response, err := client.Execute(*req, AuthResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(AuthResponse)
		require.True(t, ok)
		assert.Equal(t, "overrideuser", result.Username)
		assert.Equal(t, "overridepass", result.Password)
		assert.True(t, result.Authenticated)
	})

	t.Run("no auth provided returns unauthorized", func(t *testing.T) {
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/protected"))
		response, err := client.Execute(*req, AuthResponse{})

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)

		// For error responses (status > 299), the body is parsed as a generic map
		errorMap, ok := response.Body.(map[string]any)
		require.True(t, ok, "Expected error response to be parsed as map[string]any")
		assert.Equal(t, "no auth provided", errorMap["error"])
	})

	t.Run("convenience functions with basic auth", func(t *testing.T) {
		response, err := httpx.GET[AuthResponse](
			httpx.WithBaseURL(server.URL),
			httpx.WithPath("/protected"),
			httpx.WithBasicAuth("convenientuser", "convenientpass"),
		)

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)

		result, ok := response.Body.(AuthResponse)
		require.True(t, ok)
		assert.Equal(t, "convenientuser", result.Username)
		assert.Equal(t, "convenientpass", result.Password)
		assert.True(t, result.Authenticated)
	})
}
