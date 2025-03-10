package httpx_test

import (
	"net/http"
	"net/http/httptest"
)

// MockServer is an interface that defines the methods for a mock server
type MockServer interface {
	// GetURL returns the URL of the mock server
	GetURL() string

	// SetupMock sets up a mock server with the given endpoint, status, and body
	SetupMock(method, endpoint string, status int, body string)

	// Close closes the mock server
	Close()
}

type mockServer struct {
	server *httptest.Server
	mux    *http.ServeMux
}

// NewMockServer is a function that returns a new mock server
func NewMockServer() MockServer {
	mux := http.NewServeMux()
	return &mockServer{
		mux:    mux,
		server: httptest.NewServer(mux),
	}
}

// SetupMock is a function that sets up a mock server with the given endpoint, status, and body
func (m *mockServer) SetupMock(method, endpoint string, status int, body string) {
	m.mux.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// GetURL is a function that returns the URL of the mock server
func (m *mockServer) GetURL() string {
	return m.server.URL
}

// Close is a function that closes the mock server
func (m *mockServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
}
