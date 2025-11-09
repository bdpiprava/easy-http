package httpx

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Compressor interface for different compression algorithms
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	ContentEncoding() string
}

// CompressionConfig configures compression behavior
type CompressionConfig struct {
	Level              int      // Compression level (1-9, -1 for default)
	MinSizeBytes       int64    // Minimum size to compress (default: 1KB)
	CompressibleTypes  []string // Content types to compress
	EnableRequest      bool     // Compress request bodies
	EnableResponse     bool     // Decompress response bodies (add Accept-Encoding)
	PreferredEncodings []string // Preferred encodings in order (gzip, deflate, br)
}

// DefaultCompressionConfig returns sensible compression defaults
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Level:              gzip.DefaultCompression,
		MinSizeBytes:       1024, // 1KB
		CompressibleTypes:  []string{"application/json", "application/xml", "text/"},
		EnableRequest:      true,
		EnableResponse:     true,
		PreferredEncodings: []string{"gzip", "deflate"},
	}
}

// GzipCompressor implements gzip compression with writer pooling
type GzipCompressor struct {
	level int
	pool  sync.Pool
}

// NewGzipCompressor creates a new gzip compressor with writer pooling for performance
func NewGzipCompressor(level int) *GzipCompressor {
	if level == 0 {
		level = gzip.DefaultCompression
	}
	return &GzipCompressor{
		level: level,
		pool: sync.Pool{
			New: func() interface{} {
				// Create writer that will be Reset() before use
				w, _ := gzip.NewWriterLevel(&bytes.Buffer{}, level)
				return w
			},
		},
	}
}

// Compress compresses data using gzip
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	writer := c.pool.Get().(*gzip.Writer)
	writer.Reset(buf)
	defer func() {
		writer.Close()
		c.pool.Put(writer)
	}()

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decompress decompresses gzip data
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// ContentEncoding returns the encoding name
func (c *GzipCompressor) ContentEncoding() string {
	return "gzip"
}

// DeflateCompressor implements deflate compression
type DeflateCompressor struct {
	level int
	pool  sync.Pool
}

// NewDeflateCompressor creates a new deflate compressor
func NewDeflateCompressor(level int) *DeflateCompressor {
	if level == 0 {
		level = zlib.DefaultCompression
	}
	return &DeflateCompressor{
		level: level,
		pool: sync.Pool{
			New: func() interface{} {
				buf := &bytes.Buffer{}
				w, _ := zlib.NewWriterLevel(buf, level)
				return w
			},
		},
	}
}

