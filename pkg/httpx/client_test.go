package httpx_test

import (
	"testing"

	"github.com/bdpiprava/easy-http/pkg/httpx"
	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
}

func TestClientTestSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

func (s *ClientTestSuite) Test_Execute() {
	mockServer := NewMockServer()
	defer mockServer.Close()
	s.Run("should return response", func() {
		// given
		mockServer.SetupMock("GET", "/test", 200, `{"key": "value"}`)
		cl := httpx.NewClient(httpx.WithDefaultBaseURL(mockServer.GetURL()))

		// when
		req := httpx.NewRequest("GET", httpx.WithPath("/test"))
		resp, err := cl.Execute(*req, map[string]any{})

		// then
		s.Require().NoError(err)
		s.Require().Equal(200, resp.StatusCode)
		s.Require().Equal(map[string]any{"key": "value"}, resp.Body)
	})
}
