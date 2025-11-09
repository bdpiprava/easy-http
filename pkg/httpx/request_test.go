package httpx_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

type serverAPI struct {
	method string
	status int
	body   string
}

type testCase struct {
	name           string
	serverAPI      serverAPI
	wantStatusCode int
	wantResponse   any
	wantErr        string
	wantRawBody    string
}

type RequestTestSuite struct {
	suite.Suite
}

func TestRequestTestSuite(t *testing.T) {
	suite.Run(t, new(RequestTestSuite))
}

func (s *RequestTestSuite) Test_GET() {
	s.run(getTestCases("GET"), httpx.GET[any])
}

func (s *RequestTestSuite) Test_POST() {
	s.run(getTestCases("POST"), httpx.POST[any])
}

func (s *RequestTestSuite) Test_DELETE() {
	s.run(getTestCases("DELETE"), httpx.DELETE[any])
}

func (s *RequestTestSuite) Test_PUT() {
	s.run(getTestCases("PUT"), httpx.PUT[any])
}

func (s *RequestTestSuite) Test_PATCH() {
	s.run(getTestCases("PATCH"), httpx.PATCH[any])
}

func (s *RequestTestSuite) Test_HEAD() {
	mockServer := NewMockServer()
	defer mockServer.Close()

	testCases := []struct {
		name           string
		serverStatus   int
		wantStatusCode int
	}{
		{
			name:           "HEAD request with 200 status code",
			serverStatus:   200,
			wantStatusCode: 200,
		},
		{
			name:           "HEAD request with 404 status code",
			serverStatus:   404,
			wantStatusCode: 404,
		},
		{
			name:           "HEAD request with 500 status code",
			serverStatus:   500,
			wantStatusCode: 500,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			randomID := uuid.New().String()
			// HEAD responses don't have bodies, only headers and status
			mockServer.SetupMock("HEAD", "/api/v1/"+randomID, tc.serverStatus, "")

			resp, err := httpx.HEAD[any](
				httpx.WithBaseURL(mockServer.GetURL()),
				httpx.WithPath("/api/v1", randomID),
				httpx.WithQueryParam("region", "us"),
				httpx.WithHeader("Authorization", "Bearer abcd"),
			)

			s.Require().NoError(err)
			s.Require().Equal(tc.wantStatusCode, resp.StatusCode)
			// HEAD requests should not have a body
			s.Require().Empty(resp.RawBody)
		})
	}
}

func (s *RequestTestSuite) run(testCases []testCase, caller func(opts ...httpx.RequestOption) (*httpx.Response, error)) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			randomID := uuid.New().String()
			mockServer.SetupMock(tc.serverAPI.method, "/api/v1/"+randomID, tc.serverAPI.status, tc.serverAPI.body)

			resp, err := caller(
				httpx.WithBaseURL(mockServer.GetURL()),
				httpx.WithPath("/api/v1", randomID),
				httpx.WithQueryParam("region", "us"),
				httpx.WithHeader("Authorization", "Bearer abcd"),
				httpx.WithHeader("Content-Type", "application/json"),
			)

			s.Require().Equal(tc.wantStatusCode, resp.StatusCode)
			s.Require().Equal(tc.wantResponse, resp.Body)
			if tc.wantErr != "" {
				s.Require().Error(err)
				s.Require().EqualError(err, tc.wantErr)
				s.Require().Equal(tc.wantRawBody, string(resp.RawBody))
			}
		})
	}
}

