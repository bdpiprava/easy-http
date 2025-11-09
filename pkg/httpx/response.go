package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
)

// Response is a response struct that holds the status code, body and raw body
type Response struct {
	Status       string
	header       http.Header
	StatusCode   int
	Body         any
	RawBody      []byte
	StreamBody   io.ReadCloser  // Only set when streaming is enabled
	IsStreaming  bool           // Indicates if this response is in streaming mode
	httpResponse *http.Response // Original HTTP response for cookie access
}

// newResponse is a function that creates a new response
func newResponse(httpResp *http.Response, bType any, streaming bool) (*Response, error) {
	if bType == nil {
		return nil, fmt.Errorf("unsupported type: %T", bType)
	}

	response := &Response{
		header:       httpResp.Header,
		Status:       httpResp.Status,
		StatusCode:   httpResp.StatusCode,
		IsStreaming:  streaming,
		httpResponse: httpResp,
	}

	// In streaming mode, don't read the body into memory
	if streaming {
		response.StreamBody = httpResp.Body
		// Note: Caller is responsible for closing StreamBody
		return response, nil
	}

	// Non-streaming mode: read body into memory as before
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	response.RawBody = bodyBytes

	if httpResp.StatusCode > 299 {
		response.Body = tryParsingErrorResponse(bodyBytes)
		return response, nil
	}

	// Handle empty response bodies (e.g., 204 No Content)
	if len(bodyBytes) == 0 {
		response.Body = bType
		return response, nil
	}

	bTypeReflected := reflect.TypeOf(bType)
	if bTypeReflected.Kind() == reflect.String {
		response.Body = string(bodyBytes)
		return response, nil
	}

	// Create a new instance of the underlying type for proper JSON unmarshaling
	targetType := reflect.TypeOf(bType)
	targetValue := reflect.New(targetType).Interface()

	err = json.Unmarshal(bodyBytes, targetValue)
	if err != nil {
		return response, errors.Wrapf(err, "failed to unmarshal response as type %T", bType)
	}

	// Dereference the pointer to get the actual value
	response.Body = reflect.ValueOf(targetValue).Elem().Interface()
	return response, nil
}

// Header returns the response headers
func (r *Response) Header() http.Header {
	return r.header
}

// tryParsingErrorResponse is a function that tries to parse the error response as JSON object or returns the raw body
func tryParsingErrorResponse(contentBytes []byte) any {
	parsedBody := make(map[string]any)
	if json.Unmarshal(contentBytes, &parsedBody) != nil {
		return string(contentBytes)
	}
	return parsedBody
}

// Status category helpers

// IsSuccess returns true if the status code is 2xx
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsRedirect returns true if the status code is 3xx
func (r *Response) IsRedirect() bool {
	return r.StatusCode >= 300 && r.StatusCode < 400
}

// IsClientError returns true if the status code is 4xx
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the status code is 5xx
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// IsError returns true if the status code is 4xx or 5xx
func (r *Response) IsError() bool {
	return r.IsClientError() || r.IsServerError()
}

// Specific status code helpers

// IsOK returns true if the status code is 200
func (r *Response) IsOK() bool {
	return r.StatusCode == http.StatusOK
}

// IsCreated returns true if the status code is 201
func (r *Response) IsCreated() bool {
	return r.StatusCode == http.StatusCreated
}

// IsAccepted returns true if the status code is 202
func (r *Response) IsAccepted() bool {
	return r.StatusCode == http.StatusAccepted
}

// IsNoContent returns true if the status code is 204
func (r *Response) IsNoContent() bool {
	return r.StatusCode == http.StatusNoContent
}

// IsNotModified returns true if the status code is 304
func (r *Response) IsNotModified() bool {
	return r.StatusCode == http.StatusNotModified
}

// IsBadRequest returns true if the status code is 400
func (r *Response) IsBadRequest() bool {
	return r.StatusCode == http.StatusBadRequest
}

// IsUnauthorized returns true if the status code is 401
func (r *Response) IsUnauthorized() bool {
	return r.StatusCode == http.StatusUnauthorized
}

// IsForbidden returns true if the status code is 403
func (r *Response) IsForbidden() bool {
	return r.StatusCode == http.StatusForbidden
}

// IsNotFound returns true if the status code is 404
func (r *Response) IsNotFound() bool {
	return r.StatusCode == http.StatusNotFound
}

// IsConflict returns true if the status code is 409
func (r *Response) IsConflict() bool {
	return r.StatusCode == http.StatusConflict
}

// IsTooManyRequests returns true if the status code is 429
func (r *Response) IsTooManyRequests() bool {
	return r.StatusCode == http.StatusTooManyRequests
}

// IsInternalServerError returns true if the status code is 500
func (r *Response) IsInternalServerError() bool {
	return r.StatusCode == http.StatusInternalServerError
}

// IsBadGateway returns true if the status code is 502
func (r *Response) IsBadGateway() bool {
	return r.StatusCode == http.StatusBadGateway
}

// IsServiceUnavailable returns true if the status code is 503
func (r *Response) IsServiceUnavailable() bool {
	return r.StatusCode == http.StatusServiceUnavailable
}

// IsGatewayTimeout returns true if the status code is 504
func (r *Response) IsGatewayTimeout() bool {
	return r.StatusCode == http.StatusGatewayTimeout
}

// Header convenience helpers

// ContentType returns the Content-Type header value
func (r *Response) ContentType() string {
	return r.Header().Get("Content-Type")
}

// ContentLength returns the Content-Length header value as int64
func (r *Response) ContentLength() (int64, error) {
	lengthStr := r.Header().Get("Content-Length")
	if lengthStr == "" {
		return 0, nil
	}
	length, err := strconv.ParseInt(lengthStr, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse Content-Length: %s", lengthStr)
	}
	return length, nil
}

// Location returns the Location header value (typically used for redirects)
func (r *Response) Location() string {
	return r.Header().Get("Location")
}

// GetHeader returns the value of a header by name
func (r *Response) GetHeader(name string) string {
	return r.Header().Get(name)
}

// HasHeader returns true if the response contains the specified header
func (r *Response) HasHeader(name string) bool {
	return r.Header().Get(name) != ""
}

// Cookie access helpers

// GetCookie returns a specific cookie from the response by name
func (r *Response) GetCookie(name string) *http.Cookie {
	for _, cookie := range r.GetCookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// GetCookies returns all cookies set in the response
func (r *Response) GetCookies() []*http.Cookie {
	if r.httpResponse == nil {
		return nil
	}
	return r.httpResponse.Cookies()
}

// HasCookie checks if a specific cookie exists in the response
func (r *Response) HasCookie(name string) bool {
	return r.GetCookie(name) != nil
}
