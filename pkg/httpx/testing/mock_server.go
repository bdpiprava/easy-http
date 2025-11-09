package testing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
)

// MockServer provides a test HTTP server with fluent API for defining mock responses
type MockServer struct {
	server   *httptest.Server
	routes   []*Route
	requests []*RecordedRequest
	mu       sync.RWMutex
}

// Route represents a single mock route configuration
type Route struct {
	matcher  RequestMatcher
	response *ResponseBuilder
}

// RecordedRequest captures details about a received HTTP request
type RecordedRequest struct {
	Method      string
	URL         string
	Path        string
	QueryParams map[string][]string
	Headers     http.Header
	Body        []byte
	Request     *http.Request
}

// RequestMatcher interface for matching incoming requests
type RequestMatcher interface {
	Matches(req *http.Request) bool
	String() string
}

// NewMockServer creates a new mock HTTP server
func NewMockServer() *MockServer {
	mock := &MockServer{
		routes:   make([]*Route, 0),
		requests: make([]*RecordedRequest, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// NewMockTLSServer creates a new mock HTTPS server
func NewMockTLSServer() *MockServer {
	mock := &MockServer{
		routes:   make([]*Route, 0),
		requests: make([]*RecordedRequest, 0),
	}

	mock.server = httptest.NewTLSServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// URL returns the base URL of the mock server
func (m *MockServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockServer) Close() {
	m.server.Close()
}

// OnGet registers a mock response for GET requests to the specified path
func (m *MockServer) OnGet(path string) *ResponseBuilder {
	return m.On(MethodIs("GET"), ExactPath(path))
}

// OnPost registers a mock response for POST requests to the specified path
func (m *MockServer) OnPost(path string) *ResponseBuilder {
	return m.On(MethodIs("POST"), ExactPath(path))
}

// OnPut registers a mock response for PUT requests to the specified path
func (m *MockServer) OnPut(path string) *ResponseBuilder {
	return m.On(MethodIs("PUT"), ExactPath(path))
}

// OnDelete registers a mock response for DELETE requests to the specified path
func (m *MockServer) OnDelete(path string) *ResponseBuilder {
	return m.On(MethodIs("DELETE"), ExactPath(path))
}

// OnPatch registers a mock response for PATCH requests to the specified path
func (m *MockServer) OnPatch(path string) *ResponseBuilder {
	return m.On(MethodIs("PATCH"), ExactPath(path))
}

// On registers a mock response with custom matchers
func (m *MockServer) On(matchers ...RequestMatcher) *ResponseBuilder {
	matcher := And(matchers...)
	response := NewResponseBuilder()

	route := &Route{
		matcher:  matcher,
		response: response,
	}

	m.mu.Lock()
	m.routes = append(m.routes, route)
	m.mu.Unlock()

	return response
}

// Requests returns all recorded requests received by the mock server
func (m *MockServer) Requests() []*RecordedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*RecordedRequest, len(m.requests))
	copy(result, m.requests)
	return result
}

// RequestsTo returns all recorded requests matching the given path
func (m *MockServer) RequestsTo(path string) []*RecordedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RecordedRequest, 0)
	for _, req := range m.requests {
		if req.Path == path {
			result = append(result, req)
		}
	}
	return result
}

// RequestCount returns the total number of requests received
func (m *MockServer) RequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.requests)
}

// RequestCountTo returns the number of requests to a specific path
func (m *MockServer) RequestCountTo(path string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, req := range m.requests {
		if req.Path == path {
			count++
		}
	}
	return count
}

// Reset clears all recorded requests and routes
func (m *MockServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = make([]*RecordedRequest, 0)
	m.routes = make([]*Route, 0)
}

// ResetRequests clears only the recorded requests, keeping routes
func (m *MockServer) ResetRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = make([]*RecordedRequest, 0)
}

// handleRequest processes incoming HTTP requests
func (m *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Record the request
	recorded := m.recordRequest(r)

	m.mu.Lock()
	m.requests = append(m.requests, recorded)
	m.mu.Unlock()

	// Find matching route
	m.mu.RLock()
	var matchedRoute *Route
	for _, route := range m.routes {
		if route.matcher.Matches(r) {
			matchedRoute = route
			break
		}
	}
	m.mu.RUnlock()

	// Respond based on matched route
	if matchedRoute != nil {
		matchedRoute.response.Write(w)
	} else {
		// No matching route - return 404
		http.NotFound(w, r)
	}
}

