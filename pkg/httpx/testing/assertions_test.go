package testing_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpxtesting "github.com/bdpiprava/easy-http/pkg/httpx/testing"
)

func TestAssertions_RequestReceived(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		wantError bool
	}{
		{
			name: "passes when at least one request received",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp1, _ := http.Get(mock.URL() + "/test")
				if resp1 != nil {
					resp1.Body.Close()
				}
			},
			wantError: false,
		},
		{
			name: "fails when no requests received",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestReceived()

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		makeReqs      func(mock *httpxtesting.MockServer)
		expectedCount int
		wantError     bool
	}{
		{
			name: "passes when count matches",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp2, _ := http.Get(mock.URL() + "/test")
				if resp2 != nil {
					resp2.Body.Close()
				}
				resp3, _ := http.Get(mock.URL() + "/test")
				if resp3 != nil {
					resp3.Body.Close()
				}
			},
			expectedCount: 2,
			wantError:     false,
		},
		{
			name: "fails when count does not match",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp4, _ := http.Get(mock.URL() + "/test")
				if resp4 != nil {
					resp4.Body.Close()
				}
			},
			expectedCount: 3,
			wantError:     true,
		},
		{
			name: "passes when expecting zero requests and none received",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			expectedCount: 0,
			wantError:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestCount(tc.expectedCount)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestCountTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		makeReqs      func(mock *httpxtesting.MockServer)
		path          string
		expectedCount int
		wantError     bool
	}{
		{
			name: "passes when count to path matches",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/users").WithStatus(http.StatusOK)
				mock.OnGet("/posts").WithStatus(http.StatusOK)
				resp5, _ := http.Get(mock.URL() + "/users")
				if resp5 != nil {
					resp5.Body.Close()
				}
				resp6, _ := http.Get(mock.URL() + "/posts")
				if resp6 != nil {
					resp6.Body.Close()
				}
				resp7, _ := http.Get(mock.URL() + "/users")
				if resp7 != nil {
					resp7.Body.Close()
				}
			},
			path:          "/users",
			expectedCount: 2,
			wantError:     false,
		},
		{
			name: "fails when count to path does not match",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/users").WithStatus(http.StatusOK)
				resp8, _ := http.Get(mock.URL() + "/users")
				if resp8 != nil {
					resp8.Body.Close()
				}
			},
			path:          "/users",
			expectedCount: 3,
			wantError:     true,
		},
		{
			name: "passes when expecting zero to path and none received",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/other").WithStatus(http.StatusOK)
				resp9, _ := http.Get(mock.URL() + "/other")
				if resp9 != nil {
					resp9.Body.Close()
				}
			},
			path:          "/users",
			expectedCount: 0,
			wantError:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestCountTo(tc.path, tc.expectedCount)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		path      string
		wantError bool
	}{
		{
			name: "passes when request to path exists",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/target").WithStatus(http.StatusOK)
				resp10, _ := http.Get(mock.URL() + "/target")
				if resp10 != nil {
					resp10.Body.Close()
				}
			},
			path:      "/target",
			wantError: false,
		},
		{
			name: "fails when no request to path",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/other").WithStatus(http.StatusOK)
				resp11, _ := http.Get(mock.URL() + "/other")
				if resp11 != nil {
					resp11.Body.Close()
				}
			},
			path:      "/target",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestTo(tc.path)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestWithMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		method    string
		wantError bool
	}{
		{
			name: "passes when request with method exists",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp12, _ := http.Post(mock.URL()+"/test", "application/json", strings.NewReader("{}"))
				if resp12 != nil {
					resp12.Body.Close()
				}
			},
			method:    "POST",
			wantError: false,
		},
		{
			name: "passes with case-insensitive method",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp13, _ := http.Get(mock.URL() + "/test")
				if resp13 != nil {
					resp13.Body.Close()
				}
			},
			method:    "get",
			wantError: false,
		},
		{
			name: "fails when no request with method",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp14, _ := http.Get(mock.URL() + "/test")
				if resp14 != nil {
					resp14.Body.Close()
				}
			},
			method:    "POST",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestWithMethod(tc.method)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestWithHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		makeReqs    func(mock *httpxtesting.MockServer)
		headerKey   string
		headerValue string
		wantError   bool
	}{
		{
			name: "passes when request with header exists",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				req, _ := http.NewRequest(http.MethodGet, mock.URL()+"/test", nil)
				req.Header.Set("X-Custom", "test-value")
				resp15, _ := http.DefaultClient.Do(req)
				if resp15 != nil {
					resp15.Body.Close()
				}
			},
			headerKey:   "X-Custom",
			headerValue: "test-value",
			wantError:   false,
		},
		{
			name: "fails when header key does not exist",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp16, _ := http.Get(mock.URL() + "/test")
				if resp16 != nil {
					resp16.Body.Close()
				}
			},
			headerKey:   "X-Custom",
			headerValue: "test-value",
			wantError:   true,
		},
		{
			name: "fails when header value does not match",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				req, _ := http.NewRequest(http.MethodGet, mock.URL()+"/test", nil)
				req.Header.Set("X-Custom", "different-value")
				resp17, _ := http.DefaultClient.Do(req)
				if resp17 != nil {
					resp17.Body.Close()
				}
			},
			headerKey:   "X-Custom",
			headerValue: "test-value",
			wantError:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestWithHeader(tc.headerKey, tc.headerValue)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestWithQueryParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		makeReqs   func(mock *httpxtesting.MockServer)
		paramKey   string
		paramValue string
		wantError  bool
	}{
		{
			name: "passes when request with query param exists",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp18, _ := http.Get(mock.URL() + "/test?id=123")
				if resp18 != nil {
					resp18.Body.Close()
				}
			},
			paramKey:   "id",
			paramValue: "123",
			wantError:  false,
		},
		{
			name: "fails when query param key does not exist",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp19, _ := http.Get(mock.URL() + "/test")
				if resp19 != nil {
					resp19.Body.Close()
				}
			},
			paramKey:   "id",
			paramValue: "123",
			wantError:  true,
		},
		{
			name: "fails when query param value does not match",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp20, _ := http.Get(mock.URL() + "/test?id=456")
				if resp20 != nil {
					resp20.Body.Close()
				}
			},
			paramKey:   "id",
			paramValue: "123",
			wantError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestWithQueryParam(tc.paramKey, tc.paramValue)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestWithBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		makeReqs     func(mock *httpxtesting.MockServer)
		expectedBody string
		wantError    bool
	}{
		{
			name: "passes when request with matching body exists",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp21, _ := http.Post(mock.URL()+"/test", "text/plain", strings.NewReader("test body"))
				if resp21 != nil {
					resp21.Body.Close()
				}
			},
			expectedBody: "test body",
			wantError:    false,
		},
		{
			name: "fails when no request with matching body",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp22, _ := http.Post(mock.URL()+"/test", "text/plain", strings.NewReader("different body"))
				if resp22 != nil {
					resp22.Body.Close()
				}
			},
			expectedBody: "test body",
			wantError:    true,
		},
		{
			name: "fails when no requests received",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			expectedBody: "test body",
			wantError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestWithBody(tc.expectedBody)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_RequestWithJSONBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		makeReqs     func(mock *httpxtesting.MockServer)
		expectedJSON interface{}
		wantError    bool
	}{
		{
			name: "passes when request with matching JSON body exists",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp23, _ := http.Post(mock.URL()+"/test", "application/json", strings.NewReader(`{"name":"Alice","age":30}`))
				if resp23 != nil {
					resp23.Body.Close()
				}
			},
			expectedJSON: map[string]interface{}{
				"name": "Alice",
				"age":  float64(30),
			},
			wantError: false,
		},
		{
			name: "passes when JSON fields are in different order",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp24, _ := http.Post(mock.URL()+"/test", "application/json", strings.NewReader(`{"age":30,"name":"Alice"}`))
				if resp24 != nil {
					resp24.Body.Close()
				}
			},
			expectedJSON: map[string]interface{}{
				"name": "Alice",
				"age":  float64(30),
			},
			wantError: false,
		},
		{
			name: "fails when JSON body does not match",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp25, _ := http.Post(mock.URL()+"/test", "application/json", strings.NewReader(`{"name":"Bob"}`))
				if resp25 != nil {
					resp25.Body.Close()
				}
			},
			expectedJSON: map[string]interface{}{
				"name": "Alice",
			},
			wantError: true,
		},
		{
			name: "fails when request body is not JSON",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnPost("/test").WithStatus(http.StatusOK)
				resp26, _ := http.Post(mock.URL()+"/test", "text/plain", strings.NewReader("not json"))
				if resp26 != nil {
					resp26.Body.Close()
				}
			},
			expectedJSON: map[string]interface{}{
				"name": "Alice",
			},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().RequestWithJSONBody(tc.expectedJSON)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_NoRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		wantError bool
	}{
		{
			name: "passes when no requests received",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			wantError: false,
		},
		{
			name: "fails when requests received",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp27, _ := http.Get(mock.URL() + "/test")
				if resp27 != nil {
					resp27.Body.Close()
				}
			},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().NoRequests()

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_NoRequestsTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		path      string
		wantError bool
	}{
		{
			name: "passes when no requests to path",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/other").WithStatus(http.StatusOK)
				resp28, _ := http.Get(mock.URL() + "/other")
				if resp28 != nil {
					resp28.Body.Close()
				}
			},
			path:      "/target",
			wantError: false,
		},
		{
			name: "fails when requests to path exist",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/target").WithStatus(http.StatusOK)
				resp29, _ := http.Get(mock.URL() + "/target")
				if resp29 != nil {
					resp29.Body.Close()
				}
			},
			path:      "/target",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().NoRequestsTo(tc.path)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_LastRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		wantPath  string
		wantError bool
	}{
		{
			name: "returns last request when requests exist",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				mock.OnGet("/second").WithStatus(http.StatusOK)
				mock.OnGet("/third").WithStatus(http.StatusOK)
				resp30, _ := http.Get(mock.URL() + "/first")
				if resp30 != nil {
					resp30.Body.Close()
				}
				resp31, _ := http.Get(mock.URL() + "/second")
				if resp31 != nil {
					resp31.Body.Close()
				}
				resp32, _ := http.Get(mock.URL() + "/third")
				if resp32 != nil {
					resp32.Body.Close()
				}
			},
			wantPath:  "/third",
			wantError: false,
		},
		{
			name: "returns error when no requests exist",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			wantPath:  "",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			got, gotErr := subject.Assert().LastRequest()

			if tc.wantError {
				assert.Error(t, gotErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
				assert.Equal(t, tc.wantPath, got.Path)
			}
		})
	}
}

