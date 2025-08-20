package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// BasicAuth is a struct that holds the username and password for basic authentication
type BasicAuth struct {
	Username string
	Password string
}

// ClientOptions is a struct that holds the options for the client
type ClientOptions struct {
	BaseURL   string
	Headers   http.Header
	BasicAuth BasicAuth
	Timeout   time.Duration
	Logger    *slog.Logger // Optional structured logger
	LogLevel  slog.Level   // Minimum log level for HTTP operations
}

// ClientOption is a function that takes a pointer to Options and modifies it
type ClientOption func(client *ClientOptions)

// RequestOptions is a struct that holds the options for the request
type RequestOptions struct {
	Method      string
	BaseURL     string
	Headers     http.Header
	QueryParams url.Values
	Body        io.Reader
	BasicAuth   BasicAuth
	Path        string
	Timeout     time.Duration
	Context     context.Context
	Error       error // Stores errors from RequestOptions that can't return errors directly
	Streaming   bool  // If true, response body will not be read into memory
}

// RequestOption is a function that takes a pointer to Options and modifies it
type RequestOption func(*RequestOptions)
