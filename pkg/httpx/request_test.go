package httpx_test

import (
	"fmt"
	"io"
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
	s.run(getTestCases("GET"), httpx.GET[map[string]any])
}

func (s *RequestTestSuite) Test_POST() {
	s.run(getTestCases("POST"), httpx.POST[map[string]any])
}

func (s *RequestTestSuite) Test_DELETE() {
	s.run(getTestCases("DELETE"), httpx.DELETE[map[string]any])
}

func (s *RequestTestSuite) Test_PUT() {
	s.run(getTestCases("PUT"), httpx.PUT[map[string]any])
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
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			req, err := httpx.NewRequest("GET", tc.opts...).ToHTTPReq(httpx.ClientOptions{})

			tc.assertFn(req)
			s.Require().NoError(err)
		})
	}
}
