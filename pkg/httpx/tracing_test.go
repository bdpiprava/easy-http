package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestNewTracingMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     httpx.TracingConfig
		wantName   string
		wantNotNil bool
	}{
		{
			name: "creates middleware with full configuration",
			config: httpx.TracingConfig{
				TracerProvider:   sdktrace.NewTracerProvider(),
				Propagator:       propagation.TraceContext{},
				SpanNameFunc:     func(_ *http.Request) string { return "custom-span" },
				CaptureHeaders:   true,
				SensitiveHeaders: []string{"X-Custom-Auth"},
			},
			wantName:   "tracing",
			wantNotNil: true,
		},
		{
			name:       "creates middleware with default configuration",
			config:     httpx.TracingConfig{},
			wantName:   "tracing",
			wantNotNil: true,
		},
		{
			name: "creates middleware with custom tracer provider",
			config: httpx.TracingConfig{
				TracerProvider: sdktrace.NewTracerProvider(),
			},
			wantName:   "tracing",
			wantNotNil: true,
		},
		{
			name: "creates middleware with custom propagator",
			config: httpx.TracingConfig{
				Propagator: propagation.NewCompositeTextMapPropagator(
					propagation.TraceContext{},
					propagation.Baggage{},
				),
			},
			wantName:   "tracing",
			wantNotNil: true,
		},
		{
			name: "creates middleware with custom span name function",
			config: httpx.TracingConfig{
				SpanNameFunc: func(req *http.Request) string {
					return req.Method + " " + req.URL.Path
				},
			},
			wantName:   "tracing",
			wantNotNil: true,
		},
		{
			name: "creates middleware with header capture enabled",
			config: httpx.TracingConfig{
				CaptureHeaders:   true,
				SensitiveHeaders: []string{"Authorization", "X-API-Key"},
			},
			wantName:   "tracing",
			wantNotNil: true,
		},
		{
			name: "creates middleware with empty sensitive headers list",
			config: httpx.TracingConfig{
				CaptureHeaders:   true,
				SensitiveHeaders: []string{},
			},
			wantName:   "tracing",
			wantNotNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewTracingMiddleware(tc.config)

			if tc.wantNotNil {
				assert.NotNil(t, got)
				assert.Equal(t, tc.wantName, got.Name())
			}
		})
	}
}

func TestTracingMiddleware_Name(t *testing.T) {
	t.Parallel()

	t.Run("returns tracing as middleware name", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewTracingMiddleware(httpx.TracingConfig{})

		got := subject.Name()

		assert.Equal(t, "tracing", got)
	})
}

func TestTracingMiddleware_Execute_SpanCreation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		method       string
		url          string
		config       httpx.TracingConfig
		wantSpanName string
		wantSpanKind string
	}{
		{
			name:         "creates span with default span name for GET request",
			method:       http.MethodGet,
			url:          "/users",
			config:       httpx.TracingConfig{},
			wantSpanName: "HTTP GET",
			wantSpanKind: "client",
		},
		{
			name:         "creates span with default span name for POST request",
			method:       http.MethodPost,
			url:          "/orders",
			config:       httpx.TracingConfig{},
			wantSpanName: "HTTP POST",
			wantSpanKind: "client",
		},
		{
			name:   "creates span with custom span name function",
			method: http.MethodGet,
			url:    "/users/123",
			config: httpx.TracingConfig{
				SpanNameFunc: func(req *http.Request) string {
					return req.Method + " " + req.URL.Path
				},
			},
			wantSpanName: "GET /users/123",
			wantSpanKind: "client",
		},
		{
			name:         "creates span for PUT request",
			method:       http.MethodPut,
			url:          "/items/456",
			config:       httpx.TracingConfig{},
			wantSpanName: "HTTP PUT",
			wantSpanKind: "client",
		},
		{
			name:         "creates span for DELETE request",
			method:       http.MethodDelete,
			url:          "/resources/789",
			config:       httpx.TracingConfig{},
			wantSpanName: "HTTP DELETE",
			wantSpanKind: "client",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup test span exporter
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			tc.config.TracerProvider = tp

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			defer server.Close()

			middleware := httpx.NewTracingMiddleware(tc.config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(tc.method, httpx.WithPath(tc.url))
			resp, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Verify span was created
			spans := exporter.GetSpans()
			require.Len(t, spans, 1)
			assert.Equal(t, tc.wantSpanName, spans[0].Name)
		})
	}
}

