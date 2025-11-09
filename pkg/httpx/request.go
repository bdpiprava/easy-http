package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
		if err := validateHTTPMethod(method); err != nil {
			c.Error = errors.Wrap(err, "invalid HTTP method")
			return
		}
		c.Method = method
	})
	return &Request{opts: opts}
}

// WithBaseURL is a function that sets the base URL for the request
func WithBaseURL(baseURL string) RequestOption {
	return func(c *RequestOptions) {
		if err := validateURL(baseURL); err != nil {
			c.Error = errors.Wrap(err, "invalid base URL")
			return
		}
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

		// Validate all headers before copying
		for key, values := range headers {
			if err := validateHeaderName(key); err != nil {
				c.Error = errors.Wrap(err, "invalid header name")
				return
			}

			for _, value := range values {
				if err := validateHeaderValue(value); err != nil {
					c.Error = errors.Wrapf(err, "invalid header value for '%s'", key)
					return
				}
			}
		}

		maps.Copy(c.Headers, headers)
	}
}

// WithHeader is a function that sets the headers for the request
func WithHeader(key string, values ...string) RequestOption {
	return func(c *RequestOptions) {
		if err := validateHeaderName(key); err != nil {
			c.Error = errors.Wrap(err, "invalid header name")
			return
		}

		for _, value := range values {
			if err := validateHeaderValue(value); err != nil {
				c.Error = errors.Wrapf(err, "invalid header value for '%s'", key)
				return
			}
		}

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

// WithJSONBody is a function that sets the JSON body for the request
func WithJSONBody(body any) RequestOption {
	return func(c *RequestOptions) {
		content, err := json.Marshal(body)
		if err != nil {
			c.Error = errors.Wrap(err, "failed to marshal JSON body")
			return
		}

		c.Headers.Set("Content-Type", "application/json")
		c.Body = bytes.NewReader(content)
	}
}

// WithBasicAuth is a function that sets basic authentication for the request
func WithBasicAuth(username, password string) RequestOption {
	return func(c *RequestOptions) {
		c.BasicAuth = BasicAuth{
			Username: username,
			Password: password,
		}
	}
}

// WithStreaming is a function that enables streaming mode for the response
// In streaming mode, the response body is not read into memory and must be
// manually closed by the caller using Response.StreamBody.Close()
func WithStreaming() RequestOption {
	return func(c *RequestOptions) {
		c.Streaming = true
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

// PATCH is a function that sends a PATCH request
func PATCH[T any](opts ...RequestOption) (*Response, error) {
	req := NewRequest(http.MethodPatch, opts...)
	return defaultClient.Execute(*req, *(new(T)))
}

// HEAD is a function that sends a HEAD request
func HEAD[T any](opts ...RequestOption) (*Response, error) {
	req := NewRequest(http.MethodHead, opts...)
	return defaultClient.Execute(*req, *(new(T)))
}

// ToHTTPReq is a function that converts the request to an native http request
func (r *Request) ToHTTPReq(clientOpts ClientOptions) (*http.Request, error) {
	opts := buildOpts(clientOpts, r)
	return buildRequest(opts)
}

// buildRequest is a function that builds the request from the given options
func buildRequest(opts RequestOptions) (*http.Request, error) {
	// Check for errors that occurred during option processing
	if opts.Error != nil {
		return nil, opts.Error
	}

	if _, ok := supportedMethods[strings.ToUpper(opts.Method)]; !ok {
		return nil, errors.Errorf("unsupported method: %s", opts.Method)
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.BaseURL, opts.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.URL.Path = path.Join(req.URL.Path, opts.Path)
	req.Header = opts.Headers
	req.URL.RawQuery = opts.QueryParams.Encode()

	// Apply basic auth if specified
	if opts.BasicAuth.Username != "" || opts.BasicAuth.Password != "" {
		req.SetBasicAuth(opts.BasicAuth.Username, opts.BasicAuth.Password)
	}

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

// buildOptsFromConfig builds request options using the new configuration architecture
func buildOptsFromConfig(clientConfig ClientConfig, request *Request) RequestOptions {
	// Start with request-specific config
	requestConfig := RequestConfig{
		Method:      http.MethodGet,
		Headers:     make(http.Header),
		QueryParams: make(url.Values),
	}

	// Apply request options to config
	for _, opt := range request.opts {
		// Convert old RequestOption to new format by applying to a temporary RequestOptions
		// and then converting back to RequestConfig
		tempOpts := RequestOptions{
			Headers:     make(http.Header),
			QueryParams: make(url.Values),
		}
		opt(&tempOpts)

		// Transfer from tempOpts to requestConfig
		if tempOpts.Method != "" {
			requestConfig.Method = tempOpts.Method
		}
		if tempOpts.BaseURL != "" {
			requestConfig.BaseURL = tempOpts.BaseURL
		}
		if tempOpts.Path != "" {
			requestConfig.Path = tempOpts.Path
		}
		if len(tempOpts.Headers) > 0 {
			for key, values := range tempOpts.Headers {
				requestConfig.Headers[key] = values
			}
		}
		if len(tempOpts.QueryParams) > 0 {
			for key, values := range tempOpts.QueryParams {
				requestConfig.QueryParams[key] = values
			}
		}
		if tempOpts.Body != nil {
			requestConfig.Body = tempOpts.Body
		}
		if tempOpts.Context != nil {
			requestConfig.Context = tempOpts.Context
		}
		if tempOpts.Timeout != 0 {
			requestConfig.Timeout = tempOpts.Timeout
		}
		if tempOpts.BasicAuth.Username != "" || tempOpts.BasicAuth.Password != "" {
			requestConfig.BasicAuth = tempOpts.BasicAuth
		}
		if tempOpts.Error != nil {
			requestConfig.Error = tempOpts.Error
		}
		requestConfig.Streaming = tempOpts.Streaming
	}

	// Merge with client defaults
	requestConfig.MergeWithDefaults(clientConfig)

	// Convert back to RequestOptions for backward compatibility
	return requestConfig.ToRequestOptions()
}

// validateURL validates if the provided URL is valid
func validateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL cannot be empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse URL: %s", rawURL)
	}

	if parsedURL.Scheme == "" {
		return errors.Errorf("URL must have a scheme (http/https): %s", rawURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.Errorf("unsupported URL scheme '%s': only http and https are supported", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return errors.Errorf("URL must have a host: %s", rawURL)
	}

	return nil
}

// validateHTTPMethod validates if the provided HTTP method is supported
func validateHTTPMethod(method string) error {
	if method == "" {
		return errors.New("HTTP method cannot be empty")
	}

	upperMethod := strings.ToUpper(method)
	if _, ok := supportedMethods[upperMethod]; !ok {
		return errors.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// validateHeaderName validates if the provided header name is valid according to RFC 7230
func validateHeaderName(name string) error {
	if name == "" {
		return errors.New("header name cannot be empty")
	}

	// Check for valid header name characters according to RFC 7230
	// tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
	//         "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
	for _, char := range name {
		if !isValidHeaderNameChar(char) {
			return errors.Errorf("invalid character '%c' in header name: %s", char, name)
		}
	}

	return nil
}

// validateHeaderValue validates if the provided header value is valid according to RFC 7230
func validateHeaderValue(value string) error {
	// Header values can contain any printable ASCII characters and horizontal tab
	// but should not contain control characters (except horizontal tab)
	for i, char := range value {
		if char < 0x20 && char != 0x09 { // Allow horizontal tab (0x09)
			return errors.Errorf("invalid control character at position %d in header value", i)
		}
		if char > 0x7E { // Non-ASCII characters
			return errors.Errorf("invalid non-ASCII character at position %d in header value", i)
		}
	}

	return nil
}

// isValidHeaderNameChar checks if a character is valid in an HTTP header name
func isValidHeaderNameChar(char rune) bool {
	return (char >= 'A' && char <= 'Z') ||
		(char >= 'a' && char <= 'z') ||
		(char >= '0' && char <= '9') ||
		char == '!' || char == '#' || char == '$' || char == '%' ||
		char == '&' || char == '\'' || char == '*' || char == '+' ||
		char == '-' || char == '.' || char == '^' || char == '_' ||
		char == '`' || char == '|' || char == '~'
}
