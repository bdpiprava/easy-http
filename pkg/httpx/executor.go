package httpx

import (
	"github.com/pkg/errors"
)

// execute is a function that executes the request with given client and returns the response
func execute(client *Client, request *Request, respType any) (*Response, error) {
	if respType == nil {
		return nil, errors.New("response type cannot be nil")
	}

	req, err := request.ToHTTPReq(client.clientOptions)
	if err != nil {
		return nil, err
	}

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}

	return newResponse(resp, respType)
}