func TestTracingMiddleware_Execute_SpanAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		method            string
		path              string
		query             string
		headers           map[string]string
		contentLength     int64
		captureHeaders    bool
		wantHTTPMethod    bool
		wantHTTPURL       bool
		wantHTTPHost      bool
		wantHTTPTarget    bool
		wantHTTPQuery     bool
		wantUserAgent     bool
		wantContentLength bool
	}{
		{
			name:           "records basic HTTP attributes",
			method:         http.MethodGet,
			path:           "/api/users",
			captureHeaders: false,
			wantHTTPMethod: true,
			wantHTTPURL:    true,
			wantHTTPHost:   true,
			wantHTTPTarget: true,
		},
		{
			name:           "records query parameters as attribute",
			method:         http.MethodGet,
			path:           "/api/search",
			query:          "q=test&limit=10",
			captureHeaders: false,
			wantHTTPMethod: true,
			wantHTTPURL:    true,
			wantHTTPQuery:  true,
		},
		{
			name:           "records user agent when present",
			method:         http.MethodGet,
			path:           "/api/data",
			headers:        map[string]string{"User-Agent": "TestClient/1.0"},
			captureHeaders: false,
			wantHTTPMethod: true,
			wantUserAgent:  true,
		},
		{
			name:              "records content length for requests with body",
			method:            http.MethodPost,
			path:              "/api/create",
			contentLength:     1024,
			captureHeaders:    false,
			wantHTTPMethod:    true,
			wantContentLength: true,
		},
		{
			name:           "captures non-sensitive headers when enabled",
			method:         http.MethodGet,
			path:           "/api/test",
			headers:        map[string]string{"X-Request-ID": "12345", "X-Custom-Header": "value"},
			captureHeaders: true,
			wantHTTPMethod: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			config := httpx.TracingConfig{
				TracerProvider: tp,
				CaptureHeaders: tc.captureHeaders,
			}

			middleware := httpx.NewTracingMiddleware(config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			reqOpts := []httpx.RequestOption{httpx.WithPath(tc.path)}

			// Add query parameters if present
			if tc.query != "" {
				// Parse query string and add each parameter
				queryParams, _ := url.ParseQuery(tc.query)
				for key, values := range queryParams {
					for _, value := range values {
						reqOpts = append(reqOpts, httpx.WithQueryParam(key, value))
					}
				}
			}
			for k, v := range tc.headers {
				reqOpts = append(reqOpts, httpx.WithHeader(k, v))
			}

			req := httpx.NewRequest(tc.method, reqOpts...)
			_, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)

			attrs := spans[0].Attributes
			attrMap := make(map[string]string)
			for _, attr := range attrs {
				attrMap[string(attr.Key)] = attr.Value.AsString()
			}

			if tc.wantHTTPMethod {
				assert.Contains(t, attrMap, "http.method")
				assert.Equal(t, tc.method, attrMap["http.method"])
			}

			if tc.wantHTTPURL {
				assert.Contains(t, attrMap, "http.url")
			}

			if tc.wantHTTPHost {
				assert.Contains(t, attrMap, "http.host")
			}

			if tc.wantHTTPTarget {
				assert.Contains(t, attrMap, "http.target")
				assert.Equal(t, tc.path, attrMap["http.target"])
			}

			if tc.wantHTTPQuery {
				assert.Contains(t, attrMap, "http.query")
				// Query params may be in different order, so check that all expected params are present
				actualQuery := attrMap["http.query"]
				assert.Contains(t, actualQuery, "q=test", "query should contain q=test")
				assert.Contains(t, actualQuery, "limit=10", "query should contain limit=10")
			}

			if tc.wantUserAgent {
				assert.Contains(t, attrMap, "http.user_agent")
			}
		})
	}
}

func TestTracingMiddleware_Execute_TraceContextPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		propagator      propagation.TextMapPropagator
		wantTraceParent bool
	}{
		{
			name:            "injects W3C Trace Context headers",
			propagator:      propagation.TraceContext{},
			wantTraceParent: true,
		},
		{
			name: "injects trace context with composite propagator",
			propagator: propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
			wantTraceParent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			var receivedTraceparent string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedTraceparent = r.Header.Get("Traceparent")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			config := httpx.TracingConfig{
				TracerProvider: tp,
				Propagator:     tc.propagator,
			}

			middleware := httpx.NewTracingMiddleware(config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
			_, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)

			if tc.wantTraceParent {
				assert.NotEmpty(t, receivedTraceparent)
			}
		})
	}
}

