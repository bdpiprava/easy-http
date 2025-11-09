package httpx

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig configures OpenTelemetry tracing behavior
type TracingConfig struct {
	TracerProvider   trace.TracerProvider
	Propagator       propagation.TextMapPropagator
	SpanNameFunc     func(*http.Request) string
	CaptureHeaders   bool
	SensitiveHeaders []string // Headers to exclude from capture
}

// TracingMiddleware implements distributed tracing using OpenTelemetry
type TracingMiddleware struct {
	config TracingConfig
	tracer trace.Tracer
}

// NewTracingMiddleware creates a new OpenTelemetry tracing middleware
func NewTracingMiddleware(config TracingConfig) *TracingMiddleware {
	if config.TracerProvider == nil {
		config.TracerProvider = otel.GetTracerProvider()
	}
	if config.Propagator == nil {
		config.Propagator = otel.GetTextMapPropagator()
	}
	if config.SpanNameFunc == nil {
		config.SpanNameFunc = defaultSpanName
	}
	if config.SensitiveHeaders == nil {
		config.SensitiveHeaders = []string{
			"Authorization",
			"Cookie",
			"Set-Cookie",
			"X-API-Key",
			"X-Auth-Token",
		}
	}

	tracer := config.TracerProvider.Tracer(
		"github.com/bdpiprava/easy-http/pkg/httpx",
		trace.WithInstrumentationVersion("1.0.0"),
	)

	return &TracingMiddleware{
		config: config,
		tracer: tracer,
	}
}

// Name returns the middleware name
func (m *TracingMiddleware) Name() string {
	return "tracing"
}

// Execute implements the Middleware interface
func (m *TracingMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	// Create span
	spanName := m.config.SpanNameFunc(req)
	ctx, span := m.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			m.httpAttributes(req)...,
		),
	)
	defer span.End()

	// Inject trace context into request headers
	m.config.Propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Update request context with span
	req = req.WithContext(ctx)

	// Execute request
	resp, err := next(ctx, req)

	// Record response or error
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Record response attributes
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
	)

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		span.SetAttributes(
			attribute.String("http.response.content_length", contentLength),
		)
	}

	// Set span status based on HTTP status code
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return resp, nil
}

// httpAttributes generates OpenTelemetry semantic convention attributes for HTTP
func (m *TracingMiddleware) httpAttributes(req *http.Request) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.scheme", req.URL.Scheme),
		attribute.String("http.host", req.Host),
		attribute.String("http.target", req.URL.Path),
	}

	if req.URL.RawQuery != "" {
		attrs = append(attrs, attribute.String("http.query", req.URL.RawQuery))
	}

	if userAgent := req.Header.Get("User-Agent"); userAgent != "" {
		attrs = append(attrs, attribute.String("http.user_agent", userAgent))
	}

	// Capture headers if configured (excluding sensitive ones)
	if m.config.CaptureHeaders {
		for name, values := range req.Header {
			if m.isSensitiveHeader(name) {
				continue
			}
			for i, value := range values {
				key := fmt.Sprintf("http.request.header.%s", name)
				if i > 0 {
					key = fmt.Sprintf("%s[%d]", key, i)
				}
				attrs = append(attrs, attribute.String(key, value))
			}
		}
	}

	if req.ContentLength > 0 {
		attrs = append(attrs, attribute.Int64("http.request.content_length", req.ContentLength))
	}

	return attrs
}

// isSensitiveHeader checks if a header should be excluded from tracing
func (m *TracingMiddleware) isSensitiveHeader(name string) bool {
	for _, sensitive := range m.config.SensitiveHeaders {
		if strings.EqualFold(name, sensitive) {
			return true
		}
	}
	return false
}

// defaultSpanName generates default span name from request
func defaultSpanName(req *http.Request) string {
	return fmt.Sprintf("HTTP %s", req.Method)
}
