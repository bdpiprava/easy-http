package httpx_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/bdpiprava/easy-http/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorClassification(t *testing.T) {
	t.Run("network errors", func(t *testing.T) {
		// Test DNS error
		dnsErr := &net.DNSError{
			Err:        "no such host",
			Name:       "invalid-domain.com",
			IsNotFound: true,
		}
		
		req, _ := http.NewRequest("GET", "http://invalid-domain.com", nil)
		httpErr := httpx.ClassifyError(dnsErr, req, nil)
		
		assert.Equal(t, httpx.ErrorTypeNetwork, httpErr.Type)
		assert.True(t, httpx.IsNetworkError(httpErr))
		assert.Contains(t, httpErr.Error(), "network error")
		assert.Equal(t, req, httpErr.Request)
	})

	t.Run("timeout errors", func(t *testing.T) {
		timeoutErr := &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &net.DNSError{
				Err:       "operation timed out",
				IsTimeout: true,
			},
		}
		
		req, _ := http.NewRequest("GET", "http://slow-server.com", nil)
		httpErr := httpx.ClassifyError(timeoutErr, req, nil)
		
		assert.Equal(t, httpx.ErrorTypeTimeout, httpErr.Type)
		assert.True(t, httpx.IsTimeoutError(httpErr))
	})

	t.Run("context timeout errors", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		httpErr := httpx.ClassifyError(context.DeadlineExceeded, req, nil)
		
		assert.Equal(t, httpx.ErrorTypeTimeout, httpErr.Type)
		assert.True(t, httpx.IsTimeoutError(httpErr))
	})

	t.Run("client errors (4xx)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com/not-found", nil)
		resp := &http.Response{
			StatusCode: 404,
			Status:     "404 Not Found",
		}
		
		httpErr := httpx.ClassifyError(nil, req, resp)
		
		assert.Equal(t, httpx.ErrorTypeClient, httpErr.Type)
		assert.True(t, httpx.IsClientError(httpErr))
		assert.Equal(t, 404, httpErr.StatusCode)
		assert.Equal(t, 404, httpx.GetStatusCode(httpErr))
	})

	t.Run("server errors (5xx)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com/error", nil)
		resp := &http.Response{
			StatusCode: 500,
			Status:     "500 Internal Server Error",
		}
		
		httpErr := httpx.ClassifyError(nil, req, resp)
		
		assert.Equal(t, httpx.ErrorTypeServer, httpErr.Type)
		assert.True(t, httpx.IsServerError(httpErr))
		assert.Equal(t, 500, httpErr.StatusCode)
	})

	t.Run("validation errors", func(t *testing.T) {
		validationErr := httpx.ValidationError("invalid URL format", nil)
		
		assert.Equal(t, httpx.ErrorTypeValidation, validationErr.Type)
		assert.True(t, httpx.IsValidationError(validationErr))
		assert.Contains(t, validationErr.Error(), "validation error")
	})

	t.Run("middleware errors", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		middlewareErr := httpx.MiddlewareError("middleware failure", errors.New("underlying error"), req)
		
		assert.Equal(t, httpx.ErrorTypeMiddleware, middlewareErr.Type)
		assert.True(t, httpx.IsMiddlewareError(middlewareErr))
		assert.Equal(t, req, middlewareErr.Request)
	})
}

func TestErrorWrapping(t *testing.T) {
	t.Run("error unwrapping", func(t *testing.T) {
		originalErr := errors.New("original error")
		httpErr := httpx.NetworkError("network failure", originalErr, nil)
		
		assert.True(t, errors.Is(httpErr, originalErr))
		assert.Equal(t, originalErr, errors.Unwrap(httpErr))
	})

	t.Run("error equality", func(t *testing.T) {
		err1 := httpx.ClientError("client error", nil, &http.Response{StatusCode: 400})
		err2 := httpx.ClientError("another client error", nil, &http.Response{StatusCode: 400})
		err3 := httpx.ServerError("server error", nil, &http.Response{StatusCode: 500})
		
		assert.True(t, errors.Is(err1, err2))
		assert.False(t, errors.Is(err1, err3))
	})
}