func TestTracingMiddleware_Execute_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		simulateError  bool
		statusCode     int
		responseBody   string
		wantSpanStatus string
	}{
		{
			name:           "records error when network error occurs",
			simulateError:  true,
			wantSpanStatus: "Error",
		},
		{
			name:           "sets error status for 4xx client errors",
			statusCode:     http.StatusBadRequest,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Error",
		},
		{
			name:           "sets error status for 404 not found",
			statusCode:     http.StatusNotFound,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Error",
		},
		{
			name:           "sets error status for 5xx server errors",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Error",
		},
		{
			name:           "sets error status for 503 service unavailable",
			statusCode:     http.StatusServiceUnavailable,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Error",
		},
		{
			name:           "sets ok status for 2xx success",
			statusCode:     http.StatusOK,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Ok",
		},
		{
			name:           "sets ok status for 201 created",
			statusCode:     http.StatusCreated,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Ok",
		},
		{
			name:           "sets ok status for 204 no content",
			statusCode:     http.StatusNoContent,
			responseBody:   "",
			wantSpanStatus: "Ok",
		},
		{
			name:           "sets ok status for 3xx redirect",
			statusCode:     http.StatusMovedPermanently,
			responseBody:   `{"status":"response"}`,
			wantSpanStatus: "Ok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			if !tc.simulateError {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if tc.responseBody != "" {
						w.Header().Set("Content-Type", "application/json")
					}
					w.WriteHeader(tc.statusCode)
					if tc.responseBody != "" {
						_, _ = w.Write([]byte(tc.responseBody))
					}
				}))
				defer server.Close()

				config := httpx.TracingConfig{
					TracerProvider: tp,
				}

				middleware := httpx.NewTracingMiddleware(config)
				client := httpx.NewClientWithConfig(
					httpx.WithClientDefaultBaseURL(server.URL),
					httpx.WithClientMiddleware(middleware),
				)

				req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))

				// Use empty struct for responses without body
				type EmptyResponse struct{}
				var resp *httpx.Response
				var err error
				if tc.responseBody == "" {
					resp, err = client.Execute(*req, EmptyResponse{})
				} else {
					resp, err = client.Execute(*req, map[string]any{})
				}

				require.NoError(t, err)
				assert.Equal(t, tc.statusCode, resp.StatusCode)

				spans := exporter.GetSpans()
				require.Len(t, spans, 1)
				assert.Equal(t, tc.wantSpanStatus, spans[0].Status.Code.String())
			} else {
				// Test network error by using invalid URL
				config := httpx.TracingConfig{
					TracerProvider: tp,
				}

				middleware := httpx.NewTracingMiddleware(config)
				client := httpx.NewClientWithConfig(
					httpx.WithClientDefaultBaseURL("http://invalid-host-that-does-not-exist-12345.local"),
					httpx.WithClientMiddleware(middleware),
				)

				req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
				_, err := client.Execute(*req, map[string]any{})

				require.Error(t, err)

				spans := exporter.GetSpans()
				require.Len(t, spans, 1)
				assert.Equal(t, tc.wantSpanStatus, spans[0].Status.Code.String())
			}
		})
	}
}

