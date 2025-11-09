package testing_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"
)

func TestNewMockServer(t *testing.T) {
	t.Parallel()

	t.Run("creates functioning mock server", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		assert.NotNil(t, subject)
		assert.NotEmpty(t, subject.URL())
		assert.True(t, strings.HasPrefix(subject.URL(), "http://"))
	})
}

func TestNewMockTLSServer(t *testing.T) {
	t.Parallel()

	t.Run("creates functioning HTTPS mock server", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockTLSServer()
		defer subject.Close()

		assert.NotNil(t, subject)
		assert.NotEmpty(t, subject.URL())
		assert.True(t, strings.HasPrefix(subject.URL(), "https://"))
	})
}

func TestMockServer_OnGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		setupMock        func(*httpxtesting.MockServer)
		requestPath      string
		wantStatusCode   int
		wantResponseBody string
	}{
		{
			name: "returns configured response for GET request",
			setupMock: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/users").
					WithStatus(http.StatusOK).
					WithBodyString("user list")
			},
			requestPath:      "/users",
			wantStatusCode:   http.StatusOK,
			wantResponseBody: "user list",
		},
		{
			name: "returns JSON response for GET request",
			setupMock: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/api/data").
					WithStatus(http.StatusOK).
					WithJSON(map[string]interface{}{
						"id":   123,
						"name": "Alice",
					})
			},
			requestPath:      "/api/data",
			wantStatusCode:   http.StatusOK,
			wantResponseBody: `{"id":123,"name":"Alice"}`,
		},
		{
			name: "returns 404 for unmatched path",
			setupMock: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/existing").
					WithStatus(http.StatusOK).
					WithBodyString("ok")
			},
			requestPath:      "/nonexistent",
			wantStatusCode:   http.StatusNotFound,
			wantResponseBody: "404 page not found\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.setupMock(subject)

			resp, err := http.Get(subject.URL() + tc.requestPath)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			assert.Equal(t, tc.wantResponseBody, string(body))
		})
	}
}

func TestMockServer_OnPost(t *testing.T) {
	t.Parallel()

	t.Run("returns configured response for POST request", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnPost("/users").
			WithStatus(http.StatusCreated).
			WithJSON(map[string]interface{}{
				"id":      456,
				"created": true,
			})

		resp, err := http.Post(subject.URL()+"/users", "application/json", strings.NewReader(`{"name":"Bob"}`))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.JSONEq(t, `{"id":456,"created":true}`, string(body))
	})
}

func TestMockServer_OnPut(t *testing.T) {
	t.Parallel()

	t.Run("returns configured response for PUT request", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnPut("/users/123").
			WithStatus(http.StatusOK).
			WithBodyString("updated")

		req, err := http.NewRequest(http.MethodPut, subject.URL()+"/users/123", strings.NewReader(`{"name":"Updated"}`))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "updated", string(body))
	})
}

func TestMockServer_OnDelete(t *testing.T) {
	t.Parallel()

	t.Run("returns configured response for DELETE request", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnDelete("/users/123").
			WithStatus(http.StatusNoContent)

		req, err := http.NewRequest(http.MethodDelete, subject.URL()+"/users/123", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}

func TestMockServer_OnPatch(t *testing.T) {
	t.Parallel()

	t.Run("returns configured response for PATCH request", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnPatch("/users/123").
			WithStatus(http.StatusOK).
			WithBodyString("patched")

		req, err := http.NewRequest(http.MethodPatch, subject.URL()+"/users/123", strings.NewReader(`{"status":"active"}`))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "patched", string(body))
	})
}

func TestMockServer_Requests(t *testing.T) {
	t.Parallel()

	t.Run("records all received requests", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/path1").WithStatus(http.StatusOK)
		subject.OnGet("/path2").WithStatus(http.StatusOK)

		// Make requests
		resp1, _ := http.Get(subject.URL() + "/path1")
		if resp1 != nil {
			resp1.Body.Close()
		}
		resp2, _ := http.Get(subject.URL() + "/path2")
		if resp2 != nil {
			resp2.Body.Close()
		}
		resp3, _ := http.Get(subject.URL() + "/path1")
		if resp3 != nil {
			resp3.Body.Close()
		}

		requests := subject.Requests()

		assert.Equal(t, 3, len(requests))
		assert.Equal(t, "/path1", requests[0].Path)
		assert.Equal(t, "/path2", requests[1].Path)
		assert.Equal(t, "/path1", requests[2].Path)
	})

	t.Run("returns empty list when no requests received", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		requests := subject.Requests()

		assert.NotNil(t, requests)
		assert.Equal(t, 0, len(requests))
	})
}

