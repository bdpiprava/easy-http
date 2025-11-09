package httpx

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// MultipartFormBuilder helps construct multipart/form-data requests
type MultipartFormBuilder struct {
	fields map[string]string
	files  []MultipartFile
	buffer *bytes.Buffer
	writer *multipart.Writer
}

// MultipartFile represents a file to be uploaded
type MultipartFile struct {
	FieldName string
	FileName  string
	Reader    io.Reader
}

// NewMultipartFormBuilder creates a new multipart form builder
func NewMultipartFormBuilder() *MultipartFormBuilder {
	buffer := &bytes.Buffer{}
	return &MultipartFormBuilder{
		fields: make(map[string]string),
		files:  make([]MultipartFile, 0),
		buffer: buffer,
		writer: multipart.NewWriter(buffer),
	}
}

// AddField adds a text field to the multipart form
func (b *MultipartFormBuilder) AddField(name, value string) *MultipartFormBuilder {
	b.fields[name] = value
	return b
}

// AddFile adds a file from an io.Reader
func (b *MultipartFormBuilder) AddFile(fieldName, fileName string, reader io.Reader) *MultipartFormBuilder {
	b.files = append(b.files, MultipartFile{
		FieldName: fieldName,
		FileName:  fileName,
		Reader:    reader,
	})
	return b
}

// AddFileFromPath adds a file from a file path
func (b *MultipartFormBuilder) AddFileFromPath(fieldName, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to open file: %s", filePath)
	}

	fileName := filepath.Base(filePath)
	b.AddFile(fieldName, fileName, file)
	return nil
}

// Build constructs the multipart form body and returns reader and content-type
func (b *MultipartFormBuilder) Build() (io.Reader, string, error) {
	// Write fields
	for name, value := range b.fields {
		if err := b.writer.WriteField(name, value); err != nil {
			return nil, "", errors.Wrapf(err, "failed to write field: %s", name)
		}
	}

	// Write files
	for _, file := range b.files {
		part, err := b.writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			return nil, "", errors.Wrapf(err, "failed to create form file: %s", file.FileName)
		}

		if _, err := io.Copy(part, file.Reader); err != nil {
			return nil, "", errors.Wrapf(err, "failed to copy file content: %s", file.FileName)
		}

		// Close file if it's an *os.File to release resources
		if closer, ok := file.Reader.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return nil, "", errors.Wrapf(err, "failed to close file: %s", file.FileName)
			}
		}
	}

	// Close writer to finalize multipart body
	if err := b.writer.Close(); err != nil {
		return nil, "", errors.Wrap(err, "failed to close multipart writer")
	}

	contentType := b.writer.FormDataContentType()
	return b.buffer, contentType, nil
}
