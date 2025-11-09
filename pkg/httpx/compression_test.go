package httpx_test

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestDefaultCompressionConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns default compression configuration", func(t *testing.T) {
		t.Parallel()

		got := httpx.DefaultCompressionConfig()

		assert.Equal(t, gzip.DefaultCompression, got.Level)
		assert.Equal(t, int64(1024), got.MinSizeBytes)
		assert.True(t, got.EnableRequest)
		assert.True(t, got.EnableResponse)
		assert.Equal(t, []string{"gzip", "deflate"}, got.PreferredEncodings)
		assert.Contains(t, got.CompressibleTypes, "application/json")
		assert.Contains(t, got.CompressibleTypes, "application/xml")
	})
}

func TestNewGzipCompressor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		level      int
		wantNotNil bool
	}{
		{
			name:       "creates compressor with positive level",
			level:      gzip.BestCompression,
			wantNotNil: true,
		},
		{
			name:       "creates compressor with default level when zero",
			level:      0,
			wantNotNil: true,
		},
		{
			name:       "creates compressor with custom level",
			level:      5,
			wantNotNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewGzipCompressor(tc.level)

			if tc.wantNotNil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestGzipCompressor_Compress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "compresses simple text data",
			data:    []byte("Hello, World!"),
			wantErr: false,
		},
		{
			name:    "compresses JSON data",
			data:    []byte(`{"key":"value","nested":{"data":"test"}}`),
			wantErr: false,
		},
		{
			name:    "compresses large data",
			data:    bytes.Repeat([]byte("large data payload "), 1000),
			wantErr: false,
		},
		{
			name:    "compresses empty data",
			data:    []byte{},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewGzipCompressor(gzip.DefaultCompression)

			got, gotErr := subject.Compress(tc.data)

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
				// Compressed data should be different from original (unless empty)
				if len(tc.data) > 0 {
					assert.NotEqual(t, tc.data, got)
				}
			}
		})
	}
}

func TestGzipCompressor_Decompress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "decompresses valid gzip data",
			data:    []byte("Hello, World!"),
			wantErr: false,
		},
		{
			name:    "decompresses JSON data",
			data:    []byte(`{"key":"value","test":123}`),
			wantErr: false,
		},
		{
			name:    "decompresses large data",
			data:    bytes.Repeat([]byte("test data "), 500),
			wantErr: false,
		},
		{
			name:    "returns error for invalid gzip data",
			data:    []byte("not gzip data"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewGzipCompressor(gzip.DefaultCompression)

			// Compress first if not testing error case
			var compressed []byte
			if !tc.wantErr {
				var err error
				compressed, err = subject.Compress(tc.data)
				require.NoError(t, err)
			} else {
				compressed = tc.data
			}

			got, gotErr := subject.Decompress(compressed)

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.data, got)
			}
		})
	}
}

func TestGzipCompressor_ContentEncoding(t *testing.T) {
	t.Parallel()

	t.Run("returns gzip as content encoding", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewGzipCompressor(gzip.DefaultCompression)

		got := subject.ContentEncoding()

		assert.Equal(t, "gzip", got)
	})
}

func TestGzipCompressor_Concurrency(t *testing.T) {
	t.Parallel()

	t.Run("handles concurrent compression safely", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewGzipCompressor(gzip.DefaultCompression)
		testData := []byte("concurrent test data")

		var wg sync.WaitGroup
		numGoroutines := 100

		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				compressed, err := subject.Compress(testData)
				assert.NoError(t, err)
				decompressed, err := subject.Decompress(compressed)
				assert.NoError(t, err)
				assert.Equal(t, testData, decompressed)
			}()
		}

		wg.Wait()
	})
}

