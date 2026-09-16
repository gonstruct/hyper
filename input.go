package hyper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
)

// Query is input that travels as the query string. Values are formatted
// with fmt; a slice adds the key once per element.
type Query map[string]any

// Form is input that travels as application/x-www-form-urlencoded.
type Form map[string]any

// Bytes is a raw body. Give it a ContentType option.
type Bytes []byte

// File is one part of a Multipart body.
type File struct {
	Field string
	Name  string
	Body  io.Reader
}

// Multipart is a multipart/form-data body.
type Multipart struct {
	Fields Form
	Files  []File
}

// Stream is a body read from a reader as it is sent. Size becomes
// Content-Length; a stream cannot be retried, so Retry is ignored for it.
func Stream(reader io.Reader, size int64) Option {
	return optionFunc(func(s *settings) { s.setBody(&body{stream: reader, size: size}) })
}

// JSON forces JSON encoding for an input hyper would otherwise treat as a
// different kind, such as a string.
func JSON(value any) Option {
	return optionFunc(func(s *settings) { s.setBody(jsonBody(value)) })
}

// body is what goes on the wire. Buffered bodies can be re-sent on retry;
// a stream can not.
type body struct {
	bytes       []byte
	stream      io.Reader
	size        int64
	contentType string
}

func (s *settings) setBody(next *body) {
	s.body = next
	s.bodies++
}

func (query Query) apply(s *settings) {
	for key, value := range query {
		for _, item := range values(value) {
			s.query.Add(key, item)
		}
	}
}

func (form Form) apply(s *settings) {
	encoded := url.Values{}
	for key, value := range form {
		for _, item := range values(value) {
			encoded.Add(key, item)
		}
	}
	s.setBody(&body{bytes: []byte(encoded.Encode()), contentType: "application/x-www-form-urlencoded"})
}

func (raw Bytes) apply(s *settings) {
	s.setBody(&body{bytes: []byte(raw)})
}

func (parts Multipart) apply(s *settings) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	for key, value := range parts.Fields {
		for _, item := range values(value) {
			if err := writer.WriteField(key, item); err != nil {
				s.setBody(&body{stream: failingReader{err}})
				return
			}
		}
	}
	for _, file := range parts.Files {
		part, err := writer.CreateFormFile(file.Field, file.Name)
		if err == nil {
			_, err = io.Copy(part, file.Body)
		}
		if err != nil {
			s.setBody(&body{stream: failingReader{err}})
			return
		}
	}
	if err := writer.Close(); err != nil {
		s.setBody(&body{stream: failingReader{err}})
		return
	}

	s.setBody(&body{bytes: buffer.Bytes(), contentType: writer.FormDataContentType()})
}

func jsonBody(value any) *body {
	encoded, err := json.Marshal(value)
	if err != nil {
		return &body{stream: failingReader{fmt.Errorf("encode json body: %w", err)}}
	}

	return &body{bytes: encoded, contentType: "application/json"}
}

// input places the verb's input: an Option-like input applies itself, nil
// is nothing, anything else is JSON.
func (s *settings) input(value any) {
	switch typed := value.(type) {
	case nil:
	case Query:
		typed.apply(s)
	case Form:
		typed.apply(s)
	case Bytes:
		typed.apply(s)
	case Multipart:
		typed.apply(s)
	case Option:
		typed.apply(s)
	default:
		s.setBody(jsonBody(value))
	}
}

func values(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(value)}
	}
}

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }

func joinURL(base, path string) string {
	if base == "" || strings.Contains(path, "://") {
		return path
	}

	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
}
