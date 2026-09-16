package hyper

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/tidwall/gjson"
)

// Response is what a verb returns. It always exists, even when nothing came
// back, and carries the error for that case so a call stays one line.
type Response struct {
	Method Method
	URL    string

	status int
	header http.Header
	body   []byte
	sunk   bool
	err    error
}

// ErrTransport marks errors where no response arrived: DNS, connection,
// timeout, a cancelled context.
var ErrTransport = errors.New("hyper: transport")

// ErrNoBody is returned when a body was streamed to a Sink and then asked for.
var ErrNoBody = errors.New("hyper: the body was streamed to a sink")

// StatusError is a response with a 4xx or 5xx status. The response is
// attached so the provider's own message is still readable.
type StatusError struct {
	Method   Method
	URL      string
	Code     int
	Response *Response
}

func (err *StatusError) Error() string {
	return fmt.Sprintf("hyper: %s %s returned %d", err.Method, err.URL, err.Code)
}

// DecodeError is a 2xx whose body was not the type asked for.
type DecodeError struct {
	Response *Response
	Err      error
}

func (err *DecodeError) Error() string {
	return fmt.Sprintf("hyper: decode %s %s: %v", err.Response.Method, err.Response.URL, err.Err)
}

func (err *DecodeError) Unwrap() error { return err.Err }

// Ok is true for a 2xx that arrived.
func (r *Response) Ok() bool {
	return r.err == nil && r.status >= 200 && r.status < 300
}

// Status is the HTTP status, or 0 when nothing came back.
func (r *Response) Status() int { return r.status }

// Header reads one response header.
func (r *Response) Header(name string) string {
	if r.header == nil {
		return ""
	}

	return r.header.Get(name)
}

// Headers is every response header.
func (r *Response) Headers() http.Header { return r.header }

// Error is nil for a 2xx that arrived, otherwise the transport error or the
// *StatusError.
func (r *Response) Error() error { return r.err }

// Err is Error, for a call that only wants to know whether it went through.
func (r *Response) Err() error { return r.err }

// Bytes is the body as received.
func (r *Response) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.sunk {
		return nil, ErrNoBody
	}

	return r.body, nil
}

// Text is the body as a string.
func (r *Response) Text() (string, error) {
	data, err := r.Bytes()

	return string(data), err
}

// Field reads one key of a JSON body without decoding all of it, as a
// string. Nested keys use dots: "error.message". Missing is "".
func (r *Response) Field(path string) string {
	return gjson.GetBytes(r.body, path).String()
}

// JSON decodes the body into T. With a key, that key of the body is decoded
// instead of the root. The response's own error comes first, so a folded
// call is one check.
func (r *Response) JSON[T any](path ...string) (T, error) {
	var value T

	data, err := r.Bytes()
	if err != nil {
		return value, err
	}
	if len(path) > 0 && path[0] != "" {
		result := gjson.GetBytes(data, path[0])
		if !result.Exists() {
			return value, &DecodeError{Response: r, Err: fmt.Errorf("key %q is not in the body", path[0])}
		}
		data = []byte(result.Raw)
	}

	if err := json.Unmarshal(data, &value); err != nil {
		return value, &DecodeError{Response: r, Err: err}
	}

	return value, nil
}
