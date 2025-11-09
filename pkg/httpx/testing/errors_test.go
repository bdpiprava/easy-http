package testing_test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"
)

func TestErrorSimulator_Timeout(t *testing.T) {
	t.Parallel()

	t.Run("delays response by specified duration", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		delay := 100 * time.Millisecond
		subject.OnGet("/timeout").
			SimulateError().
			Timeout(delay)

		start := time.Now()
		resp, err := http.Get(subject.URL() + "/timeout")
		elapsed := time.Since(start)

		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, elapsed >= delay, "Expected delay of at least %v, got %v", delay, elapsed)
	})
}

func TestErrorSimulator_NetworkError(t *testing.T) {
	t.Parallel()

	t.Run("returns 502 Bad Gateway to simulate network error", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/network-error").
			SimulateError().
			NetworkError()

		resp, err := http.Get(subject.URL() + "/network-error")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		assert.Equal(t, "simulated network error", string(body))
	})
}

func TestErrorSimulator_InternalServerError(t *testing.T) {
	t.Parallel()

	t.Run("returns 500 Internal Server Error with JSON body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/internal-error").
			SimulateError().
			InternalServerError()

		resp, err := http.Get(subject.URL() + "/internal-error")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Contains(t, string(body), "internal server error")
		assert.Contains(t, string(body), "simulated server failure")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})
}

func TestErrorSimulator_BadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		message        string
		wantMessage    string
		wantStatusCode int
	}{
		{
			name:           "returns 400 Bad Request with custom message",
			message:        "invalid input provided",
			wantMessage:    "invalid input provided",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "returns default message when empty",
			message:        "",
			wantMessage:    "bad request",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/bad-request").
				SimulateError().
				BadRequest(tc.message)

			resp, err := http.Get(subject.URL() + "/bad-request")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantMessage)
			assert.Contains(t, string(body), "bad_request")
		})
	}
}

func TestErrorSimulator_Unauthorized(t *testing.T) {
	t.Parallel()

	t.Run("returns 401 Unauthorized with JSON body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/unauthorized").
			SimulateError().
			Unauthorized()

		resp, err := http.Get(subject.URL() + "/unauthorized")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, string(body), "unauthorized")
		assert.Contains(t, string(body), "authentication required")
	})
}

func TestErrorSimulator_Forbidden(t *testing.T) {
	t.Parallel()

	t.Run("returns 403 Forbidden with JSON body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/forbidden").
			SimulateError().
			Forbidden()

		resp, err := http.Get(subject.URL() + "/forbidden")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, string(body), "forbidden")
		assert.Contains(t, string(body), "access denied")
	})
}

func TestErrorSimulator_NotFound(t *testing.T) {
	t.Parallel()

	t.Run("returns 404 Not Found with JSON body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/not-found").
			SimulateError().
			NotFound()

		resp, err := http.Get(subject.URL() + "/not-found")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Contains(t, string(body), "not_found")
		assert.Contains(t, string(body), "resource not found")
	})
}

func TestErrorSimulator_Conflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		message        string
		wantMessage    string
		wantStatusCode int
	}{
		{
			name:           "returns 409 Conflict with custom message",
			message:        "resource already exists",
			wantMessage:    "resource already exists",
			wantStatusCode: http.StatusConflict,
		},
		{
			name:           "returns default message when empty",
			message:        "",
			wantMessage:    "resource conflict",
			wantStatusCode: http.StatusConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/conflict").
				SimulateError().
				Conflict(tc.message)

			resp, err := http.Get(subject.URL() + "/conflict")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantMessage)
			assert.Contains(t, string(body), "conflict")
		})
	}
}