func getTestCases(method string) []testCase {
	method = strings.ToUpper(method)
	return []testCase{
		{
			name:           fmt.Sprintf("%s request with 200 status code", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `{"name": "test"}`},
			wantResponse:   map[string]any{"name": "test"},
			wantStatusCode: 200,
		},
		{
			name:           fmt.Sprintf("%s request with 404 status code", method),
			serverAPI:      serverAPI{method: method, status: 404, body: `{"error": "not found"}`},
			wantResponse:   map[string]any{"error": "not found"},
			wantStatusCode: 404,
		},
		{
			name:           fmt.Sprintf("%s request with 500 status code", method),
			serverAPI:      serverAPI{method: method, status: 500, body: `{"error": "internal server error"}`},
			wantResponse:   map[string]any{"error": "internal server error"},
			wantStatusCode: 500,
		},
		{
			name:           fmt.Sprintf("%s request with JSON array of objects", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `[{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]`},
			wantResponse:   []any{map[string]any{"id": float64(1), "name": "Alice"}, map[string]any{"id": float64(2), "name": "Bob"}},
			wantStatusCode: 200,
		},
		{
			name:           fmt.Sprintf("%s request with JSON array of primitives", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `[1, 2, 3, 4, 5]`},
			wantResponse:   []any{float64(1), float64(2), float64(3), float64(4), float64(5)},
			wantStatusCode: 200,
		},
		{
			name:           fmt.Sprintf("%s request with JSON string primitive", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `"hello world"`},
			wantResponse:   "hello world",
			wantStatusCode: 200,
		},
		{
			name:           fmt.Sprintf("%s request with JSON number primitive", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `42.5`},
			wantResponse:   float64(42.5),
			wantStatusCode: 200,
		},
		{
			name:           fmt.Sprintf("%s request with JSON boolean primitive", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `true`},
			wantResponse:   true,
			wantStatusCode: 200,
		},
		{
			name:           fmt.Sprintf("%s request with invalid json payload", method),
			serverAPI:      serverAPI{method: method, status: 200, body: `not-a-json`},
			wantStatusCode: 200,
			wantErr:        "failed to unmarshal response as type map[string]interface {}: invalid character 'o' in literal null (expecting 'u')",
			wantRawBody:    `not-a-json`,
		},
	}
}

