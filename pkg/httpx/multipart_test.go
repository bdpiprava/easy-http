package httpx_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

type MultipartTestSuite struct {
	suite.Suite
	tempDir string
}

func TestMultipartTestSuite(t *testing.T) {
	suite.Run(t, new(MultipartTestSuite))
}

func (s *MultipartTestSuite) SetupTest() {
	// Create temp directory for test files
	tempDir, err := os.MkdirTemp("", "httpx-multipart-test-*")
	s.Require().NoError(err)
	s.tempDir = tempDir
}

func (s *MultipartTestSuite) TearDownTest() {
	// Clean up temp directory
	if s.tempDir != "" {
		os.RemoveAll(s.tempDir)
	}
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_FieldsOnly() {
	builder := httpx.NewMultipartFormBuilder()
	builder.AddField("username", "alice")
	builder.AddField("email", "alice@example.com")

	reader, contentType, err := builder.Build()

	s.Require().NoError(err)
	s.Contains(contentType, "multipart/form-data")
	s.Contains(contentType, "boundary=")

	// Parse the multipart content to verify fields
	content, err := io.ReadAll(reader)
	s.Require().NoError(err)

	// Extract boundary from content type
	boundary := extractBoundary(contentType)
	s.NotEmpty(boundary)

	// Verify fields are present
	contentStr := string(content)
	s.Contains(contentStr, "username")
	s.Contains(contentStr, "alice")
	s.Contains(contentStr, "email")
	s.Contains(contentStr, "alice@example.com")
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_SingleFile() {
	// Create test file
	testFile := filepath.Join(s.tempDir, "test.txt")
	testContent := "Hello, multipart world!"
	err := os.WriteFile(testFile, []byte(testContent), 0600)
	s.Require().NoError(err)

	builder := httpx.NewMultipartFormBuilder()
	err = builder.AddFileFromPath("file", testFile)
	s.Require().NoError(err)

	reader, contentType, err := builder.Build()

	s.Require().NoError(err)
	s.Contains(contentType, "multipart/form-data")

	// Parse and verify file content
	content, err := io.ReadAll(reader)
	s.Require().NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "test.txt")
	s.Contains(contentStr, testContent)
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_MultipleFiles() {
	// Create test files
	testFile1 := filepath.Join(s.tempDir, "file1.txt")
	testFile2 := filepath.Join(s.tempDir, "file2.txt")
	err := os.WriteFile(testFile1, []byte("content1"), 0600)
	s.Require().NoError(err)
	err = os.WriteFile(testFile2, []byte("content2"), 0600)
	s.Require().NoError(err)

	builder := httpx.NewMultipartFormBuilder()
	err = builder.AddFileFromPath("file1", testFile1)
	s.Require().NoError(err)
	err = builder.AddFileFromPath("file2", testFile2)
	s.Require().NoError(err)

	reader, contentType, err := builder.Build()

	s.Require().NoError(err)
	s.Contains(contentType, "multipart/form-data")

	content, err := io.ReadAll(reader)
	s.Require().NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "file1.txt")
	s.Contains(contentStr, "content1")
	s.Contains(contentStr, "file2.txt")
	s.Contains(contentStr, "content2")
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_MixedFieldsAndFiles() {
	// Create test file
	testFile := filepath.Join(s.tempDir, "data.csv")
	csvContent := "name,value\nalice,100"
	err := os.WriteFile(testFile, []byte(csvContent), 0600)
	s.Require().NoError(err)

	builder := httpx.NewMultipartFormBuilder()
	builder.AddField("user_id", "12345")
	builder.AddField("action", "import")
	err = builder.AddFileFromPath("data", testFile)
	s.Require().NoError(err)

	reader, contentType, err := builder.Build()

	s.Require().NoError(err)
	s.Contains(contentType, "multipart/form-data")

	content, err := io.ReadAll(reader)
	s.Require().NoError(err)

	contentStr := string(content)
	// Verify fields
	s.Contains(contentStr, "user_id")
	s.Contains(contentStr, "12345")
	s.Contains(contentStr, "action")
	s.Contains(contentStr, "import")
	// Verify file
	s.Contains(contentStr, "data.csv")
	s.Contains(contentStr, csvContent)
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_FileFromReader() {
	reader := strings.NewReader("test content from reader")

	builder := httpx.NewMultipartFormBuilder()
	builder.AddFile("document", "readme.md", reader)

	result, contentType, err := builder.Build()

	s.Require().NoError(err)
	s.Contains(contentType, "multipart/form-data")

	content, err := io.ReadAll(result)
	s.Require().NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "readme.md")
	s.Contains(contentStr, "test content from reader")
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_NonexistentFile() {
	builder := httpx.NewMultipartFormBuilder()
	err := builder.AddFileFromPath("file", "/nonexistent/file.txt")

	s.Error(err)
	s.Contains(err.Error(), "failed to open file")
}

func (s *MultipartTestSuite) TestMultipartFormBuilder_ChainableMethods() {
	// Test that AddField and AddFile return builder for chaining
	builder := httpx.NewMultipartFormBuilder().
		AddField("field1", "value1").
		AddField("field2", "value2").
		AddFile("file", "test.txt", strings.NewReader("content"))

	s.NotNil(builder)

	reader, contentType, err := builder.Build()
	s.NoError(err)
	s.Contains(contentType, "multipart/form-data")

	content, err := io.ReadAll(reader)
	s.Require().NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "field1")
	s.Contains(contentStr, "field2")
	s.Contains(contentStr, "test.txt")
}

func (s *MultipartTestSuite) TestWithMultipartForm_NilBuilder() {
	req := httpx.NewRequest("POST", httpx.WithMultipartForm(nil))
	_, err := req.ToHTTPReq(httpx.ClientOptions{})

	s.Error(err)
	s.Contains(err.Error(), "multipart form builder cannot be nil")
}

func (s *MultipartTestSuite) TestWithFile_Convenience() {
	// Create test file
	testFile := filepath.Join(s.tempDir, "upload.txt")
	err := os.WriteFile(testFile, []byte("upload content"), 0600)
	s.Require().NoError(err)

	req := httpx.NewRequest("POST", httpx.WithFile("attachment", testFile))
	httpReq, err := req.ToHTTPReq(httpx.ClientOptions{})

	s.Require().NoError(err)
	s.Contains(httpReq.Header.Get("Content-Type"), "multipart/form-data")

	// Read and verify body
	content, err := io.ReadAll(httpReq.Body)
	s.Require().NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "upload.txt")
	s.Contains(contentStr, "upload content")
}

func (s *MultipartTestSuite) TestWithFile_NonexistentFile() {
	req := httpx.NewRequest("POST", httpx.WithFile("file", "/nonexistent/path.txt"))
	_, err := req.ToHTTPReq(httpx.ClientOptions{})

	s.Error(err)
	s.Contains(err.Error(), "failed to add file from path")
}

func (s *MultipartTestSuite) TestMultipartIntegration() {
	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Create test file
	testFile := filepath.Join(s.tempDir, "avatar.jpg")
	err := os.WriteFile(testFile, []byte("fake image data"), 0600)
	s.Require().NoError(err)

	mockServer.SetupMock("POST", "/upload", 200, `{"status":"uploaded","file":"avatar.jpg"}`)

	// Build multipart form
	builder := httpx.NewMultipartFormBuilder()
	builder.AddField("user_id", "42")
	builder.AddField("description", "My avatar")
	err = builder.AddFileFromPath("avatar", testFile)
	s.Require().NoError(err)

	// Send request
	resp, err := httpx.POST[map[string]any](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/upload"),
		httpx.WithMultipartForm(builder),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(200, resp.StatusCode)
	s.Equal("uploaded", resp.Body.(map[string]any)["status"])
	s.Equal("avatar.jpg", resp.Body.(map[string]any)["file"])
}

func (s *MultipartTestSuite) TestWithFile_IntegrationConvenience() {
	// Setup mock server
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Create test file
	testFile := filepath.Join(s.tempDir, "document.pdf")
	err := os.WriteFile(testFile, []byte("fake pdf data"), 0600)
	s.Require().NoError(err)

	mockServer.SetupMock("POST", "/upload", 201, `{"id":"doc123"}`)

	// Send request using WithFile convenience function
	resp, err := httpx.POST[map[string]any](
		httpx.WithBaseURL(mockServer.GetURL()),
		httpx.WithPath("/upload"),
		httpx.WithFile("file", testFile),
	)

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Equal(201, resp.StatusCode)
	s.Equal("doc123", resp.Body.(map[string]any)["id"])
}

// Helper function to extract boundary from Content-Type header
func extractBoundary(contentType string) string {
	parts := strings.Split(contentType, "boundary=")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// TestParseMultipartContent verifies we can parse the generated multipart content
func TestParseMultipartContent(t *testing.T) {
	builder := httpx.NewMultipartFormBuilder()
	builder.AddField("name", "test")
	builder.AddFile("file", "test.txt", strings.NewReader("file content"))

	reader, contentType, err := builder.Build()
	require.NoError(t, err)

	// Extract boundary
	boundary := extractBoundary(contentType)
	require.NotEmpty(t, boundary)

	// Parse multipart content
	content, err := io.ReadAll(reader)
	require.NoError(t, err)

	mr := multipart.NewReader(bytes.NewReader(content), boundary)

	// Verify fields
	fieldCount := 0
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		partContent, err := io.ReadAll(part)
		require.NoError(t, err)

		if part.FormName() == "name" {
			assert.Equal(t, "test", string(partContent))
			fieldCount++
		} else if part.FormName() == "file" {
			assert.Equal(t, "test.txt", part.FileName())
			assert.Equal(t, "file content", string(partContent))
			fieldCount++
		}
	}

	assert.Equal(t, 2, fieldCount, "should have parsed both field and file")
}