func TestErrorSimulator_TooManyRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		retryAfter        int
		wantStatusCode    int
		wantRetryAfterSet bool
	}{
		{
			name:              "returns 429 with Retry-After header",
			retryAfter:        60,
			wantStatusCode:    http.StatusTooManyRequests,
			wantRetryAfterSet: true,
		},
		{
			name:              "returns 429 without Retry-After when zero",
			retryAfter:        0,
			wantStatusCode:    http.StatusTooManyRequests,
			wantRetryAfterSet: false,
		},
		{
			name:              "returns 429 without Retry-After when negative",
			retryAfter:        -1,
			wantStatusCode:    http.StatusTooManyRequests,
			wantRetryAfterSet: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/rate-limit").
				SimulateError().
				TooManyRequests(tc.retryAfter)

			resp, err := http.Get(subject.URL() + "/rate-limit")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Contains(t, string(body), "rate_limit_exceeded")
			assert.Contains(t, string(body), "too many requests")

			if tc.wantRetryAfterSet {
				assert.NotEmpty(t, resp.Header.Get("Retry-After"))
			} else {
				assert.Empty(t, resp.Header.Get("Retry-After"))
			}
		})
	}
}

func TestErrorSimulator_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		retryAfter        int
		wantStatusCode    int
		wantRetryAfterSet bool
	}{
		{
			name:              "returns 503 with Retry-After header",
			retryAfter:        120,
			wantStatusCode:    http.StatusServiceUnavailable,
			wantRetryAfterSet: true,
		},
		{
			name:              "returns 503 without Retry-After when zero",
			retryAfter:        0,
			wantStatusCode:    http.StatusServiceUnavailable,
			wantRetryAfterSet: false,
		},
		{
			name:              "returns 503 without Retry-After when negative",
			retryAfter:        -5,
			wantStatusCode:    http.StatusServiceUnavailable,
			wantRetryAfterSet: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/unavailable").
				SimulateError().
				ServiceUnavailable(tc.retryAfter)

			resp, err := http.Get(subject.URL() + "/unavailable")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Contains(t, string(body), "service_unavailable")
			assert.Contains(t, string(body), "service temporarily unavailable")

			if tc.wantRetryAfterSet {
				assert.NotEmpty(t, resp.Header.Get("Retry-After"))
			} else {
				assert.Empty(t, resp.Header.Get("Retry-After"))
			}
		})
	}
}

func TestErrorSimulator_GatewayTimeout(t *testing.T) {
	t.Parallel()

	t.Run("returns 504 Gateway Timeout with JSON body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/gateway-timeout").
			SimulateError().
			GatewayTimeout()

		resp, err := http.Get(subject.URL() + "/gateway-timeout")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
		assert.Contains(t, string(body), "gateway_timeout")
		assert.Contains(t, string(body), "upstream request timed out")
	})
}

func TestErrorSimulator_CustomError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		statusCode     int
		message        string
		wantStatusCode int
		wantMessage    string
	}{
		{
			name:           "returns custom status code with message",
			statusCode:     http.StatusTeapot,
			message:        "I'm a teapot",
			wantStatusCode: http.StatusTeapot,
			wantMessage:    "I'm a teapot",
		},
		{
			name:           "returns custom 422 Unprocessable Entity",
			statusCode:     http.StatusUnprocessableEntity,
			message:        "validation failed",
			wantStatusCode: http.StatusUnprocessableEntity,
			wantMessage:    "validation failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/custom-error").
				SimulateError().
				CustomError(tc.statusCode, tc.message)

			resp, err := http.Get(subject.URL() + "/custom-error")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantMessage)
		})
	}
}

func TestErrorSimulator_Slow(t *testing.T) {
	t.Parallel()

	t.Run("delays response by specified duration", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		delay := 100 * time.Millisecond
		subject.OnGet("/slow").
			SimulateError().
			Slow(delay)

		start := time.Now()
		resp, err := http.Get(subject.URL() + "/slow")
		elapsed := time.Since(start)

		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, elapsed >= delay, "Expected delay of at least %v, got %v", delay, elapsed)
	})
}

func TestErrorSimulator_FluentChaining(t *testing.T) {
	t.Parallel()

	t.Run("allows chaining error simulation with response configuration", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/chained").
			SimulateError().
			Unauthorized().
			WithHeader("X-Custom-Header", "test-value")

		resp, err := http.Get(subject.URL() + "/chained")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "test-value", resp.Header.Get("X-Custom-Header"))
	})
}

