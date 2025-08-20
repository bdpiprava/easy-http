package httpx

import (
	"context"
	"net/http"
)

// Middleware represents a function that can intercept and modify requests/responses
type Middleware interface {
	// Name returns a unique identifier for this middleware
	Name() string

	// Execute processes the request and delegates to the next middleware or final handler
	Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error)
}

// MiddlewareFunc is a function signature for middleware execution
type MiddlewareFunc func(ctx context.Context, req *http.Request) (*http.Response, error)

// MiddlewareChain manages a collection of middlewares and executes them in order
type MiddlewareChain struct {
	middlewares []Middleware
	finalFunc   MiddlewareFunc
}

// NewMiddlewareChain creates a new middleware chain with the given final function
func NewMiddlewareChain(finalFunc MiddlewareFunc) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
		finalFunc:   finalFunc,
	}
}

// Add appends a middleware to the chain
func (c *MiddlewareChain) Add(middleware Middleware) *MiddlewareChain {
	c.middlewares = append(c.middlewares, middleware)
	return c
}

// Execute runs the middleware chain with the given request
func (c *MiddlewareChain) Execute(ctx context.Context, req *http.Request) (*http.Response, error) {
	if len(c.middlewares) == 0 {
		return c.finalFunc(ctx, req)
	}

	// Build the chain by wrapping each middleware around the next
	var current MiddlewareFunc = c.finalFunc

	// Start from the end and work backwards to build the chain
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		middleware := c.middlewares[i]
		next := current
		current = func(ctx context.Context, req *http.Request) (*http.Response, error) {
			return middleware.Execute(ctx, req, next)
		}
	}

	return current(ctx, req)
}

// RequestInterceptor provides hooks for request modification
type RequestInterceptor interface {
	// BeforeRequest is called before the request is sent
	BeforeRequest(ctx context.Context, req *http.Request) error
}

// ResponseInterceptor provides hooks for response processing
type ResponseInterceptor interface {
	// AfterResponse is called after a successful response is received
	AfterResponse(ctx context.Context, req *http.Request, resp *http.Response) (*http.Response, error)
}

// ErrorInterceptor provides hooks for error handling
type ErrorInterceptor interface {
	// OnError is called when an error occurs during request execution
	OnError(ctx context.Context, req *http.Request, err error) error
}

// CombinedInterceptor combines all interceptor interfaces
type CombinedInterceptor interface {
	RequestInterceptor
	ResponseInterceptor
	ErrorInterceptor
}

// InterceptorMiddleware wraps interceptors as middleware
type InterceptorMiddleware struct {
	name         string
	interceptors []interface{}
}

// NewInterceptorMiddleware creates middleware from interceptors
func NewInterceptorMiddleware(name string, interceptors ...interface{}) *InterceptorMiddleware {
	return &InterceptorMiddleware{
		name:         name,
		interceptors: interceptors,
	}
}

// Name returns the middleware name
func (m *InterceptorMiddleware) Name() string {
	return m.name
}

// Execute implements the Middleware interface
func (m *InterceptorMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	// Execute request interceptors
	for _, interceptor := range m.interceptors {
		if reqInterceptor, ok := interceptor.(RequestInterceptor); ok {
			if err := reqInterceptor.BeforeRequest(ctx, req); err != nil {
				return nil, err
			}
		}
	}

	// Execute the next middleware/handler
	resp, err := next(ctx, req)

	// Handle errors with error interceptors
	if err != nil {
		for _, interceptor := range m.interceptors {
			if errInterceptor, ok := interceptor.(ErrorInterceptor); ok {
				if handledErr := errInterceptor.OnError(ctx, req, err); handledErr != nil {
					err = handledErr
				}
			}
		}
		return nil, err
	}

	// Execute response interceptors
	for _, interceptor := range m.interceptors {
		if respInterceptor, ok := interceptor.(ResponseInterceptor); ok {
			modifiedResp, respErr := respInterceptor.AfterResponse(ctx, req, resp)
			if respErr != nil {
				return nil, respErr
			}
			if modifiedResp != nil {
				resp = modifiedResp
			}
		}
	}

	return resp, nil
}