// recordRequest captures request details for verification
func (m *MockServer) recordRequest(r *http.Request) *RecordedRequest {
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	// Create a copy of query params
	queryParams := make(map[string][]string)
	for k, v := range r.URL.Query() {
		queryParams[k] = v
	}

	return &RecordedRequest{
		Method:      r.Method,
		URL:         r.URL.String(),
		Path:        r.URL.Path,
		QueryParams: queryParams,
		Headers:     r.Header.Clone(),
		Body:        body,
		Request:     r,
	}
}

// ResponseBuilder provides fluent API for building mock responses
type ResponseBuilder struct {
	statusCode int
	headers    http.Header
	body       []byte
	delay      func()
	mu         sync.RWMutex
}

// NewResponseBuilder creates a new response builder with defaults
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{
		statusCode: http.StatusOK,
		headers:    make(http.Header),
		body:       nil,
		delay:      nil,
	}
}

// WithStatus sets the HTTP status code
func (rb *ResponseBuilder) WithStatus(statusCode int) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.statusCode = statusCode
	return rb
}

// WithHeader sets a response header
func (rb *ResponseBuilder) WithHeader(key, value string) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.headers.Set(key, value)
	return rb
}

// WithHeaders sets multiple response headers
func (rb *ResponseBuilder) WithHeaders(headers map[string]string) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for k, v := range headers {
		rb.headers.Set(k, v)
	}
	return rb
}

// WithBody sets the raw response body
func (rb *ResponseBuilder) WithBody(body []byte) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.body = body
	return rb
}

// WithBodyString sets the response body from a string
func (rb *ResponseBuilder) WithBodyString(body string) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.body = []byte(body)
	return rb
}

// WithJSON sets the response body as JSON
func (rb *ResponseBuilder) WithJSON(data interface{}) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		// Fallback to error response
		rb.body = []byte(fmt.Sprintf(`{"error":"failed to marshal JSON: %v"}`, err))
		rb.headers.Set("Content-Type", "application/json")
		return rb
	}

	rb.body = jsonBytes
	rb.headers.Set("Content-Type", "application/json")
	return rb
}

// WithDelay adds a delay function to simulate slow responses
func (rb *ResponseBuilder) WithDelay(delayFunc func()) *ResponseBuilder {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.delay = delayFunc
	return rb
}

// Write writes the configured response to the http.ResponseWriter
func (rb *ResponseBuilder) Write(w http.ResponseWriter) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	// Apply delay if configured
	if rb.delay != nil {
		rb.delay()
	}

	// Write headers
	for key, values := range rb.headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code
	w.WriteHeader(rb.statusCode)

	// Write body
	if rb.body != nil {
		_, _ = w.Write(rb.body)
	}
}

// Request Matchers

// exactPathMatcher matches requests with exact path
type exactPathMatcher struct {
	path string
}

// ExactPath creates a matcher for exact path matching
func ExactPath(path string) RequestMatcher {
	return &exactPathMatcher{path: path}
}

func (m *exactPathMatcher) Matches(req *http.Request) bool {
	return req.URL.Path == m.path
}

func (m *exactPathMatcher) String() string {
	return fmt.Sprintf("path=%s", m.path)
}

// methodMatcher matches requests by HTTP method
type methodMatcher struct {
	method string
}

// MethodIs creates a matcher for HTTP method
func MethodIs(method string) RequestMatcher {
	return &methodMatcher{method: strings.ToUpper(method)}
}

func (m *methodMatcher) Matches(req *http.Request) bool {
	return strings.ToUpper(req.Method) == m.method
}

func (m *methodMatcher) String() string {
	return fmt.Sprintf("method=%s", m.method)
}

// pathPrefixMatcher matches requests with path prefix
type pathPrefixMatcher struct {
	prefix string
}

// PathPrefix creates a matcher for path prefix
func PathPrefix(prefix string) RequestMatcher {
	return &pathPrefixMatcher{prefix: prefix}
}

func (m *pathPrefixMatcher) Matches(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, m.prefix)
}

