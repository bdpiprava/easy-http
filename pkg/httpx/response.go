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
	Status     string
	header     http.Header
	StatusCode int
	Body       any
	RawBody    []byte
}

// newResponse is a function that creates a new response
func newResponse(httpResp *http.Response, bType any) (*Response, error) {
	defer httpResp.Body.Close()

	if bType == nil {
		return nil, fmt.Errorf("unsupported type: %T", bType)
	}

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	response := &Response{
		header:     httpResp.Header,
		Status:     httpResp.Status,
		StatusCode: httpResp.StatusCode,
		RawBody:    bodyBytes,
	}

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

// tryParsingErrorResponse is a function that tries to parse the error response as JSON object or returns the raw body
func tryParsingErrorResponse(contentBytes []byte) any {
	parsedBody := make(map[string]any)
	if json.Unmarshal(contentBytes, &parsedBody) != nil {
		return string(contentBytes)
	}
	return parsedBody
}
