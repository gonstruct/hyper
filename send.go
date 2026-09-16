package hyper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// ErrConflictingBodies is returned when a request was given two bodies.
var ErrConflictingBodies = errors.New("hyper: a request can carry one body")

func send(ctx context.Context, method Method, target string, input any, s settings) *Response {
	s.input(input)

	response := &Response{Method: method, URL: joinURL(s.base, target)}

	if s.bodies > 1 {
		response.err = ErrConflictingBodies
		return response
	}
	if len(s.query) > 0 {
		separator := "?"
		if strings.Contains(response.URL, "?") {
			separator = "&"
		}
		response.URL += separator + s.query.Encode()
	}

	client := &http.Client{Transport: transportFor(s)}
	attempts := 1 + s.retry.attempts
	if s.body != nil && s.body.stream != nil {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		result := once(ctx, client, response.Method, response.URL, s)
		if attempt == attempts || !s.retry.should(result) {
			return result
		}

		if err := wait(ctx, s.retry.delay(attempt, result)); err != nil {
			result.err = fmt.Errorf("%w: %v", ErrTransport, err)
			return result
		}
	}

	return response
}

func once(ctx context.Context, client *http.Client, method Method, target string, s settings) *Response {
	response := &Response{Method: method, URL: target}

	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	var reader io.Reader
	var size int64 = -1
	if s.body != nil {
		if s.body.stream != nil {
			reader, size = s.body.stream, s.body.size
		} else {
			reader, size = bytes.NewReader(s.body.bytes), int64(len(s.body.bytes))
		}
	}

	request, err := http.NewRequestWithContext(ctx, string(method), target, reader)
	if err != nil {
		response.err = fmt.Errorf("%w: %v", ErrTransport, err)
		return response
	}
	if size >= 0 {
		request.ContentLength = size
	}
	request.Header = s.headers.Clone()
	if s.body != nil {
		if s.contentType != "" {
			request.Header.Set("Content-Type", s.contentType)
		} else if s.body.contentType != "" && request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", s.body.contentType)
		}
	}
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/json")
	}

	raw, err := client.Do(request)
	if err != nil {
		response.err = fmt.Errorf("%w: %v", ErrTransport, err)
		return response
	}
	defer raw.Body.Close()

	response.status = raw.StatusCode
	response.header = raw.Header

	if s.sink != nil {
		if _, err := io.Copy(s.sink, raw.Body); err != nil {
			response.err = fmt.Errorf("%w: read body: %v", ErrTransport, err)
			return response
		}
		response.sunk = true
	} else {
		data, err := io.ReadAll(raw.Body)
		if err != nil {
			response.err = fmt.Errorf("%w: read body: %v", ErrTransport, err)
			return response
		}
		response.body = data
	}

	if raw.StatusCode >= 400 {
		response.err = &StatusError{Method: method, URL: target, Code: raw.StatusCode, Response: response}
	}

	return response
}

func transportFor(s settings) http.RoundTripper {
	transport := s.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if s.untraced {
		return transport
	}

	return otelhttp.NewTransport(transport)
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