func TestMockServer_RequestsTo(t *testing.T) {
	t.Parallel()

	t.Run("returns only requests to specific path", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/target").WithStatus(http.StatusOK)
		subject.OnGet("/other").WithStatus(http.StatusOK)

		// Make requests
		resp4, _ := http.Get(subject.URL() + "/target")
		if resp4 != nil {
			resp4.Body.Close()
		}
		resp5, _ := http.Get(subject.URL() + "/other")
		if resp5 != nil {
			resp5.Body.Close()
		}
		resp6, _ := http.Get(subject.URL() + "/target")
		if resp6 != nil {
			resp6.Body.Close()
		}

		requests := subject.RequestsTo("/target")

		assert.Equal(t, 2, len(requests))
		for _, req := range requests {
			assert.Equal(t, "/target", req.Path)
		}
	})

	t.Run("returns empty list when no requests to path", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		requests := subject.RequestsTo("/nonexistent")

		assert.NotNil(t, requests)
		assert.Equal(t, 0, len(requests))
	})
}

func TestMockServer_RequestCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(string)
		wantCount int
	}{
		{
			name: "returns zero for no requests",
			makeReqs: func(_ string) {
				// No requests
			},
			wantCount: 0,
		},
		{
			name: "returns correct count for multiple requests",
			makeReqs: func(baseURL string) {
				resp1, _ := http.Get(baseURL + "/path1")
				if resp1 != nil {
					resp1.Body.Close()
				}
				resp2, _ := http.Get(baseURL + "/path2")
				if resp2 != nil {
					resp2.Body.Close()
				}
				resp3, _ := http.Get(baseURL + "/path3")
				if resp3 != nil {
					resp3.Body.Close()
				}
			},
			wantCount: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/path1").WithStatus(http.StatusOK)
			subject.OnGet("/path2").WithStatus(http.StatusOK)
			subject.OnGet("/path3").WithStatus(http.StatusOK)

			tc.makeReqs(subject.URL())

			got := subject.RequestCount()

			assert.Equal(t, tc.wantCount, got)
		})
	}
}

func TestMockServer_RequestCountTo(t *testing.T) {
	t.Parallel()

	t.Run("returns correct count for specific path", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/users").WithStatus(http.StatusOK)
		subject.OnGet("/posts").WithStatus(http.StatusOK)

		// Make requests
		resp7, _ := http.Get(subject.URL() + "/users")
		if resp7 != nil {
			resp7.Body.Close()
		}
		resp8, _ := http.Get(subject.URL() + "/posts")
		if resp8 != nil {
			resp8.Body.Close()
		}
		resp9, _ := http.Get(subject.URL() + "/users")
		if resp9 != nil {
			resp9.Body.Close()
		}
		resp10, _ := http.Get(subject.URL() + "/users")
		if resp10 != nil {
			resp10.Body.Close()
		}

		got := subject.RequestCountTo("/users")

		assert.Equal(t, 3, got)
	})
}

func TestMockServer_Reset(t *testing.T) {
	t.Parallel()

	t.Run("clears all requests and routes", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/test").WithStatus(http.StatusOK)
		resp11, _ := http.Get(subject.URL() + "/test")
		if resp11 != nil {
			resp11.Body.Close()
		}

		assert.Equal(t, 1, subject.RequestCount())

		subject.Reset()

		assert.Equal(t, 0, subject.RequestCount())

		// Route should also be cleared - should get 404
		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestMockServer_ResetRequests(t *testing.T) {
	t.Parallel()

	t.Run("clears requests but keeps routes", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/test").WithStatus(http.StatusOK).WithBodyString("ok")
		resp12, _ := http.Get(subject.URL() + "/test")
		if resp12 != nil {
			resp12.Body.Close()
		}

		assert.Equal(t, 1, subject.RequestCount())

		subject.ResetRequests()

		assert.Equal(t, 0, subject.RequestCount())

		// Route should still work
		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "ok", string(body))
	})
}