func TestAssertions_FirstRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		wantPath  string
		wantError bool
	}{
		{
			name: "returns first request when requests exist",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				mock.OnGet("/second").WithStatus(http.StatusOK)
				resp33, _ := http.Get(mock.URL() + "/first")
				if resp33 != nil {
					resp33.Body.Close()
				}
				resp34, _ := http.Get(mock.URL() + "/second")
				if resp34 != nil {
					resp34.Body.Close()
				}
			},
			wantPath:  "/first",
			wantError: false,
		},
		{
			name: "returns error when no requests exist",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			wantPath:  "",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			got, gotErr := subject.Assert().FirstRequest()

			if tc.wantError {
				assert.Error(t, gotErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
				assert.Equal(t, tc.wantPath, got.Path)
			}
		})
	}
}

func TestAssertions_RequestAtIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		makeReqs  func(mock *httpxtesting.MockServer)
		index     int
		wantPath  string
		wantError bool
	}{
		{
			name: "returns request at valid index",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				mock.OnGet("/second").WithStatus(http.StatusOK)
				mock.OnGet("/third").WithStatus(http.StatusOK)
				resp35, _ := http.Get(mock.URL() + "/first")
				if resp35 != nil {
					resp35.Body.Close()
				}
				resp36, _ := http.Get(mock.URL() + "/second")
				if resp36 != nil {
					resp36.Body.Close()
				}
				resp37, _ := http.Get(mock.URL() + "/third")
				if resp37 != nil {
					resp37.Body.Close()
				}
			},
			index:     1,
			wantPath:  "/second",
			wantError: false,
		},
		{
			name: "returns error for negative index",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp38, _ := http.Get(mock.URL() + "/test")
				if resp38 != nil {
					resp38.Body.Close()
				}
			},
			index:     -1,
			wantPath:  "",
			wantError: true,
		},
		{
			name: "returns error for index out of range",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp39, _ := http.Get(mock.URL() + "/test")
				if resp39 != nil {
					resp39.Body.Close()
				}
			},
			index:     10,
			wantPath:  "",
			wantError: true,
		},
		{
			name: "returns error when no requests exist",
			makeReqs: func(_ *httpxtesting.MockServer) {
				// No requests
			},
			index:     0,
			wantPath:  "",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			got, gotErr := subject.Assert().RequestAtIndex(tc.index)

			if tc.wantError {
				assert.Error(t, gotErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
				assert.Equal(t, tc.wantPath, got.Path)
			}
		})
	}
}

