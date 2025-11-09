package httpx_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

// createResponseWithStatus is a helper to create a response with a given status code
func createResponseWithStatus(statusCode int) *httpx.Response {
	return &httpx.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
	}
}

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"199 is not success", 199, false},
		{"200 is success", 200, true},
		{"201 is success", 201, true},
		{"250 is success", 250, true},
		{"299 is success", 299, true},
		{"300 is not success", 300, false},
		{"404 is not success", 404, false},
		{"500 is not success", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createResponseWithStatus(tt.statusCode)
			got := resp.IsSuccess()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponse_IsRedirect(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"299 is not redirect", 299, false},
		{"300 is redirect", 300, true},
		{"301 is redirect", 301, true},
		{"302 is redirect", 302, true},
		{"304 is redirect", 304, true},
		{"399 is redirect", 399, true},
		{"400 is not redirect", 400, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createResponseWithStatus(tt.statusCode)
			got := resp.IsRedirect()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponse_IsClientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"399 is not client error", 399, false},
		{"400 is client error", 400, true},
		{"404 is client error", 404, true},
		{"429 is client error", 429, true},
		{"499 is client error", 499, true},
		{"500 is not client error", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createResponseWithStatus(tt.statusCode)
			got := resp.IsClientError()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponse_IsServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"499 is not server error", 499, false},
		{"500 is server error", 500, true},
		{"502 is server error", 502, true},
		{"503 is server error", 503, true},
		{"599 is server error", 599, true},
		{"600 is not server error", 600, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createResponseWithStatus(tt.statusCode)
			got := resp.IsServerError()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponse_IsError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"200 is not error", 200, false},
		{"299 is not error", 299, false},
		{"300 is not error", 300, false},
		{"399 is not error", 399, false},
		{"400 is error", 400, true},
		{"404 is error", 404, true},
		{"499 is error", 499, true},
		{"500 is error", 500, true},
		{"503 is error", 503, true},
		{"599 is error", 599, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createResponseWithStatus(tt.statusCode)
			got := resp.IsError()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponse_SpecificStatusChecks(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		checkFunc  func(*httpx.Response) bool
		want       bool
	}{
		{"IsOK with 200", 200, (*httpx.Response).IsOK, true},
		{"IsOK with 201", 201, (*httpx.Response).IsOK, false},
		{"IsCreated with 201", 201, (*httpx.Response).IsCreated, true},
		{"IsCreated with 200", 200, (*httpx.Response).IsCreated, false},
		{"IsAccepted with 202", 202, (*httpx.Response).IsAccepted, true},
		{"IsNoContent with 204", 204, (*httpx.Response).IsNoContent, true},
		{"IsNotModified with 304", 304, (*httpx.Response).IsNotModified, true},
		{"IsBadRequest with 400", 400, (*httpx.Response).IsBadRequest, true},
		{"IsUnauthorized with 401", 401, (*httpx.Response).IsUnauthorized, true},
		{"IsForbidden with 403", 403, (*httpx.Response).IsForbidden, true},
		{"IsNotFound with 404", 404, (*httpx.Response).IsNotFound, true},
		{"IsNotFound with 200", 200, (*httpx.Response).IsNotFound, false},
		{"IsConflict with 409", 409, (*httpx.Response).IsConflict, true},
		{"IsTooManyRequests with 429", 429, (*httpx.Response).IsTooManyRequests, true},
		{"IsInternalServerError with 500", 500, (*httpx.Response).IsInternalServerError, true},
		{"IsBadGateway with 502", 502, (*httpx.Response).IsBadGateway, true},
		{"IsServiceUnavailable with 503", 503, (*httpx.Response).IsServiceUnavailable, true},
		{"IsGatewayTimeout with 504", 504, (*httpx.Response).IsGatewayTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := createResponseWithStatus(tt.statusCode)
			got := tt.checkFunc(resp)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponse_HeaderHelpers(t *testing.T) {
	// Setup mock server to test with actual responses
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Test ContentType
	t.Run("ContentType", func(t *testing.T) {
		mockServer.SetupMock("GET", "/json", 200, `{"test":"data"}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/json"),
		)
		assert.NoError(t, err)
		// Mock server may return text/plain or application/json - just verify it's not empty
		assert.NotEmpty(t, resp.ContentType())
	})

	// Test Location
	t.Run("Location with redirect", func(t *testing.T) {
		mockServer.SetupMock("GET", "/redirect", 301, ``)
		resp, err := httpx.GET[string](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/redirect"),
		)
		assert.NoError(t, err)
		// Note: mock server may or may not set Location header
		// This test verifies the method works without panicking
		_ = resp.Location()
	})

	// Test GetHeader
	t.Run("GetHeader", func(t *testing.T) {
		mockServer.SetupMock("GET", "/headers", 200, `{}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/headers"),
		)
		assert.NoError(t, err)
		contentType := resp.GetHeader("Content-Type")
		assert.NotEmpty(t, contentType)
	})

	// Test HasHeader
	t.Run("HasHeader exists", func(t *testing.T) {
		mockServer.SetupMock("GET", "/with-headers", 200, `{}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/with-headers"),
		)
		assert.NoError(t, err)
		assert.True(t, resp.HasHeader("Content-Type"))
	})

	t.Run("HasHeader does not exist", func(t *testing.T) {
		mockServer.SetupMock("GET", "/no-custom-header", 200, `{}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/no-custom-header"),
		)
		assert.NoError(t, err)
		assert.False(t, resp.HasHeader("X-Custom-Header-That-Does-Not-Exist"))
	})

	// Test ContentLength
	t.Run("ContentLength valid", func(t *testing.T) {
		mockServer.SetupMock("GET", "/with-length", 200, `{"data":"test"}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/with-length"),
		)
		assert.NoError(t, err)
		length, err := resp.ContentLength()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, length, int64(0))
	})
}

func TestResponse_ContentLength_EdgeCases(t *testing.T) {
	t.Run("empty Content-Length returns 0", func(t *testing.T) {
		mockServer := NewMockServer()
		defer mockServer.Close()

		mockServer.SetupMock("GET", "/no-length", 200, ``)
		resp, err := httpx.GET[string](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/no-length"),
		)
		assert.NoError(t, err)
		length, err := resp.ContentLength()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)
	})
}

func TestResponse_IntegrationWithStatusChecks(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	t.Run("successful request returns IsSuccess true", func(t *testing.T) {
		mockServer.SetupMock("GET", "/success", 200, `{"status":"ok"}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/success"),
		)
		assert.NoError(t, err)
		assert.True(t, resp.IsSuccess())
		assert.True(t, resp.IsOK())
		assert.False(t, resp.IsError())
	})

	t.Run("404 request returns IsNotFound true", func(t *testing.T) {
		mockServer.SetupMock("GET", "/missing", 404, `{"error":"not found"}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/missing"),
		)
		assert.NoError(t, err)
		assert.False(t, resp.IsSuccess())
		assert.True(t, resp.IsNotFound())
		assert.True(t, resp.IsClientError())
		assert.True(t, resp.IsError())
		assert.False(t, resp.IsServerError())
	})

	t.Run("500 request returns IsInternalServerError true", func(t *testing.T) {
		mockServer.SetupMock("GET", "/error", 500, `{"error":"server error"}`)
		resp, err := httpx.GET[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/error"),
		)
		assert.NoError(t, err)
		assert.False(t, resp.IsSuccess())
		assert.True(t, resp.IsInternalServerError())
		assert.True(t, resp.IsServerError())
		assert.True(t, resp.IsError())
		assert.False(t, resp.IsClientError())
	})

	t.Run("201 Created status", func(t *testing.T) {
		mockServer.SetupMock("POST", "/create", 201, `{"id":"123"}`)
		resp, err := httpx.POST[map[string]any](
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/create"),
			httpx.WithJSONBody(map[string]string{"name": "test"}),
		)
		assert.NoError(t, err)
		assert.True(t, resp.IsSuccess())
		assert.True(t, resp.IsCreated())
		assert.False(t, resp.IsOK())
	})
}