func TestResponseBuilder_WithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		statusCode     int
		wantStatusCode int
	}{
		{
			name:           "sets 200 OK status",
			statusCode:     http.StatusOK,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "sets 404 Not Found status",
			statusCode:     http.StatusNotFound,
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "sets 500 Internal Server Error status",
			statusCode:     http.StatusInternalServerError,
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/test").WithStatus(tc.statusCode)

			resp, err := http.Get(subject.URL() + "/test")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
		})
	}
}

func TestResponseBuilder_WithHeader(t *testing.T) {
	t.Parallel()

	t.Run("sets single response header", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/test").
			WithStatus(http.StatusOK).
			WithHeader("X-Custom-Header", "custom-value")

		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "custom-value", resp.Header.Get("X-Custom-Header"))
	})
}

func TestResponseBuilder_WithHeaders(t *testing.T) {
	t.Parallel()

	t.Run("sets multiple response headers", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/test").
			WithStatus(http.StatusOK).
			WithHeaders(map[string]string{
				"X-Header-1": "value1",
				"X-Header-2": "value2",
			})

		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "value1", resp.Header.Get("X-Header-1"))
		assert.Equal(t, "value2", resp.Header.Get("X-Header-2"))
	})
}

func TestResponseBuilder_WithBody(t *testing.T) {
	t.Parallel()

	t.Run("sets raw byte response body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		bodyBytes := []byte("raw body content")
		subject.OnGet("/test").
			WithStatus(http.StatusOK).
			WithBody(bodyBytes)

		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, bodyBytes, body)
	})
}

func TestResponseBuilder_WithBodyString(t *testing.T) {
	t.Parallel()

	t.Run("sets string response body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/test").
			WithStatus(http.StatusOK).
			WithBodyString("string body")

		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, "string body", string(body))
	})
}