func TestAssertions_VerifySequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		makeReqs      func(mock *httpxtesting.MockServer)
		expectedPaths []string
		wantError     bool
	}{
		{
			name: "passes when sequence matches",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				mock.OnGet("/second").WithStatus(http.StatusOK)
				mock.OnGet("/third").WithStatus(http.StatusOK)
				resp40, _ := http.Get(mock.URL() + "/first")
				if resp40 != nil {
					resp40.Body.Close()
				}
				resp41, _ := http.Get(mock.URL() + "/second")
				if resp41 != nil {
					resp41.Body.Close()
				}
				resp42, _ := http.Get(mock.URL() + "/third")
				if resp42 != nil {
					resp42.Body.Close()
				}
			},
			expectedPaths: []string{"/first", "/second", "/third"},
			wantError:     false,
		},
		{
			name: "passes when checking partial sequence",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				mock.OnGet("/second").WithStatus(http.StatusOK)
				mock.OnGet("/third").WithStatus(http.StatusOK)
				resp43, _ := http.Get(mock.URL() + "/first")
				if resp43 != nil {
					resp43.Body.Close()
				}
				resp44, _ := http.Get(mock.URL() + "/second")
				if resp44 != nil {
					resp44.Body.Close()
				}
				resp45, _ := http.Get(mock.URL() + "/third")
				if resp45 != nil {
					resp45.Body.Close()
				}
			},
			expectedPaths: []string{"/first", "/second"},
			wantError:     false,
		},
		{
			name: "fails when sequence order is wrong",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				mock.OnGet("/second").WithStatus(http.StatusOK)
				resp46, _ := http.Get(mock.URL() + "/first")
				if resp46 != nil {
					resp46.Body.Close()
				}
				resp47, _ := http.Get(mock.URL() + "/second")
				if resp47 != nil {
					resp47.Body.Close()
				}
			},
			expectedPaths: []string{"/second", "/first"},
			wantError:     true,
		},
		{
			name: "fails when not enough requests",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/first").WithStatus(http.StatusOK)
				resp48, _ := http.Get(mock.URL() + "/first")
				if resp48 != nil {
					resp48.Body.Close()
				}
			},
			expectedPaths: []string{"/first", "/second", "/third"},
			wantError:     true,
		},
		{
			name: "passes with empty expected sequence",
			makeReqs: func(mock *httpxtesting.MockServer) {
				mock.OnGet("/test").WithStatus(http.StatusOK)
				resp49, _ := http.Get(mock.URL() + "/test")
				if resp49 != nil {
					resp49.Body.Close()
				}
			},
			expectedPaths: []string{},
			wantError:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpxtesting.NewMockServer()
			defer subject.Close()

			tc.makeReqs(subject)

			gotErr := subject.Assert().VerifySequence(tc.expectedPaths...)

			if tc.wantError {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

func TestAssertions_Assert(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil assertions helper", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		got := subject.Assert()

		require.NotNil(t, got)
	})
}