func TestErrorContext(t *testing.T) {
	t.Run("request context preservation", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "test-key", "test-value")
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
		
		httpErr := httpx.NetworkError("network error", nil, req)
		
		extractedCtx := httpx.GetRequestContext(httpErr)
		require.NotNil(t, extractedCtx)
		assert.Equal(t, "test-value", extractedCtx.Value("test-key"))
	})

	t.Run("nil context handling", func(t *testing.T) {
		httpErr := httpx.ValidationError("validation error", nil)
		extractedCtx := httpx.GetRequestContext(httpErr)
		assert.Nil(t, extractedCtx)
	})
}

func TestIntegrationWithHTTPClient(t *testing.T) {
	t.Run("network error integration", func(t *testing.T) {
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL("http://invalid-domain-that-does-not-exist.com"),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		_, err := client.Execute(*req, map[string]any{})

		require.Error(t, err)
		assert.True(t, httpx.IsNetworkError(err), "Expected network error, got: %T", err)
		
		if httpErr, ok := err.(*httpx.HTTPError); ok {
			assert.NotNil(t, httpErr.Request)
			assert.Contains(t, httpErr.Request.URL.String(), "invalid-domain-that-does-not-exist.com")
		}
	})

	t.Run("client error integration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "bad request"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/bad-request"))
		response, err := client.Execute(*req, map[string]any{})

		// For 4xx status codes, the request succeeds but returns error response
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("server error integration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/server-error"))
		response, err := client.Execute(*req, map[string]any{})

		// For 5xx status codes, the request succeeds but returns error response
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})

	t.Run("validation error integration", func(t *testing.T) {
		client := httpx.NewClientWithConfig()

		req := httpx.NewRequest(http.MethodGet, 
			httpx.WithBaseURL("invalid-url-without-scheme"),
			httpx.WithPath("/test"),
		)
		_, err := client.Execute(*req, map[string]any{})

		require.Error(t, err)
		assert.True(t, httpx.IsValidationError(err), "Expected validation error, got: %T", err)
	})
}

func TestTimeoutErrorDetection(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name: "net timeout error",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: &net.DNSError{IsTimeout: true},
			},
			expected: true,
		},
		{
			name: "syscall timeout",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: syscall.ETIMEDOUT,
			},
			expected: true, // syscall.ETIMEDOUT should be detected as timeout
		},
		{
			name:     "timeout in error message",
			err:      errors.New("operation timeout"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := httpx.ClassifyError(tt.err, nil, nil)
			isTimeout := httpErr.Type == httpx.ErrorTypeTimeout
			assert.Equal(t, tt.expected, isTimeout)
		})
	}
}

func TestNetworkErrorDetection(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "DNS error",
			err:      &net.DNSError{Err: "no such host"},
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      errors.New("no such host"),
			expected: true,
		},
		{
			name:     "dial tcp error",
			err:      errors.New("dial tcp: connection failed"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := httpx.ClassifyError(tt.err, nil, nil)
			isNetwork := httpErr.Type == httpx.ErrorTypeNetwork
			assert.Equal(t, tt.expected, isNetwork)
		})
	}
}

func TestErrorHelperFunctions(t *testing.T) {
	t.Run("GetStatusCode", func(t *testing.T) {
		httpErr := httpx.ClientError("client error", nil, &http.Response{StatusCode: 404})
		assert.Equal(t, 404, httpx.GetStatusCode(httpErr))
		
		regularErr := errors.New("regular error")
		assert.Equal(t, 0, httpx.GetStatusCode(regularErr))
	})

	t.Run("GetRequestContext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "key", "value")
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
		httpErr := httpx.NetworkError("network error", nil, req)
		
		extractedCtx := httpx.GetRequestContext(httpErr)
		require.NotNil(t, extractedCtx)
		assert.Equal(t, "value", extractedCtx.Value("key"))
		
		regularErr := errors.New("regular error")
		assert.Nil(t, httpx.GetRequestContext(regularErr))
	})
}