package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
)

// Response is a response struct that holds the status code, body and raw body
type Response struct {
	Status      string
	header      http.Header
	StatusCode  int
	Body        any
	RawBody     []byte
	StreamBody  io.ReadCloser // Only set when streaming is enabled
	IsStreaming bool          // Indicates if this response is in streaming mode
}

// newResponse is a function that creates a new response
func newResponse(httpResp *http.Response, bType any, streaming bool) (*Response, error) {
	if bType == nil {
		return nil, fmt.Errorf("unsupported type: %T", bType)
	}

	response := &Response{
		header:      httpResp.Header,
		Status:      httpResp.Status,
		StatusCode:  httpResp.StatusCode,
		IsStreaming: streaming,
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

	if reflect.TypeOf(bType).Kind() == reflect.String {
		response.Body = string(bodyBytes)
		return response, nil
	}

	err = json.Unmarshal(bodyBytes, &bType)
	if err != nil {
		return response, errors.Wrapf(err, "failed to unmarshal response as type %T", bType)
	}

	response.Body = bType
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
