package testing

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Assertions provides helper methods for verifying mock server behavior
type Assertions struct {
	mock *MockServer
}

// Assert returns an assertions helper for the mock server
func (m *MockServer) Assert() *Assertions {
	return &Assertions{mock: m}
}

// RequestReceived verifies that at least one request was received
func (a *Assertions) RequestReceived() error {
	if a.mock.RequestCount() == 0 {
		return fmt.Errorf("expected at least one request, but none were received")
	}
	return nil
}

// RequestCount verifies the exact number of requests received
func (a *Assertions) RequestCount(expected int) error {
	actual := a.mock.RequestCount()
	if actual != expected {
		return fmt.Errorf("expected %d requests, but got %d", expected, actual)
	}
	return nil
}

// RequestCountTo verifies the number of requests to a specific path
func (a *Assertions) RequestCountTo(path string, expected int) error {
	actual := a.mock.RequestCountTo(path)
	if actual != expected {
		return fmt.Errorf("expected %d requests to %s, but got %d", expected, path, actual)
	}
	return nil
}

// RequestTo verifies that a request was made to the specified path
func (a *Assertions) RequestTo(path string) error {
	requests := a.mock.RequestsTo(path)
	if len(requests) == 0 {
		return fmt.Errorf("expected request to %s, but none were received", path)
	}
	return nil
}

// RequestWithMethod verifies that a request with the specified method was received
func (a *Assertions) RequestWithMethod(method string) error {
	method = strings.ToUpper(method)
	for _, req := range a.mock.Requests() {
		if strings.ToUpper(req.Method) == method {
			return nil
		}
	}
	return fmt.Errorf("expected request with method %s, but none were received", method)
}

// RequestWithHeader verifies that a request with the specified header was received
func (a *Assertions) RequestWithHeader(key, value string) error {
	for _, req := range a.mock.Requests() {
		if req.Headers.Get(key) == value {
			return nil
		}
	}
	return fmt.Errorf("expected request with header %s=%s, but none were received", key, value)
}

// RequestWithQueryParam verifies that a request with the specified query parameter was received
func (a *Assertions) RequestWithQueryParam(key, value string) error {
	for _, req := range a.mock.Requests() {
		values := req.QueryParams[key]
		for _, v := range values {
			if v == value {
				return nil
			}
		}
	}
	return fmt.Errorf("expected request with query param %s=%s, but none were received", key, value)
}

// RequestWithBody verifies that a request with the specified body was received
func (a *Assertions) RequestWithBody(expectedBody string) error {
	for _, req := range a.mock.Requests() {
		if string(req.Body) == expectedBody {
			return nil
		}
	}
	return fmt.Errorf("expected request with body %q, but none were received", expectedBody)
}

// RequestWithJSONBody verifies that a request with matching JSON body was received
func (a *Assertions) RequestWithJSONBody(expected interface{}) error {
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("failed to marshal expected JSON: %w", err)
	}

	for _, req := range a.mock.Requests() {
		// Try to parse the request body as JSON
		var reqData interface{}
		if err := json.Unmarshal(req.Body, &reqData); err != nil {
			continue // Not JSON, skip
		}

		// Re-marshal to normalize formatting
		normalizedReq, err := json.Marshal(reqData)
		if err != nil {
			continue
		}

		if string(normalizedReq) == string(expectedJSON) {
			return nil
		}
	}

	return fmt.Errorf("expected request with JSON body %s, but none were received", string(expectedJSON))
}

// NoRequests verifies that no requests were received
func (a *Assertions) NoRequests() error {
	count := a.mock.RequestCount()
	if count > 0 {
		return fmt.Errorf("expected no requests, but got %d", count)
	}
	return nil
}

// NoRequestsTo verifies that no requests were made to the specified path
func (a *Assertions) NoRequestsTo(path string) error {
	count := a.mock.RequestCountTo(path)
	if count > 0 {
		return fmt.Errorf("expected no requests to %s, but got %d", path, count)
	}
	return nil
}

// LastRequest returns the most recent request or an error if none exist
func (a *Assertions) LastRequest() (*RecordedRequest, error) {
	requests := a.mock.Requests()
	if len(requests) == 0 {
		return nil, fmt.Errorf("no requests received")
	}
	return requests[len(requests)-1], nil
}

// FirstRequest returns the first request or an error if none exist
func (a *Assertions) FirstRequest() (*RecordedRequest, error) {
	requests := a.mock.Requests()
	if len(requests) == 0 {
		return nil, fmt.Errorf("no requests received")
	}
	return requests[0], nil
}

// RequestAtIndex returns the request at the specified index or an error
func (a *Assertions) RequestAtIndex(index int) (*RecordedRequest, error) {
	requests := a.mock.Requests()
	if index < 0 || index >= len(requests) {
		return nil, fmt.Errorf("request index %d out of range (0-%d)", index, len(requests)-1)
	}
	return requests[index], nil
}

// VerifySequence verifies that requests were received in the specified order
func (a *Assertions) VerifySequence(expectedPaths ...string) error {
	requests := a.mock.Requests()
	if len(requests) < len(expectedPaths) {
		return fmt.Errorf("expected at least %d requests, but only got %d", len(expectedPaths), len(requests))
	}

	for i, expectedPath := range expectedPaths {
		if requests[i].Path != expectedPath {
			return fmt.Errorf("expected request %d to be %s, but was %s", i, expectedPath, requests[i].Path)
		}
	}

	return nil
}