func (s *RequestTestSuite) Test_RequestOpts() {
	testCases := []struct {
		name     string
		opts     []httpx.RequestOption
		assertFn func(*http.Request)
	}{
		{
			name: "WithBaseURL",
			opts: []httpx.RequestOption{httpx.WithBaseURL("http://localhost:8080")},
			assertFn: func(req *http.Request) {
				s.Equal("http://localhost:8080", req.URL.String())
			},
		},
		{
			name: "WithPath",
			opts: []httpx.RequestOption{httpx.WithPath("/api/v1", "test")},
			assertFn: func(req *http.Request) {
				s.Equal("/api/v1/test", req.URL.Path)
			},
		},
		{
			name: "WithQueryParam",
			opts: []httpx.RequestOption{httpx.WithQueryParam("region", "us")},
			assertFn: func(req *http.Request) {
				s.Equal("us", req.URL.Query().Get("region"))
			},
		},
		{
			name: "WithHeader",
			opts: []httpx.RequestOption{httpx.WithHeader("Authorization", "Bearer abcd")},
			assertFn: func(req *http.Request) {
				s.Equal("Bearer abcd", req.Header.Get("Authorization"))
			},
		},
		{
			name: "WithHeaders",
			opts: []httpx.RequestOption{httpx.WithHeaders(http.Header{
				"Accept":       []string{"application/json"},
				"Content-Type": []string{"application/csv"},
			})},
			assertFn: func(req *http.Request) {
				s.Equal("application/csv", req.Header.Get("Content-Type"))
				s.Equal("application/json", req.Header.Get("Accept"))
			},
		},
		{
			name: "WithQueryParams",
			opts: []httpx.RequestOption{httpx.WithQueryParams(map[string][]string{
				"region": {"us"},
				"env":    {"prod"},
			})},
			assertFn: func(req *http.Request) {
				s.Equal("us", req.URL.Query().Get("region"))
				s.Equal("prod", req.URL.Query().Get("env"))
			},
		},
		{
			name: "WithBody",
			opts: []httpx.RequestOption{httpx.WithBody(strings.NewReader("test"))},
			assertFn: func(req *http.Request) {
				content, err := io.ReadAll(req.Body)
				s.Require().NoError(err)
				s.Equal("test", string(content))
			},
		},
		{
			name: "WithJSONBody",
			opts: []httpx.RequestOption{httpx.WithJSONBody(map[string]any{"name": "test"})},
			assertFn: func(req *http.Request) {
				content, err := io.ReadAll(req.Body)
				s.Require().NoError(err)
				s.JSONEq(`{"name":"test"}`, string(content))
				s.Equal("application/json", req.Header.Get("Content-Type"))
			},
		},
		{
			name: "WithStreaming",
			opts: []httpx.RequestOption{httpx.WithStreaming()},
			assertFn: func(req *http.Request) {
				// WithStreaming doesn't affect the actual HTTP request,
				// only the response processing, so just verify the request is valid
				s.NotNil(req)
			},
		},
		{
			name: "WithFormData",
			opts: []httpx.RequestOption{httpx.WithFormData(map[string][]string{
				"username": {"alice"},
				"password": {"secret123"},
			})},
			assertFn: func(req *http.Request) {
				content, err := io.ReadAll(req.Body)
				s.Require().NoError(err)
				s.Equal("application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
				// URL encoding can have different orders, so check both possibilities
				bodyStr := string(content)
				s.Contains(bodyStr, "username=alice")
				s.Contains(bodyStr, "password=secret123")
			},
		},
		{
			name: "WithFormData with special characters",
			opts: []httpx.RequestOption{httpx.WithFormData(map[string][]string{
				"email": {"user@example.com"},
				"name":  {"John Doe"},
			})},
			assertFn: func(req *http.Request) {
				content, err := io.ReadAll(req.Body)
				s.Require().NoError(err)
				s.Equal("application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
				bodyStr := string(content)
				// @ should be encoded as %40, space as +
				s.Contains(bodyStr, "email=user%40example.com")
				s.Contains(bodyStr, "name=John+Doe")
			},
		},
		{
			name: "WithFormData with nil values",
			opts: []httpx.RequestOption{httpx.WithFormData(nil)},
			assertFn: func(req *http.Request) {
				// Should not set body or content-type when nil
				s.Nil(req.Body)
			},
		},
		{
			name: "WithFormFields",
			opts: []httpx.RequestOption{httpx.WithFormFields(map[string]string{
				"grant_type": "client_credentials",
				"client_id":  "my-client-id",
			})},
			assertFn: func(req *http.Request) {
				content, err := io.ReadAll(req.Body)
				s.Require().NoError(err)
				s.Equal("application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
				bodyStr := string(content)
				s.Contains(bodyStr, "grant_type=client_credentials")
				s.Contains(bodyStr, "client_id=my-client-id")
			},
		},
		{
			name: "WithFormFields with nil values",
			opts: []httpx.RequestOption{httpx.WithFormFields(nil)},
			assertFn: func(req *http.Request) {
				// Should not set body or content-type when nil
				s.Nil(req.Body)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			req, err := httpx.NewRequest("GET", tc.opts...).ToHTTPReq(httpx.ClientOptions{})

			tc.assertFn(req)
			s.Require().NoError(err)
		})
	}
}

// unmarshalableType represents a type that cannot be marshaled to JSON
type unmarshalableType struct {
	Channel chan int `json:"channel"` // channels cannot be marshaled to JSON
}

func (s *RequestTestSuite) TestWithJSONBodyError() {
	// Test that WithJSONBody properly propagates JSON marshal errors
	invalidData := unmarshalableType{Channel: make(chan int)}

	req := httpx.NewRequest("POST", httpx.WithJSONBody(invalidData))
	_, err := req.ToHTTPReq(httpx.ClientOptions{})

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to marshal JSON body")
}

func (s *RequestTestSuite) TestConvenienceFunctionJSONError() {
	// Test that convenience functions (GET, POST, etc.) also propagate JSON marshal errors
	invalidData := unmarshalableType{Channel: make(chan int)}

	_, err := httpx.POST[map[string]any](
		httpx.WithBaseURL("http://example.com"),
		httpx.WithPath("/test"),
		httpx.WithJSONBody(invalidData),
	)

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to marshal JSON body")
}

const testResponseBody = `{"message":"test response"}`

func (s *RequestTestSuite) TestWithStreaming() {
	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.SetupMock("GET", "/test", 200, testResponseBody)

	// Test streaming mode
	resp, err := httpx.GET[string](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/test"),
		httpx.WithStreaming(),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify streaming mode is enabled
	s.True(resp.IsStreaming)
	s.NotNil(resp.StreamBody)
	s.Nil(resp.RawBody) // Should be nil in streaming mode
	s.Nil(resp.Body)    // Should be nil in streaming mode

	// Verify we can read from the stream
	defer resp.StreamBody.Close()
	content, err := io.ReadAll(resp.StreamBody)
	s.Require().NoError(err)
	s.JSONEq(testResponseBody, string(content))
}

func (s *RequestTestSuite) TestNonStreamingMode() {
	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.SetupMock("GET", "/test", 200, testResponseBody)

	// Test non-streaming mode (default)
	resp, err := httpx.GET[map[string]any](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/test"),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify non-streaming mode
	s.False(resp.IsStreaming)
	s.Nil(resp.StreamBody) // Should be nil in non-streaming mode
	s.NotNil(resp.RawBody) // Should contain raw bytes
	s.NotNil(resp.Body)    // Should contain parsed body

	// Verify the body was parsed correctly
	body, ok := resp.Body.(map[string]any)
	s.True(ok)
	s.Equal("test response", body["message"])
}

func (s *RequestTestSuite) TestStreamingModeWithHTTPError() {
	// Setup mock server with error response
	mockServer := NewMockServer()
	defer mockServer.Close()

	testErrorBody := `{"error":"not found"}`
	mockServer.SetupMock("GET", "/test", 404, testErrorBody)

	// Test streaming mode with HTTP error
	resp, err := httpx.GET[string](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/test"),
		httpx.WithStreaming(),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify streaming mode is enabled even for error responses
	s.True(resp.IsStreaming)
	s.NotNil(resp.StreamBody)
	s.Equal(404, resp.StatusCode)

	// Verify we can read the error response from stream
	defer resp.StreamBody.Close()
	content, err := io.ReadAll(resp.StreamBody)
	s.Require().NoError(err)
	s.JSONEq(testErrorBody, string(content))
}

func (s *RequestTestSuite) TestValidationErrors() {
	tests := []struct {
		name    string
		option  httpx.RequestOption
		wantErr string
	}{
		{
			name:    "invalid empty URL",
			option:  httpx.WithBaseURL(""),
			wantErr: "invalid base URL: URL cannot be empty",
		},
		{
			name:    "invalid URL without scheme",
			option:  httpx.WithBaseURL("example.com"),
			wantErr: "invalid base URL: URL must have a scheme (http/https): example.com",
		},
		{
			name:    "invalid URL with unsupported scheme",
			option:  httpx.WithBaseURL("ftp://example.com"),
			wantErr: "invalid base URL: unsupported URL scheme 'ftp': only http and https are supported",
		},
		{
			name:    "invalid URL without host",
			option:  httpx.WithBaseURL("http://"),
			wantErr: "invalid base URL: URL must have a host: http://",
		},
		{
			name:    "invalid header name with space",
			option:  httpx.WithHeader("Content Type", "application/json"),
			wantErr: "invalid header name: invalid character ' ' in header name: Content Type",
		},
		{
			name:    "invalid header name empty",
			option:  httpx.WithHeader("", "value"),
			wantErr: "invalid header name: header name cannot be empty",
		},
		{
			name:    "invalid header value with control character",
			option:  httpx.WithHeader("Content-Type", "application/json\x00"),
			wantErr: "invalid header value for 'Content-Type': invalid control character at position 16 in header value",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			req := httpx.NewRequest("GET", tt.option)
			_, err := req.ToHTTPReq(httpx.ClientOptions{})

			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantErr)
		})
	}
}

func (s *RequestTestSuite) TestHTTPMethodValidation() {
	tests := []struct {
		name       string
		method     string
		shouldFail bool
		wantErr    string
	}{
		{
			name:       "valid GET method",
			method:     "GET",
			shouldFail: false,
		},
		{
			name:       "valid lowercase post method",
			method:     "post",
			shouldFail: false,
		},
		{
			name:       "invalid empty method",
			method:     "",
			shouldFail: true,
			wantErr:    "invalid HTTP method: HTTP method cannot be empty",
		},
		{
			name:       "invalid custom method",
			method:     "CUSTOM",
			shouldFail: true,
			wantErr:    "invalid HTTP method: unsupported HTTP method: CUSTOM",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			req := httpx.NewRequest(tt.method)
			_, err := req.ToHTTPReq(httpx.ClientOptions{})

			if tt.shouldFail {
				s.Require().Error(err)
				s.Contains(err.Error(), tt.wantErr)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *RequestTestSuite) TestWithHeadersValidation() {
	invalidHeaders := http.Header{
		"Valid-Header":   []string{"valid-value"},
		"Invalid Header": []string{"value"}, // space in header name
	}

	req := httpx.NewRequest("GET", httpx.WithHeaders(invalidHeaders))
	_, err := req.ToHTTPReq(httpx.ClientOptions{})

	s.Require().Error(err)
	s.Contains(err.Error(), "invalid header name")
}

func (s *RequestTestSuite) TestStructuredLogging() {
	// Create a test logger that captures logs
	var logBuffer strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.SetupMock("GET", "/test", 200, testResponseBody)

	// Create client with logger
	client := httpx.NewClient(
		httpx.WithLogger(logger),
		httpx.WithLogLevel(slog.LevelDebug),
	)

	// Make request
	resp, err := client.Execute(
		*httpx.NewRequest("GET",
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/test"),
		),
		"",
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(200, resp.StatusCode)

	// Verify logs were written
	logs := logBuffer.String()
	s.Contains(logs, "HTTP request")
	s.Contains(logs, "HTTP response")
	s.Contains(logs, "method=GET")
	s.Contains(logs, "status_code=200")
}

func (s *RequestTestSuite) TestLoggingLevels() {
	// Create a logger that only captures INFO and above
	var logBuffer strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.SetupMock("GET", "/test", 200, testResponseBody)

	// Create client with logger set to INFO level
	client := httpx.NewClient(
		httpx.WithLogger(logger),
		httpx.WithLogLevel(slog.LevelInfo),
	)

	// Make request
	resp, err := client.Execute(
		*httpx.NewRequest("GET",
			httpx.WithBaseURL(mockServer.GetURL()),
			httpx.WithPath("/test"),
		),
		"",
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)

	// Verify that DEBUG request logs are NOT present (because log level is INFO)
	// but INFO response logs ARE present
	logs := logBuffer.String()
	s.NotContains(logs, "HTTP request") // DEBUG level, should be filtered out
	s.Contains(logs, "HTTP response")   // INFO level, should be present
}

func (s *RequestTestSuite) TestLoggingWithErrors() {
	// Create a test logger
	var logBuffer strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create client with logger
	client := httpx.NewClient(
		httpx.WithLogger(logger),
		httpx.WithLogLevel(slog.LevelDebug),
	)

	// Make request to invalid URL to trigger error
	_, err := client.Execute(
		*httpx.NewRequest("GET",
			httpx.WithBaseURL("http://invalid-domain-that-does-not-exist.com"),
			httpx.WithPath("/test"),
		),
		"",
	)

	s.Require().Error(err)

	// Verify error logs were written
	logs := logBuffer.String()
	s.Contains(logs, "Failed to execute HTTP request")
	s.Contains(logs, "level=ERROR")
}

func (s *RequestTestSuite) TestFormDataIntegration() {
	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.SetupMock("POST", "/login", 200, `{"token":"abc123"}`)

	// Test WithFormFields
	resp, err := httpx.POST[map[string]any](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/login"),
		httpx.WithFormFields(map[string]string{
			"username": "alice",
			"password": "secret123",
		}),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(200, resp.StatusCode)
	s.Equal("abc123", resp.Body.(map[string]any)["token"])
}

func (s *RequestTestSuite) TestFormDataWithMultipleValues() {
	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.SetupMock("POST", "/oauth/token", 200, `{"access_token":"xyz789"}`)

	// Test WithFormData with url.Values for multiple values per key
	formData := map[string][]string{
		"grant_type": {"client_credentials"},
		"scope":      {"read", "write"},
	}

	resp, err := httpx.POST[map[string]any](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/oauth/token"),
		httpx.WithFormData(formData),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(200, resp.StatusCode)
	s.Equal("xyz789", resp.Body.(map[string]any)["access_token"])
}