func TestTracingMiddleware_Execute_SensitiveHeaderFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		headers          map[string]string
		sensitiveHeaders []string
		captureHeaders   bool
		wantCaptured     []string
		wantFiltered     []string
	}{
		{
			name: "filters default sensitive headers",
			headers: map[string]string{
				"Authorization": "Bearer secret-token",
				"X-Request-ID":  "12345",
				"Cookie":        "session=abc123",
			},
			captureHeaders: true,
			wantCaptured:   []string{"X-Request-ID"},
			wantFiltered:   []string{"Authorization", "Cookie"},
		},
		{
			name: "filters custom sensitive headers",
			headers: map[string]string{
				"X-API-Key":    "secret-key",
				"X-Request-ID": "67890",
				"X-Auth-Token": "token123",
			},
			sensitiveHeaders: []string{"X-API-Key", "X-Auth-Token"},
			captureHeaders:   true,
			wantCaptured:     []string{"X-Request-ID"},
			wantFiltered:     []string{"X-API-Key", "X-Auth-Token"},
		},
		{
			name: "filters Set-Cookie header",
			headers: map[string]string{
				"Set-Cookie":   "sessionid=xyz789",
				"Content-Type": "application/json",
			},
			captureHeaders: true,
			wantCaptured:   []string{"Content-Type"},
			wantFiltered:   []string{"Set-Cookie"},
		},
		{
			name: "does not capture headers when disabled",
			headers: map[string]string{
				"X-Request-ID": "abc",
				"X-Custom":     "value",
			},
			captureHeaders: false,
			wantCaptured:   []string{},
			wantFiltered:   []string{"X-Request-ID", "X-Custom"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			config := httpx.TracingConfig{
				TracerProvider:   tp,
				CaptureHeaders:   tc.captureHeaders,
				SensitiveHeaders: tc.sensitiveHeaders,
			}

			middleware := httpx.NewTracingMiddleware(config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			reqOpts := []httpx.RequestOption{httpx.WithPath("/test")}
			for k, v := range tc.headers {
				reqOpts = append(reqOpts, httpx.WithHeader(k, v))
			}

			req := httpx.NewRequest(http.MethodGet, reqOpts...)
			_, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)

			attrs := spans[0].Attributes
			attrMap := make(map[string]bool)
			for _, attr := range attrs {
				attrMap[string(attr.Key)] = true
			}

			for _, captured := range tc.wantCaptured {
				headerKey := "http.request.header." + captured
				assert.Contains(t, attrMap, headerKey, "expected header %s to be captured", captured)
			}

			for _, filtered := range tc.wantFiltered {
				headerKey := "http.request.header." + filtered
				assert.NotContains(t, attrMap, headerKey, "expected header %s to be filtered", filtered)
			}
		})
	}
}

func TestTracingMiddleware_Execute_ResponseAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		statusCode            int
		responseBody          string
		wantStatusCodeAttr    bool
		wantContentLengthAttr bool
	}{
		{
			name:                  "records status code and content length",
			statusCode:            http.StatusOK,
			responseBody:          `{"test":"response"}`,
			wantStatusCodeAttr:    true,
			wantContentLengthAttr: true,
		},
		{
			name:                  "records status code without content length",
			statusCode:            http.StatusNoContent,
			responseBody:          "",
			wantStatusCodeAttr:    true,
			wantContentLengthAttr: false,
		},
		{
			name:                  "records status code for error responses",
			statusCode:            http.StatusInternalServerError,
			responseBody:          `{"error":"internal server error"}`,
			wantStatusCodeAttr:    true,
			wantContentLengthAttr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.responseBody != "" {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tc.statusCode)
				if tc.responseBody != "" {
					_, _ = w.Write([]byte(tc.responseBody))
				}
			}))
			defer server.Close()

			config := httpx.TracingConfig{
				TracerProvider: tp,
			}

			middleware := httpx.NewTracingMiddleware(config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))

			// Use empty struct for responses without body
			type EmptyResponse struct{}
			var resp *httpx.Response
			var err error
			if tc.responseBody == "" {
				resp, err = client.Execute(*req, EmptyResponse{})
			} else {
				resp, err = client.Execute(*req, map[string]any{})
			}

			require.NoError(t, err)
			assert.Equal(t, tc.statusCode, resp.StatusCode)

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)

			attrs := spans[0].Attributes
			attrMap := make(map[string]string)
			for _, attr := range attrs {
				attrMap[string(attr.Key)] = attr.Value.AsString()
			}

			if tc.wantStatusCodeAttr {
				assert.Contains(t, attrMap, "http.status_code")
			}

			if tc.wantContentLengthAttr {
				assert.Contains(t, attrMap, "http.response.content_length")
				// Content-Length is set automatically by Go's http server based on body size
			} else {
				assert.NotContains(t, attrMap, "http.response.content_length")
			}
		})
	}
}

func TestTracingMiddleware_Execute_ContextPropagation(t *testing.T) {
	t.Parallel()

	t.Run("propagates existing span context from parent", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(exporter),
		)
		otel.SetTracerProvider(tp)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		config := httpx.TracingConfig{
			TracerProvider: tp,
		}

		middleware := httpx.NewTracingMiddleware(config)
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware),
		)

		// Create parent span
		tracer := tp.Tracer("test")
		ctx, parentSpan := tracer.Start(context.Background(), "parent-operation")

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"), httpx.WithContext(ctx))
		_, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)

		// End parent span before checking exporter
		parentSpan.End()

		// Should have 2 spans: parent and child (HTTP request)
		spans := exporter.GetSpans()
		require.Len(t, spans, 2)

		// Find the child span (HTTP request span)
		var childSpan *tracetest.SpanStub
		for i := range spans {
			if spans[i].Name == "HTTP GET" {
				childSpan = &spans[i]
				break
			}
		}

		require.NotNil(t, childSpan)
		// Verify child span has parent span ID
		assert.Equal(t, parentSpan.SpanContext().SpanID(), childSpan.Parent.SpanID())
	})
}