// Compress compresses data using deflate
func (c *DeflateCompressor) Compress(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	writer := c.pool.Get().(*zlib.Writer)
	writer.Reset(buf)
	defer func() {
		writer.Close()
		c.pool.Put(writer)
	}()

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decompress decompresses deflate data
func (c *DeflateCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// ContentEncoding returns the encoding name
func (c *DeflateCompressor) ContentEncoding() string {
	return "deflate"
}

// CompressionMiddleware handles automatic compression/decompression
type CompressionMiddleware struct {
	config      CompressionConfig
	compressors map[string]Compressor
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(config CompressionConfig) *CompressionMiddleware {
	// Set defaults if not configured
	if config.MinSizeBytes == 0 {
		config.MinSizeBytes = 1024
	}
	if config.Level == 0 {
		config.Level = gzip.DefaultCompression
	}
	if len(config.PreferredEncodings) == 0 {
		config.PreferredEncodings = []string{"gzip", "deflate"}
	}
	if len(config.CompressibleTypes) == 0 {
		config.CompressibleTypes = []string{"application/json", "application/xml", "text/"}
	}

	compressors := make(map[string]Compressor)
	compressors["gzip"] = NewGzipCompressor(config.Level)
	compressors["deflate"] = NewDeflateCompressor(config.Level)

	return &CompressionMiddleware{
		config:      config,
		compressors: compressors,
	}
}

// Name returns the middleware name
func (m *CompressionMiddleware) Name() string {
	return "compression"
}

// Execute implements the Middleware interface
func (m *CompressionMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	// Add Accept-Encoding header if response decompression is enabled
	if m.config.EnableResponse && len(m.config.PreferredEncodings) > 0 {
		acceptEncoding := strings.Join(m.config.PreferredEncodings, ", ")
		if req.Header.Get("Accept-Encoding") == "" {
			req.Header.Set("Accept-Encoding", acceptEncoding)
		}
	} else if !m.config.EnableResponse {
		// Explicitly disable Go's automatic compression by setting Accept-Encoding to empty
		// This prevents http.Transport from automatically adding "Accept-Encoding: gzip"
		req.Header.Set("Accept-Encoding", "identity")
	}

	// Compress request body if enabled
	if m.config.EnableRequest && req.Body != nil && req.ContentLength > m.config.MinSizeBytes {
		contentType := req.Header.Get("Content-Type")
		if m.shouldCompress(contentType) {
			if err := m.compressRequest(req); err != nil {
				// Log error but continue with uncompressed request
				// Compression failure shouldn't break the request
				_ = err
			}
		}
	}

	// Execute request
	resp, err := next(ctx, req)
	if err != nil {
		return nil, err
	}

	// Decompress response body if it's encoded
	if m.config.EnableResponse && resp.Header.Get("Content-Encoding") != "" {
		if err := m.decompressResponse(resp); err != nil {
			// Return error if decompression fails - this is a real problem
			return resp, &HTTPError{
				Type:     ErrorTypeMiddleware,
				Message:  fmt.Sprintf("failed to decompress response: %v", err),
				Cause:    err,
				Response: resp,
			}
		}
	}

	return resp, nil
}

// compressRequest compresses the request body
func (m *CompressionMiddleware) compressRequest(req *http.Request) error {
	// Read request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	req.Body.Close()

	// Get preferred compressor
	encoding := m.config.PreferredEncodings[0]
	compressor, ok := m.compressors[encoding]
	if !ok {
		// No compressor available, restore original body
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return fmt.Errorf("compressor not found: %s", encoding)
	}

	// Compress data
	compressed, err := compressor.Compress(bodyBytes)
	if err != nil {
		// Restore original body on compression failure
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return err
	}

	// Only use compression if it actually reduces size
	if len(compressed) < len(bodyBytes) {
		req.Body = io.NopCloser(bytes.NewReader(compressed))
		req.ContentLength = int64(len(compressed))
		req.Header.Set("Content-Encoding", compressor.ContentEncoding())
	} else {
		// Compression didn't help, use original
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return nil
}

// decompressResponse decompresses the response body
func (m *CompressionMiddleware) decompressResponse(resp *http.Response) error {
	encoding := strings.TrimSpace(strings.ToLower(resp.Header.Get("Content-Encoding")))
	if encoding == "" {
		return nil
	}

	// Handle multiple encodings (e.g., "gzip, deflate")
	encodings := strings.Split(encoding, ",")
	if len(encodings) == 0 {
		return nil
	}

	// Use the first (most recent) encoding
	encoding = strings.TrimSpace(encodings[0])

	compressor, ok := m.compressors[encoding]
	if !ok {
		// Unsupported encoding, return as-is
		// This is not an error - the caller can still read the body
		return nil
	}

	// Read compressed body
	compressed, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Decompress
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		return err
	}

	// Replace body with decompressed data
	resp.Body = io.NopCloser(bytes.NewReader(decompressed))
	resp.ContentLength = int64(len(decompressed))
	resp.Header.Del("Content-Encoding") // Remove encoding header since we decompressed

	return nil
}

// shouldCompress checks if content type should be compressed
func (m *CompressionMiddleware) shouldCompress(contentType string) bool {
	if contentType == "" {
		return false
	}

	// Remove charset and other parameters
	contentType = strings.ToLower(strings.Split(contentType, ";")[0])
	contentType = strings.TrimSpace(contentType)

	for _, compressible := range m.config.CompressibleTypes {
		// Support prefix matching (e.g., "text/" matches "text/html", "text/plain", etc.)
		if strings.HasPrefix(contentType, strings.ToLower(compressible)) {
			return true
		}
	}

	return false
}