func TestNewDeflateCompressor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		level      int
		wantNotNil bool
	}{
		{
			name:       "creates compressor with positive level",
			level:      zlib.BestCompression,
			wantNotNil: true,
		},
		{
			name:       "creates compressor with default level when zero",
			level:      0,
			wantNotNil: true,
		},
		{
			name:       "creates compressor with custom level",
			level:      6,
			wantNotNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewDeflateCompressor(tc.level)

			if tc.wantNotNil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestDeflateCompressor_Compress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "compresses simple text data",
			data:    []byte("Hello, Deflate!"),
			wantErr: false,
		},
		{
			name:    "compresses JSON data",
			data:    []byte(`{"deflate":"compression","works":true}`),
			wantErr: false,
		},
		{
			name:    "compresses large data",
			data:    bytes.Repeat([]byte("deflate test "), 1000),
			wantErr: false,
		},
		{
			name:    "compresses empty data",
			data:    []byte{},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewDeflateCompressor(zlib.DefaultCompression)

			got, gotErr := subject.Compress(tc.data)

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestDeflateCompressor_Decompress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "decompresses valid deflate data",
			data:    []byte("Hello, Deflate Decompression!"),
			wantErr: false,
		},
		{
			name:    "decompresses JSON data",
			data:    []byte(`{"test":"deflate","value":456}`),
			wantErr: false,
		},
		{
			name:    "decompresses large data",
			data:    bytes.Repeat([]byte("deflate decompression "), 500),
			wantErr: false,
		},
		{
			name:    "returns error for invalid deflate data",
			data:    []byte("not deflate data"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewDeflateCompressor(zlib.DefaultCompression)

			// Compress first if not testing error case
			var compressed []byte
			if !tc.wantErr {
				var err error
				compressed, err = subject.Compress(tc.data)
				require.NoError(t, err)
			} else {
				compressed = tc.data
			}

			got, gotErr := subject.Decompress(compressed)

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.data, got)
			}
		})
	}
}

func TestDeflateCompressor_ContentEncoding(t *testing.T) {
	t.Parallel()

	t.Run("returns deflate as content encoding", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewDeflateCompressor(zlib.DefaultCompression)

		got := subject.ContentEncoding()

		assert.Equal(t, "deflate", got)
	})
}

func TestNewCompressionMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		config           httpx.CompressionConfig
		wantMinSize      int64
		wantLevel        int
		wantEncodingsLen int
		wantTypesLen     int
	}{
		{
			name: "creates middleware with custom config",
			config: httpx.CompressionConfig{
				Level:              gzip.BestCompression,
				MinSizeBytes:       2048,
				CompressibleTypes:  []string{"application/json"},
				EnableRequest:      true,
				EnableResponse:     true,
				PreferredEncodings: []string{"gzip"},
			},
			wantMinSize:      2048,
			wantLevel:        gzip.BestCompression,
			wantEncodingsLen: 1,
			wantTypesLen:     1,
		},
		{
			name:   "applies defaults when not specified",
			config: httpx.CompressionConfig{
				// Empty config
			},
			wantMinSize:      1024,
			wantLevel:        gzip.DefaultCompression,
			wantEncodingsLen: 2,
			wantTypesLen:     3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewCompressionMiddleware(tc.config)

			assert.NotNil(t, got)
			assert.Equal(t, "compression", got.Name())
		})
	}
}

func TestCompressionMiddleware_Name(t *testing.T) {
	t.Parallel()

	t.Run("returns compression as middleware name", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewCompressionMiddleware(httpx.DefaultCompressionConfig())

		got := subject.Name()

		assert.Equal(t, "compression", got)
	})
}