func TestMockServer_OnFlaky(t *testing.T) {
	t.Parallel()

	t.Run("creates flaky response configuration", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		flaky := subject.OnFlaky("/flaky", 3)

		assert.NotNil(t, flaky)
	})
}

func TestFlakyResponse_WithPattern(t *testing.T) {
	t.Parallel()

	t.Run("configures flaky pattern", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		flaky := subject.OnFlaky("/flaky", 3).
			WithPattern(2, 1)

		assert.NotNil(t, flaky)
	})
}

func TestFlakyResponse_Configure(t *testing.T) {
	t.Parallel()

	t.Run("configures flaky behavior on mock server", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		flaky := subject.OnFlaky("/flaky", 2).
			WithPattern(3, 2)

		// Call Configure to set up the behavior
		flaky.Configure()

		// Should be able to make request to the path
		resp, err := http.Get(subject.URL() + "/flaky")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "configured flaky response", string(body))
	})
}

func TestErrorSimulator_MultipleErrors(t *testing.T) {
	t.Parallel()

	t.Run("can configure different error types for different paths", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/error1").SimulateError().NotFound()
		subject.OnGet("/error2").SimulateError().Unauthorized()
		subject.OnGet("/error3").SimulateError().InternalServerError()

		// Test error1
		resp1, err := http.Get(subject.URL() + "/error1")
		require.NoError(t, err)
		defer resp1.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp1.StatusCode)

		// Test error2
		resp2, err := http.Get(subject.URL() + "/error2")
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)

		// Test error3
		resp3, err := http.Get(subject.URL() + "/error3")
		require.NoError(t, err)
		defer resp3.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, resp3.StatusCode)
	})
}

func TestErrorSimulator_CombinedWithNormalResponses(t *testing.T) {
	t.Parallel()

	t.Run("can mix error simulation with normal responses", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		// Normal successful response
		subject.OnGet("/success").
			WithStatus(http.StatusOK).
			WithBodyString("success")

		// Error response
		subject.OnGet("/error").
			SimulateError().
			InternalServerError()

		// Test success
		resp1, err := http.Get(subject.URL() + "/success")
		require.NoError(t, err)
		defer resp1.Body.Close()
		body1, _ := io.ReadAll(resp1.Body)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		assert.Equal(t, "success", string(body1))

		// Test error
		resp2, err := http.Get(subject.URL() + "/error")
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, resp2.StatusCode)
	})
}

func TestResponseBuilder_SimulateError(t *testing.T) {
	t.Parallel()

	t.Run("returns error simulator from response builder", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		simulator := subject.OnGet("/test").SimulateError()

		assert.NotNil(t, simulator)
	})
}

func TestErrorSimulator_WithDelayAndError(t *testing.T) {
	t.Parallel()

	t.Run("slow error response takes time", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		delay := 50 * time.Millisecond
		subject.OnGet("/slow-error").
			SimulateError().
			Slow(delay)

		start := time.Now()
		resp, err := http.Get(subject.URL() + "/slow-error")
		elapsed := time.Since(start)

		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, elapsed >= delay)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestErrorSimulator_JSONResponseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupError     func(*httpxtesting.MockServer)
		wantStatusCode int
		wantFields     []string
	}{
		{
			name: "InternalServerError returns properly formatted JSON",
			setupError: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").SimulateError().InternalServerError()
			},
			wantStatusCode: http.StatusInternalServerError,
			wantFields:     []string{"error", "message"},
		},
		{
			name: "BadRequest returns properly formatted JSON",
			setupError: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").SimulateError().BadRequest("test error")
			},
			wantStatusCode: http.StatusBadRequest,
			wantFields:     []string{"error", "message"},
		},
		{
			name: "TooManyRequests returns properly formatted JSON",
			setupError: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").SimulateError().TooManyRequests(60)
			},
			wantStatusCode: http.StatusTooManyRequests,
			wantFields:     []string{"error", "message"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.setupError(subject)

			resp, err := http.Get(subject.URL() + "/test")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			bodyStr := string(body)
			for _, field := range tc.wantFields {
				assert.Contains(t, bodyStr, field)
			}
		})
	}
}
