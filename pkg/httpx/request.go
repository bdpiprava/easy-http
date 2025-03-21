package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"
)

var supportedMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodConnect: true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
	http.MethodHead:    true,
}

var defaultClient = &Client{client: http.DefaultClient}

// Request is a request struct
type Request struct {
	opts []RequestOption
}

// NewRequest is a function that returns a new request with the given options
func NewRequest(method string, opts ...RequestOption) *Request {
	opts = append(opts, func(c *RequestOptions) {
		c.Method = method
	})
	return &Request{opts: opts}
}

// WithBaseURL is a function that sets the base URL for the request
func WithBaseURL(baseURL string) RequestOption {
	return func(c *RequestOptions) {
		c.BaseURL = baseURL
	}
}

// WithPath is a function that sets the path for the request
func WithPath(paths ...string) RequestOption {
	return func(c *RequestOptions) {
		c.Path = path.Join(paths...)
	}
}

// WithHeaders is a function that sets the headers for the request
func WithHeaders(headers http.Header) RequestOption {
	return func(c *RequestOptions) {
		if headers == nil {
			return
		}

		maps.Copy(c.Headers, headers)
	}
}

// WithHeader is a function that sets the headers for the request
func WithHeader(key string, values ...string) RequestOption {
	return func(c *RequestOptions) {
		if cur, ok := c.Headers[key]; ok {
			c.Headers[key] = append(cur, values...)
			return
		}
		c.Headers[key] = values
	}
}

// WithQueryParams is a function that sets the query parameters for the request
func WithQueryParams(params url.Values) RequestOption {
	return func(c *RequestOptions) {
		if params == nil {
			return
		}

		maps.Copy(c.QueryParams, params)
	}
}

// WithQueryParam is a function that sets the query parameters for the request
func WithQueryParam(key string, values ...string) RequestOption {
	return func(c *RequestOptions) {
		if cur, ok := c.QueryParams[key]; ok {
			c.QueryParams[key] = append(cur, values...)
			return
		}
		c.QueryParams[key] = values
	}
}

// WithContext is a function that sets the context for the request
func WithContext(ctx context.Context) RequestOption {
	return func(c *RequestOptions) {
		c.Context = ctx
	}
}

// WithBody is a function that sets the body for the request
func WithBody(body io.Reader) RequestOption {
	return func(c *RequestOptions) {
		c.Body = body
	}
}

// WithBody is a function that sets the body for the request
func WithJSONBody(body any) RequestOption {
	return func(c *RequestOptions) {
		content, err := json.Marshal(body)
		if err != nil {
			log.Println(err)
			return
		}

		c.Headers.Set("Content-Type", "application/json")
		c.Body = bytes.NewReader(content)
	}
}

// GET is a function that sends a GET request
func GET[T any](opts ...RequestOption) (*Response, error) {
	req := NewRequest(http.MethodGet, opts...)
	return defaultClient.Execute(*req, *(new(T)))
}

// POST is a function that sends a POST request
func POST[T any](opts ...RequestOption) (*Response, error) {
	req := NewRequest(http.MethodPost, opts...)
	return defaultClient.Execute(*req, *(new(T)))
}

// PUT is a function that sends a PUT request
func PUT[T any](opts ...RequestOption) (*Response, error) {
	req := NewRequest(http.MethodPut, opts...)
	return defaultClient.Execute(*req, *(new(T)))
}

// DELETE is a function that sends a DELETE request
func DELETE[T any](opts ...RequestOption) (*Response, error) {
	req := NewRequest(http.MethodDelete, opts...)
	return defaultClient.Execute(*req, *(new(T)))
}

// ToHTTPReq is a function that converts the request to an native http request
func (r *Request) ToHTTPReq(clientOpts ClientOptions) (*http.Request, error) {
	opts := buildOpts(clientOpts, r)
	return buildRequest(opts)
}

// buildRequest is a function that builds the request from the given options
func buildRequest(opts RequestOptions) (*http.Request, error) {
	if _, ok := supportedMethods[strings.ToUpper(opts.Method)]; !ok {
		return nil, errors.Errorf("unsupported method: %s", opts.Method)
	}

	req, err := http.NewRequest(opts.Method, opts.BaseURL, opts.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	if opts.Context != nil {
		req = req.WithContext(opts.Context)
	}
	req.URL.Path = path.Join(req.URL.Path, opts.Path)
	req.Header = opts.Headers
	req.URL.RawQuery = opts.QueryParams.Encode()

	return req, nil
}

// buildOpts is a function that builds the request options
func buildOpts(clientOpts ClientOptions, request *Request) RequestOptions {
	opts := RequestOptions{
		Headers:     http.Header{},
		BaseURL:     clientOpts.BaseURL,
		Timeout:     clientOpts.Timeout,
		Method:      http.MethodGet,
		QueryParams: url.Values{},
	}

	if clientOpts.Headers != nil {
		maps.Copy(opts.Headers, clientOpts.Headers)
	}

	for _, opt := range request.opts {
		opt(&opts)
	}
	return opts
}