func TestCompressionMiddleware_Execute_AcceptEncodingHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		config             httpx.CompressionConfig
		existingHeader     string
		wantAcceptEncoding string
		wantHeaderSet      bool
	}{
		{
			name: "adds Accept-Encoding header when response decompression enabled",
			config: httpx.CompressionConfig{
				EnableResponse:     true,
				PreferredEncodings: []string{"gzip", "deflate"},
			},
			existingHeader:     "",
			wantAcceptEncoding: "gzip, deflate",
			wantHeaderSet:      true,
		},
		{
			name: "does not override existing Accept-Encoding header",
			config: httpx.CompressionConfig{
				EnableResponse:     true,
				PreferredEncodings: []string{"gzip"},
			},
			existingHeader:     "br",
			wantAcceptEncoding: "br",
			wantHeaderSet:      true,
		},
		{
			name: "sets identity encoding when response decompression disabled",
			config: httpx.CompressionConfig{
				EnableResponse:     false,
				PreferredEncodings: []string{"gzip"},
			},
			existingHeader:     "",
			wantAcceptEncoding: "identity",
			wantHeaderSet:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				acceptEncoding := r.Header.Get("Accept-Encoding")
				w.Header().Set("X-Received-Accept-Encoding", acceptEncoding)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			defer server.Close()

			middleware := httpx.NewCompressionMiddleware(tc.config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
			if tc.existingHeader != "" {
				req = httpx.NewRequest(http.MethodGet,
					httpx.WithPath("/test"),
					httpx.WithHeader("Accept-Encoding", tc.existingHeader))
			}

			resp, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)
			receivedHeader := resp.Header().Get("X-Received-Accept-Encoding")
			if tc.wantHeaderSet {
				assert.Equal(t, tc.wantAcceptEncoding, receivedHeader)
			} else {
				assert.Empty(t, receivedHeader)
			}
		})
	}
}

func TestCompressionMiddleware_Execute_RequestCompression(t *testing.T) {
	t.Parallel()

	largeJSON := `{"data":"` + strings.Repeat("x", 2000) + `"}`

	tests := []struct {
		name           string
		config         httpx.CompressionConfig
		requestBody    string
		contentType    string
		wantCompressed bool
	}{
		{
			name: "compresses large JSON request",
			config: httpx.CompressionConfig{
				EnableRequest:      true,
				MinSizeBytes:       1024,
				CompressibleTypes:  []string{"application/json"},
				PreferredEncodings: []string{"gzip"},
			},
			requestBody:    largeJSON,
			contentType:    "application/json",
			wantCompressed: true,
		},
		{
			name: "does not compress small request below threshold",
			config: httpx.CompressionConfig{
				EnableRequest:      true,
				MinSizeBytes:       5000,
				CompressibleTypes:  []string{"application/json"},
				PreferredEncodings: []string{"gzip"},
			},
			requestBody:    largeJSON,
			contentType:    "application/json",
			wantCompressed: false,
		},
		{
			name: "does not compress non-compressible content type",
			config: httpx.CompressionConfig{
				EnableRequest:      true,
				MinSizeBytes:       1024,
				CompressibleTypes:  []string{"application/json"},
				PreferredEncodings: []string{"gzip"},
			},
			requestBody:    largeJSON,
			contentType:    "image/png",
			wantCompressed: false,
		},
		{
			name: "does not compress when request compression disabled",
			config: httpx.CompressionConfig{
				EnableRequest:      false,
				MinSizeBytes:       1024,
				CompressibleTypes:  []string{"application/json"},
				PreferredEncodings: []string{"gzip"},
			},
			requestBody:    largeJSON,
			contentType:    "application/json",
			wantCompressed: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receivedEncoding := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedEncoding = r.Header.Get("Content-Encoding")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			defer server.Close()

			middleware := httpx.NewCompressionMiddleware(tc.config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(http.MethodPost,
				httpx.WithPath("/test"),
				httpx.WithHeader("Content-Type", tc.contentType),
				httpx.WithBody(bytes.NewReader([]byte(tc.requestBody))))

			_, err := client.Execute(*req, map[string]any{})

			require.NoError(t, err)
			if tc.wantCompressed {
				assert.Equal(t, "gzip", receivedEncoding)
			} else {
				assert.Empty(t, receivedEncoding)
			}
		})
	}
}