func (m *pathPrefixMatcher) String() string {
	return fmt.Sprintf("pathPrefix=%s", m.prefix)
}

// pathRegexMatcher matches requests with regex pattern
type pathRegexMatcher struct {
	pattern *regexp.Regexp
}

// PathRegex creates a matcher for regex pattern matching
func PathRegex(pattern string) RequestMatcher {
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Return a matcher that never matches if regex is invalid
		return &alwaysFalseMatcher{reason: fmt.Sprintf("invalid regex: %v", err)}
	}
	return &pathRegexMatcher{pattern: re}
}

func (m *pathRegexMatcher) Matches(req *http.Request) bool {
	return m.pattern.MatchString(req.URL.Path)
}

func (m *pathRegexMatcher) String() string {
	return fmt.Sprintf("pathRegex=%s", m.pattern.String())
}

// queryParamMatcher matches requests with specific query parameter
type queryParamMatcher struct {
	key   string
	value string
}

// HasQueryParam creates a matcher for query parameters
func HasQueryParam(key, value string) RequestMatcher {
	return &queryParamMatcher{key: key, value: value}
}

func (m *queryParamMatcher) Matches(req *http.Request) bool {
	values := req.URL.Query()[m.key]
	for _, v := range values {
		if v == m.value {
			return true
		}
	}
	return false
}

func (m *queryParamMatcher) String() string {
	return fmt.Sprintf("queryParam[%s]=%s", m.key, m.value)
}

// headerMatcher matches requests with specific header
type headerMatcher struct {
	key   string
	value string
}

// HasHeader creates a matcher for HTTP headers
func HasHeader(key, value string) RequestMatcher {
	return &headerMatcher{key: key, value: value}
}

func (m *headerMatcher) Matches(req *http.Request) bool {
	return req.Header.Get(m.key) == m.value
}

func (m *headerMatcher) String() string {
	return fmt.Sprintf("header[%s]=%s", m.key, m.value)
}

// Composite Matchers

// andMatcher matches when all sub-matchers match
type andMatcher struct {
	matchers []RequestMatcher
}

// And creates a matcher that requires all sub-matchers to match
func And(matchers ...RequestMatcher) RequestMatcher {
	return &andMatcher{matchers: matchers}
}

func (m *andMatcher) Matches(req *http.Request) bool {
	for _, matcher := range m.matchers {
		if !matcher.Matches(req) {
			return false
		}
	}
	return true
}

func (m *andMatcher) String() string {
	parts := make([]string, len(m.matchers))
	for i, matcher := range m.matchers {
		parts[i] = matcher.String()
	}
	return fmt.Sprintf("AND(%s)", strings.Join(parts, ", "))
}

// orMatcher matches when any sub-matcher matches
type orMatcher struct {
	matchers []RequestMatcher
}

// Or creates a matcher that requires at least one sub-matcher to match
func Or(matchers ...RequestMatcher) RequestMatcher {
	return &orMatcher{matchers: matchers}
}

func (m *orMatcher) Matches(req *http.Request) bool {
	for _, matcher := range m.matchers {
		if matcher.Matches(req) {
			return true
		}
	}
	return false
}

func (m *orMatcher) String() string {
	parts := make([]string, len(m.matchers))
	for i, matcher := range m.matchers {
		parts[i] = matcher.String()
	}
	return fmt.Sprintf("OR(%s)", strings.Join(parts, ", "))
}

// notMatcher inverts the result of the sub-matcher
type notMatcher struct {
	matcher RequestMatcher
}

// Not creates a matcher that inverts the sub-matcher result
func Not(matcher RequestMatcher) RequestMatcher {
	return &notMatcher{matcher: matcher}
}

func (m *notMatcher) Matches(req *http.Request) bool {
	return !m.matcher.Matches(req)
}

func (m *notMatcher) String() string {
	return fmt.Sprintf("NOT(%s)", m.matcher.String())
}

// alwaysFalseMatcher never matches (used for error cases)
type alwaysFalseMatcher struct {
	reason string
}

func (m *alwaysFalseMatcher) Matches(_ *http.Request) bool {
	return false
}

func (m *alwaysFalseMatcher) String() string {
	return fmt.Sprintf("NEVER(%s)", m.reason)
}