func TestAssertions_MultipleAssertions(t *testing.T) {
	t.Parallel()

	t.Run("can chain multiple assertions", func(t *testing.T) {
		t.Parallel()

		subject := httpxtesting.NewMockServer()
		defer subject.Close()

		subject.OnGet("/users").WithStatus(http.StatusOK)
		subject.OnPost("/users").WithStatus(http.StatusCreated)

		req1, _ := http.NewRequest(http.MethodGet, subject.URL()+"/users?id=123", nil)
		req1.Header.Set("Authorization", "Bearer token")
		resp50, _ := http.DefaultClient.Do(req1)
		if resp50 != nil {
			resp50.Body.Close()
		}

		resp151, _ := http.Post(subject.URL()+"/users", "application/json", strings.NewReader(`{"name":"Alice"}`))
		if resp151 != nil {
			resp151.Body.Close()
		}

		// Multiple assertions should all pass
		assert.NoError(t, subject.Assert().RequestCount(2))
		assert.NoError(t, subject.Assert().RequestTo("/users"))
		assert.NoError(t, subject.Assert().RequestWithMethod("GET"))
		assert.NoError(t, subject.Assert().RequestWithMethod("POST"))
		assert.NoError(t, subject.Assert().RequestWithHeader("Authorization", "Bearer token"))
		assert.NoError(t, subject.Assert().RequestWithQueryParam("id", "123"))
		assert.NoError(t, subject.Assert().VerifySequence("/users", "/users"))
	})
}