func TestCompressionMiddleware_Execute_ResponseDecompression(t *testing.T) {
	t.Parallel()

	testData := []byte(`{"message":"This is compressed response data","value":12345}`)

	tests := []struct {
		name             string
		config           httpx.CompressionConfig
		serverEncoding   string
		compressData     bool
		wantDecompressed bool
		wantErr          bool
	}{
		{
			name: "decompresses gzip response",
			config: httpx.CompressionConfig{
				EnableResponse: true,
			},
			serverEncoding:   "gzip",
			compressData:     true,
			wantDecompressed: true,
			wantErr:          false,
		},
		{
			name: "decompresses deflate response",
			config: httpx.CompressionConfig{
				EnableResponse: true,
			},
			serverEncoding:   "deflate",
			compressData:     true,
			wantDecompressed: true,
			wantErr:          false,
		},
		{
			name: "handles unsupported encoding gracefully",
			config: httpx.CompressionConfig{
				EnableResponse: true,
			},
			serverEncoding:   "br",
			compressData:     false,
			wantDecompressed: false,
			wantErr:          false,
		},
		{
			name: "returns error for invalid compressed data",
			config: httpx.CompressionConfig{
				EnableResponse: true,
			},
			serverEncoding:   "gzip",
			compressData:     false, // Send uncompressed but claim gzip
			wantDecompressed: false,
			wantErr:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				var dataToSend []byte
				if tc.compressData {
					switch tc.serverEncoding {
					case "gzip":
						buf := bytes.NewBuffer(nil)
						gw := gzip.NewWriter(buf)
						_, _ = gw.Write(testData)
						_ = gw.Close()
						dataToSend = buf.Bytes()
					case "deflate":
						buf := bytes.NewBuffer(nil)
						zw := zlib.NewWriter(buf)
						_, _ = zw.Write(testData)
						_ = zw.Close()
						dataToSend = buf.Bytes()
					}
				} else {
					dataToSend = testData
				}

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", tc.serverEncoding)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(dataToSend)
			}))
			defer server.Close()

			middleware := httpx.NewCompressionMiddleware(tc.config)
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
			resp, err := client.Execute(*req, map[string]any{})

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.wantDecompressed {
					// Response should be decompressed
					assert.Equal(t, http.StatusOK, resp.StatusCode)
					// Content-Encoding should be removed after decompression
					assert.Empty(t, resp.Header().Get("Content-Encoding"))
				}
			}
		})
	}
}

func TestCompressionMiddleware_Integration(t *testing.T) {
	t.Parallel()

	t.Run("end-to-end compression with client configuration", func(t *testing.T) {
		t.Parallel()

		testJSON := `{"large":"` + strings.Repeat("data", 500) + `"}`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Record the encoding we received
			encoding := r.Header.Get("Content-Encoding")

			// Return uncompressed JSON (not echoing the compressed body)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Received-Encoding", encoding)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCompression(),
		)

		req := httpx.NewRequest(http.MethodPost,
			httpx.WithPath("/test"),
			httpx.WithJSONBody(map[string]string{"data": testJSON}))

		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// Verify request was compressed
		assert.NotEmpty(t, resp.Header().Get("X-Received-Encoding"))
	})

	t.Run("compression with custom configuration", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Return gzip compressed response
			testData := []byte(`{"compressed":"response"}`)
			buf := bytes.NewBuffer(nil)
			gw := gzip.NewWriter(buf)
			_, _ = gw.Write(testData)
			_ = gw.Close()

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(buf.Bytes())
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientCompression(httpx.CompressionConfig{
				Level:              gzip.BestSpeed,
				MinSizeBytes:       512,
				EnableRequest:      true,
				EnableResponse:     true,
				PreferredEncodings: []string{"gzip"},
			}),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/test"))
		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// Response should be decompressed, Content-Encoding removed
		assert.Empty(t, resp.Header().Get("Content-Encoding"))
	})
}

func TestCompressionMiddleware_CompressionSavings(t *testing.T) {
	t.Parallel()

	t.Run("only compresses when it reduces size", func(t *testing.T) {
		t.Parallel()

		// Small data that won't compress well
		smallData := []byte("abc")
		receivedSize := int64(0)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedSize = r.ContentLength
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientCompression(httpx.CompressionConfig{
				EnableRequest:      true,
				MinSizeBytes:       1, // Set very low to try compression
				CompressibleTypes:  []string{"text/"},
				PreferredEncodings: []string{"gzip"},
			}),
		)

		req := httpx.NewRequest(http.MethodPost,
			httpx.WithPath("/test"),
			httpx.WithHeader("Content-Type", "text/plain"),
			httpx.WithBody(bytes.NewReader(smallData)))

		_, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		// Should send uncompressed because compression didn't reduce size
		assert.Equal(t, int64(len(smallData)), receivedSize)
	})
}
