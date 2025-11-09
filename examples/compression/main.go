package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func main() {
	fmt.Println("HTTP Compression Example")
	fmt.Println("========================")

	// Example 1: Basic compression with default settings
	example1()

	// Example 2: Custom compression configuration
	example2()

	// Example 3: Request-only compression
	example3()

	// Example 4: Response decompression
	example4()

	// Example 5: Compression savings demonstration
	example5()
}

func example1() {
	fmt.Println("\nExample 1: Default Compression")
	fmt.Println("-------------------------------")

	// Create a test server that records request details
	requestSize := int64(0)
	requestEncoding := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSize = r.ContentLength
		requestEncoding = r.Header.Get("Content-Encoding")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"received"}`))
	}))
	defer server.Close()

	// Create client with default compression (gzip + deflate support)
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultCompression(),
	)

	// Send a large JSON payload that will benefit from compression
	largeData := map[string]string{
		"data": strings.Repeat("This is test data for compression. ", 100),
	}

	req := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/api/data"),
		httpx.WithJSONBody(largeData))

	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Request was compressed: %s\n", requestEncoding)
	fmt.Printf("Compressed size sent to server: %d bytes\n", requestSize)
	fmt.Println("✓ Automatic compression reduced bandwidth usage")
}

func example2() {
	fmt.Println("\nExample 2: Custom Compression Configuration")
	fmt.Println("-------------------------------------------")

	requestEncoding := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestEncoding = r.Header.Get("Content-Encoding")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create client with custom compression settings
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientCompression(httpx.CompressionConfig{
			Level:              gzip.BestSpeed,               // Favor speed over compression ratio
			MinSizeBytes:       512,                          // Compress if > 512 bytes
			CompressibleTypes:  []string{"application/json"}, // Only compress JSON
			EnableRequest:      true,                         // Compress outgoing requests
			EnableResponse:     true,                         // Decompress incoming responses
			PreferredEncodings: []string{"gzip"},             // Use gzip encoding
		}),
	)

	// Send a JSON payload larger than 512 bytes
	data := map[string]string{
		"message": strings.Repeat("data", 200), // ~800 bytes
	}

	req := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/api/data"),
		httpx.WithJSONBody(data))

	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Request encoding: %s\n", requestEncoding)
	fmt.Println("✓ Custom compression config: fast compression, JSON only, 512 byte threshold")
}

func example3() {
	fmt.Println("\nExample 3: Request-Only Compression")
	fmt.Println("------------------------------------")

	requestEncoding := ""
	acceptEncoding := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestEncoding = r.Header.Get("Content-Encoding")
		acceptEncoding = r.Header.Get("Accept-Encoding")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Enable request compression only (no response decompression)
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientCompression(httpx.CompressionConfig{
			EnableRequest:      true,  // Compress requests
			EnableResponse:     false, // Don't handle compressed responses
			MinSizeBytes:       1024,  // 1KB threshold
			CompressibleTypes:  []string{"application/json"},
			PreferredEncodings: []string{"gzip"},
		}),
	)

	// Send large JSON that exceeds threshold
	largeData := map[string]string{
		"payload": strings.Repeat("test data ", 200), // ~2KB
	}

	req := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/upload"),
		httpx.WithJSONBody(largeData))

	resp, err := client.Execute(*req, map[string]any{})
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Request was compressed: %s\n", requestEncoding)
	fmt.Printf("Accept-Encoding sent: %s\n", acceptEncoding)
	fmt.Println("✓ Compressed outgoing data, disabled automatic response decompression")
}

func example4() {
	fmt.Println("\nExample 4: Response Decompression")
	fmt.Println("----------------------------------")

	// Create server that sends gzip-compressed responses
	originalData := []byte(`{"message":"This is a compressed response","data":"` + strings.Repeat("test", 500) + `"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Compress the response
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, _ = gw.Write(originalData)
		_ = gw.Close()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	// Enable response decompression
	client := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientCompression(httpx.CompressionConfig{
			EnableRequest:      false, // Don't compress requests
			EnableResponse:     true,  // Decompress responses automatically
			PreferredEncodings: []string{"gzip", "deflate"},
		}),
	)

	req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/api/data"))

	// Execute and unmarshal response (body is automatically decompressed)
	var result map[string]any
	resp, err := client.Execute(*req, &result)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Original compressed data size: %d bytes\n", len(originalData))
	fmt.Printf("Successfully unmarshaled decompressed JSON: %v\n", result["message"] != nil)
	fmt.Printf("Content-Encoding header after decompression: '%s'\n", resp.Header().Get("Content-Encoding"))
	fmt.Println("✓ Automatically decompressed gzip response, removed Content-Encoding header")
}

func example5() {
	fmt.Println("\nExample 5: Compression Savings Demonstration")
	fmt.Println("--------------------------------------------")

	var uncompressedSize int64
	var compressedSize int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record the size we received
		if r.Header.Get("Content-Encoding") == "gzip" {
			compressedSize = r.ContentLength
		} else {
			uncompressedSize = r.ContentLength
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// First, send without compression
	clientNoCompression := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		// No compression middleware
	)

	// Create a large, highly compressible payload
	largeData := map[string]string{
		"data1": strings.Repeat("This is repetitive test data for compression demonstration. ", 50),
		"data2": strings.Repeat("More repetitive content that compresses very well. ", 50),
		"data3": strings.Repeat("Additional redundant information for testing purposes. ", 50),
	}

	req1 := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/api/data"),
		httpx.WithJSONBody(largeData))

	_, err := clientNoCompression.Execute(*req1, map[string]any{})
	if err != nil {
		log.Printf("Uncompressed request failed: %v", err)
		return
	}

	// Now send with compression
	clientWithCompression := httpx.NewClientWithConfig(
		httpx.WithClientDefaultBaseURL(server.URL),
		httpx.WithClientDefaultCompression(),
	)

	req2 := httpx.NewRequest(http.MethodPost,
		httpx.WithPath("/api/data"),
		httpx.WithJSONBody(largeData))

	_, err = clientWithCompression.Execute(*req2, map[string]any{})
	if err != nil {
		log.Printf("Compressed request failed: %v", err)
		return
	}

	savings := float64(uncompressedSize-compressedSize) / float64(uncompressedSize) * 100

	fmt.Printf("Uncompressed size: %d bytes\n", uncompressedSize)
	fmt.Printf("Compressed size:   %d bytes\n", compressedSize)
	fmt.Printf("Bandwidth saved:   %.1f%%\n", savings)
	fmt.Printf("Compression ratio: %.2fx\n", float64(uncompressedSize)/float64(compressedSize))
	fmt.Println("✓ Significant bandwidth savings with automatic compression!")
}