func TestTracingMiddleware_Integration(t *testing.T) {
	t.Parallel()

	t.Run("end-to-end tracing with client configuration", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(exporter),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceparent := r.Header.Get("Traceparent")
			w.Header().Set("X-Received-Traceparent", traceparent)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"traced":true}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientTracing(httpx.TracingConfig{
				TracerProvider: tp,
				Propagator:     propagation.TraceContext{},
				CaptureHeaders: true,
			}),
		)

		req := httpx.NewRequest(http.MethodPost,
			httpx.WithPath("/api/orders"),
			httpx.WithJSONBody(map[string]string{"item": "test"}))

		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify traceparent was injected
		assert.NotEmpty(t, resp.Header().Get("X-Received-Traceparent"))

		// Verify span was created
		spans := exporter.GetSpans()
		require.Len(t, spans, 1)
		assert.Equal(t, "HTTP POST", spans[0].Name)
		assert.Equal(t, "Ok", spans[0].Status.Code.String())
	})

	t.Run("tracing with default configuration", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(exporter),
		)
		otel.SetTracerProvider(tp)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultTracing(),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		_, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)

		spans := exporter.GetSpans()
		assert.NotEmpty(t, spans)
	})
}

func TestDefaultSpanName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		url      string
		wantName string
	}{
		{
			name:     "generates span name for GET request",
			method:   http.MethodGet,
			url:      "http://example.com/users",
			wantName: "HTTP GET",
		},
		{
			name:     "generates span name for POST request",
			method:   http.MethodPost,
			url:      "http://example.com/orders",
			wantName: "HTTP POST",
		},
		{
			name:     "generates span name for PUT request",
			method:   http.MethodPut,
			url:      "http://example.com/items/123",
			wantName: "HTTP PUT",
		},
		{
			name:     "generates span name for DELETE request",
			method:   http.MethodDelete,
			url:      "http://example.com/resources/456",
			wantName: "HTTP DELETE",
		},
		{
			name:     "generates span name for PATCH request",
			method:   http.MethodPatch,
			url:      "http://example.com/data",
			wantName: "HTTP PATCH",
		},
		{
			name:     "generates span name for HEAD request",
			method:   http.MethodHead,
			url:      "http://example.com/status",
			wantName: "HTTP HEAD",
		},
		{
			name:     "generates span name for OPTIONS request",
			method:   http.MethodOptions,
			url:      "http://example.com/api",
			wantName: "HTTP OPTIONS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			middleware := httpx.NewTracingMiddleware(httpx.TracingConfig{
				TracerProvider: tp,
			})

			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(tc.method, httpx.WithPath("/test"))
			_, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)
			assert.Equal(t, tc.wantName, spans[0].Name)
		})
	}
}

func TestTracingMiddleware_Execute_NetworkErrors(t *testing.T) {
	t.Parallel()

	t.Run("records span with error for connection refused", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(exporter),
		)

		middleware := httpx.NewTracingMiddleware(httpx.TracingConfig{
			TracerProvider: tp,
		})

		// Use an invalid port that's likely not in use
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL("http://localhost:9"),
			httpx.WithClientMiddleware(middleware),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		_, err := client.Execute(*req, map[string]any{})

		require.Error(t, err)

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)
		assert.Equal(t, "Error", spans[0].Status.Code.String())
		assert.NotEmpty(t, spans[0].Status.Description)
	})

	t.Run("records span with error for context cancellation", func(t *testing.T) {
		t.Parallel()

		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(exporter),
		)

		// Create a slow server
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			// This will never complete
			<-make(chan struct{})
		}))
		defer server.Close()

		middleware := httpx.NewTracingMiddleware(httpx.TracingConfig{
			TracerProvider: tp,
		})

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientMiddleware(middleware),
		)

		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"), httpx.WithContext(ctx))
		_, err := client.Execute(*req, map[string]any{})

		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)
		assert.Equal(t, "Error", spans[0].Status.Code.String())
	})
}