func TestResponseBuilder_WithJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		jsonData interface{}
		wantJSON string
	}{
		{
			name: "marshals map to JSON",
			jsonData: map[string]interface{}{
				"id":   123,
				"name": "Alice",
			},
			wantJSON: `{"id":123,"name":"Alice"}`,
		},
		{
			name: "marshals struct to JSON",
			jsonData: struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{
				ID:   456,
				Name: "Bob",
			},
			wantJSON: `{"id":456,"name":"Bob"}`,
		},
		{
			name:     "marshals slice to JSON",
			jsonData: []string{"a", "b", "c"},
			wantJSON: `["a","b","c"]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.OnGet("/test").
				WithStatus(http.StatusOK).
				WithJSON(tc.jsonData)

			resp, err := http.Get(subject.URL() + "/test")
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.JSONEq(t, tc.wantJSON, string(body))
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		})
	}
}

func TestResponseBuilder_WithDelay(t *testing.T) {
	t.Parallel()

	t.Run("delays response by specified duration", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		delay := 100 * time.Millisecond
		subject.OnGet("/slow").
			WithStatus(http.StatusOK).
			WithDelay(func() {
				time.Sleep(delay)
			})

		start := time.Now()
		resp, err := http.Get(subject.URL() + "/slow")
		elapsed := time.Since(start)

		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, elapsed >= delay, "Expected delay of at least %v, got %v", delay, elapsed)
	})
}

func TestExactPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		matchPath   string
		requestPath string
		wantMatch   bool
	}{
		{
			name:        "matches exact path",
			matchPath:   "/users",
			requestPath: "/users",
			wantMatch:   true,
		},
		{
			name:        "does not match different path",
			matchPath:   "/users",
			requestPath: "/posts",
			wantMatch:   false,
		},
		{
			name:        "does not match path with query params",
			matchPath:   "/users",
			requestPath: "/users?id=123",
			wantMatch:   true, // Query params are part of URL, not path
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.On(httpxtesting.ExactPath(tc.matchPath)).
				WithStatus(http.StatusOK).
				WithBodyString("matched")

			resp, err := http.Get(subject.URL() + tc.requestPath)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if tc.wantMatch {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, "matched", string(body))
			} else {
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			}
		})
	}
}

func TestMethodIs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		matchMethod   string
		requestMethod string
		wantMatch     bool
	}{
		{
			name:          "matches GET method",
			matchMethod:   "GET",
			requestMethod: http.MethodGet,
			wantMatch:     true,
		},
		{
			name:          "matches POST method",
			matchMethod:   "POST",
			requestMethod: http.MethodPost,
			wantMatch:     true,
		},
		{
			name:          "does not match different method",
			matchMethod:   "GET",
			requestMethod: http.MethodPost,
			wantMatch:     false,
		},
		{
			name:          "matches method case-insensitively",
			matchMethod:   "get",
			requestMethod: http.MethodGet,
			wantMatch:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.On(
				httpxtesting.MethodIs(tc.matchMethod),
				httpxtesting.ExactPath("/test"),
			).WithStatus(http.StatusOK).WithBodyString("matched")

			req, err := http.NewRequest(tc.requestMethod, subject.URL()+"/test", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.wantMatch {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			}
		})
	}
}

func TestPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prefix      string
		requestPath string
		wantMatch   bool
	}{
		{
			name:        "matches path with prefix",
			prefix:      "/api",
			requestPath: "/api/users",
			wantMatch:   true,
		},
		{
			name:        "matches exact prefix",
			prefix:      "/api",
			requestPath: "/api",
			wantMatch:   true,
		},
		{
			name:        "does not match without prefix",
			prefix:      "/api",
			requestPath: "/users",
			wantMatch:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.On(httpxtesting.PathPrefix(tc.prefix)).
				WithStatus(http.StatusOK).
				WithBodyString("matched")

			resp, err := http.Get(subject.URL() + tc.requestPath)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.wantMatch {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			}
		})
	}
}

func TestPathRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		requestPath string
		wantMatch   bool
	}{
		{
			name:        "matches numeric ID pattern",
			pattern:     `/users/\d+`,
			requestPath: "/users/123",
			wantMatch:   true,
		},
		{
			name:        "does not match non-numeric ID",
			pattern:     `/users/\d+`,
			requestPath: "/users/abc",
			wantMatch:   false,
		},
		{
			name:        "matches wildcard pattern",
			pattern:     `/api/.*`,
			requestPath: "/api/v1/users",
			wantMatch:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.On(httpxtesting.PathRegex(tc.pattern)).
				WithStatus(http.StatusOK).
				WithBodyString("matched")

			resp, err := http.Get(subject.URL() + tc.requestPath)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.wantMatch {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			}
		})
	}
}

func TestHasQueryParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		paramKey   string
		paramValue string
		requestURL string
		wantMatch  bool
	}{
		{
			name:       "matches query parameter",
			paramKey:   "id",
			paramValue: "123",
			requestURL: "/test?id=123",
			wantMatch:  true,
		},
		{
			name:       "does not match different value",
			paramKey:   "id",
			paramValue: "123",
			requestURL: "/test?id=456",
			wantMatch:  false,
		},
		{
			name:       "does not match missing parameter",
			paramKey:   "id",
			paramValue: "123",
			requestURL: "/test",
			wantMatch:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.On(httpxtesting.HasQueryParam(tc.paramKey, tc.paramValue)).
				WithStatus(http.StatusOK).
				WithBodyString("matched")

			resp, err := http.Get(subject.URL() + tc.requestURL)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.wantMatch {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			}
		})
	}
}

func TestHasHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		headerKey   string
		headerValue string
		setHeader   bool
		wantMatch   bool
	}{
		{
			name:        "matches request header",
			headerKey:   "X-Custom",
			headerValue: "test-value",
			setHeader:   true,
			wantMatch:   true,
		},
		{
			name:        "does not match missing header",
			headerKey:   "X-Custom",
			headerValue: "test-value",
			setHeader:   false,
			wantMatch:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			subject.On(httpxtesting.HasHeader(tc.headerKey, tc.headerValue)).
				WithStatus(http.StatusOK).
				WithBodyString("matched")

			req, err := http.NewRequest(http.MethodGet, subject.URL()+"/test", nil)
			require.NoError(t, err)

			if tc.setHeader {
				req.Header.Set(tc.headerKey, tc.headerValue)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.wantMatch {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			}
		})
	}
}

func TestAnd(t *testing.T) {
	t.Parallel()

	t.Run("matches when all matchers match", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.On(
			httpxtesting.And(
				httpxtesting.MethodIs("GET"),
				httpxtesting.ExactPath("/users"),
				httpxtesting.HasQueryParam("id", "123"),
			),
		).WithStatus(http.StatusOK).WithBodyString("all matched")

		resp, err := http.Get(subject.URL() + "/users?id=123")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "all matched", string(body))
	})

	t.Run("does not match when one matcher fails", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.On(
			httpxtesting.And(
				httpxtesting.MethodIs("GET"),
				httpxtesting.ExactPath("/users"),
				httpxtesting.HasQueryParam("id", "123"),
			),
		).WithStatus(http.StatusOK).WithBodyString("matched")

		// Missing query param
		resp, err := http.Get(subject.URL() + "/users")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestOr(t *testing.T) {
	t.Parallel()

	t.Run("matches when any matcher matches", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.On(
			httpxtesting.Or(
				httpxtesting.ExactPath("/users"),
				httpxtesting.ExactPath("/posts"),
			),
		).WithStatus(http.StatusOK).WithBodyString("one matched")

		resp1, err := http.Get(subject.URL() + "/users")
		require.NoError(t, err)
		defer resp1.Body.Close()

		resp2, err := http.Get(subject.URL() + "/posts")
		require.NoError(t, err)
		defer resp2.Body.Close()

		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
	})

	t.Run("does not match when all matchers fail", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.On(
			httpxtesting.Or(
				httpxtesting.ExactPath("/users"),
				httpxtesting.ExactPath("/posts"),
			),
		).WithStatus(http.StatusOK).WithBodyString("matched")

		resp, err := http.Get(subject.URL() + "/comments")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestNot(t *testing.T) {
	t.Parallel()

	t.Run("matches when sub-matcher does not match", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.On(
			httpxtesting.Not(httpxtesting.ExactPath("/excluded")),
		).WithStatus(http.StatusOK).WithBodyString("not excluded")

		resp, err := http.Get(subject.URL() + "/included")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "not excluded", string(body))
	})

	t.Run("does not match when sub-matcher matches", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.On(
			httpxtesting.Not(httpxtesting.ExactPath("/excluded")),
		).WithStatus(http.StatusOK).WithBodyString("matched")

		resp, err := http.Get(subject.URL() + "/excluded")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestMockServer_Concurrency(t *testing.T) {
	t.Parallel()

	t.Run("handles concurrent requests safely", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/test").WithStatus(http.StatusOK).WithBodyString("ok")

		var wg sync.WaitGroup
		numRequests := 100

		for range numRequests {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := http.Get(subject.URL() + "/test")
				if err == nil {
					resp.Body.Close()
				}
			}()
		}

		wg.Wait()

		assert.Equal(t, numRequests, subject.RequestCount())
	})
}

func TestRecordedRequest_CapturesDetails(t *testing.T) {
	t.Parallel()

	t.Run("captures request method, path, headers, and body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnPost("/api/users").WithStatus(http.StatusCreated)

		reqBody := `{"name":"Alice"}`
		req, err := http.NewRequest(http.MethodPost, subject.URL()+"/api/users?role=admin", strings.NewReader(reqBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Custom", "test-value")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		requests := subject.Requests()
		require.Equal(t, 1, len(requests))

		recorded := requests[0]
		assert.Equal(t, http.MethodPost, recorded.Method)
		assert.Equal(t, "/api/users", recorded.Path)
		assert.Equal(t, reqBody, string(recorded.Body))
		assert.Equal(t, "application/json", recorded.Headers.Get("Content-Type"))
		assert.Equal(t, "test-value", recorded.Headers.Get("X-Custom"))
		assert.Equal(t, "admin", recorded.QueryParams["role"][0])
	})
}

func TestResponseBuilder_FluentChaining(t *testing.T) {
	t.Parallel()

	t.Run("allows fluent method chaining", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/chain").
			WithStatus(http.StatusOK).
			WithHeader("X-Header-1", "value1").
			WithHeader("X-Header-2", "value2").
			WithJSON(map[string]string{"result": "chained"})

		resp, err := http.Get(subject.URL() + "/chain")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "value1", resp.Header.Get("X-Header-1"))
		assert.Equal(t, "value2", resp.Header.Get("X-Header-2"))
		assert.JSONEq(t, `{"result":"chained"}`, string(body))
	})
}

func TestPathRegex_InvalidPattern(t *testing.T) {
	t.Parallel()

	t.Run("handles invalid regex pattern gracefully", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		// Invalid regex pattern with unclosed bracket
		subject.On(httpxtesting.PathRegex("[invalid")).
			WithStatus(http.StatusOK)

		// Should not match anything due to invalid regex
		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestResponseBuilder_WithJSON_MarshalError(t *testing.T) {
	t.Parallel()

	t.Run("handles JSON marshal error gracefully", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		// Channel cannot be marshaled to JSON
		subject.OnGet("/test").
			WithStatus(http.StatusOK).
			WithJSON(make(chan int))

		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should return error response
		assert.Contains(t, string(body), "failed to marshal JSON")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})
}

func TestMockServer_MultipleRoutes(t *testing.T) {
	t.Parallel()

	t.Run("handles multiple routes with different matchers", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/users").WithStatus(http.StatusOK).WithBodyString("users")
		subject.OnPost("/users").WithStatus(http.StatusCreated).WithBodyString("created")
		subject.OnGet("/posts").WithStatus(http.StatusOK).WithBodyString("posts")

		// Test GET /users
		resp1, err := http.Get(subject.URL() + "/users")
		require.NoError(t, err)
		defer resp1.Body.Close()
		body1, _ := io.ReadAll(resp1.Body)

		// Test POST /users
		resp2, err := http.Post(subject.URL()+"/users", "application/json", strings.NewReader("{}"))
		require.NoError(t, err)
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)

		// Test GET /posts
		resp3, err := http.Get(subject.URL() + "/posts")
		require.NoError(t, err)
		defer resp3.Body.Close()
		body3, _ := io.ReadAll(resp3.Body)

		assert.Equal(t, "users", string(body1))
		assert.Equal(t, "created", string(body2))
		assert.Equal(t, "posts", string(body3))
	})
}

func TestMockServer_Close(t *testing.T) {
	t.Parallel()

	t.Run("server cannot be accessed after close", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		url := subject.URL()

		subject.OnGet("/test").WithStatus(http.StatusOK)
		subject.Close()

		// Request should fail after close
		_, err := http.Get(url + "/test") //nolint:bodyclose // Request expected to fail
		assert.Error(t, err)
	})
}

func TestNewResponseBuilder(t *testing.T) {
	t.Parallel()

	t.Run("creates response builder with defaults", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		// Use default response builder (no configuration)
		subject.OnGet("/test")

		resp, err := http.Get(subject.URL() + "/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 200 OK by default
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestRecordedRequest_ParseJSON(t *testing.T) {
	t.Parallel()

	t.Run("can parse JSON from recorded request body", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnPost("/api/data").WithStatus(http.StatusCreated)

		jsonData := map[string]interface{}{
			"name": "Test",
			"age":  30,
		}
		jsonBytes, _ := json.Marshal(jsonData)

		resp, err := http.Post(subject.URL()+"/api/data", "application/json", strings.NewReader(string(jsonBytes)))
		require.NoError(t, err)
		resp.Body.Close()

		requests := subject.Requests()
		require.Equal(t, 1, len(requests))

		var parsed map[string]interface{}
		err = json.Unmarshal(requests[0].Body, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "Test", parsed["name"])
		assert.Equal(t, float64(30), parsed["age"])
	})
}
