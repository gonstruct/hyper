package hyper

import (
	"encoding/base64"
	"io"
	"maps"
	"net/http"
	"net/url"
	"time"
)

// Option configures a client or a single request. The same options work in
// both places: on the client they are the defaults, on a call they add to or
// replace them for that call. Inputs such as Query and Form are options too.
type Option interface {
	apply(*settings)
}

type optionFunc func(*settings)

func (option optionFunc) apply(s *settings) { option(s) }

// H is a set of headers.
type H map[string]string

// settings is what a request is sent with, after the client's and the call's
// options are merged.
type settings struct {
	base        string
	headers     http.Header
	query       url.Values
	timeout     time.Duration
	retry       retryPolicy
	transport   http.RoundTripper
	untraced    bool
	contentType string
	sink        io.Writer
	body        *body
	bodies      int
}

func newSettings() settings {
	return settings{headers: http.Header{}, query: url.Values{}}
}

func (s settings) clone() settings {
	cloned := s
	cloned.headers = s.headers.Clone()
	cloned.query = maps.Clone(s.query)
	if cloned.query == nil {
		cloned.query = url.Values{}
	}

	return cloned
}

func (s settings) apply(options ...Option) settings {
	for _, option := range options {
		if option != nil {
			option.apply(&s)
		}
	}

	return s
}

// Base is the URL every relative path is resolved against.
func Base(base string) Option {
	return optionFunc(func(s *settings) { s.base = base })
}

// Header sets one header. Setting the same name again replaces it.
func Header(name, value string) Option {
	return optionFunc(func(s *settings) { s.headers.Set(name, value) })
}

// Headers sets several headers.
func Headers(headers H) Option {
	return optionFunc(func(s *settings) {
		for name, value := range headers {
			s.headers.Set(name, value)
		}
	})
}

// BearerToken sets the Authorization header.
func BearerToken(token string) Option {
	return Header("Authorization", "Bearer "+token)
}

// BasicAuth sets the Authorization header.
func BasicAuth(username, password string) Option {
	return Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}

// Accept sets the Accept header. Requests get application/json without it.
func Accept(mime string) Option {
	return Header("Accept", mime)
}

// ContentType sets the Content-Type of a Bytes or Stream body. JSON, Form
// and Multipart bodies set their own.
func ContentType(mime string) Option {
	return optionFunc(func(s *settings) { s.contentType = mime })
}

// Timeout bounds one attempt. A retry gets a fresh one.
func Timeout(timeout time.Duration) Option {
	return optionFunc(func(s *settings) { s.timeout = timeout })
}

// Transport is what actually sends the request. It is wrapped with
// OpenTelemetry tracing unless Untraced is set. The default is
// http.DefaultTransport.
func Transport(transport http.RoundTripper) Option {
	return optionFunc(func(s *settings) { s.transport = transport })
}

// Untraced turns the OpenTelemetry wrapping off.
func Untraced() Option {
	return optionFunc(func(s *settings) { s.untraced = true })
}

// Sink streams the response body into the writer instead of buffering it.
// The Response's body is then empty; Bytes, Text and JSON report ErrNoBody.
func Sink(writer io.Writer) Option {
	return optionFunc(func(s *settings) { s.sink = writer })
}
