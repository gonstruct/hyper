// Package tests exercises hyper from the outside, against a real server
// where a real server is what matters and against the fake where it is not.
package tests_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gonstruct/hyper"
)

// echo is what the test server answers with: a description of the request
// it received.
type echo struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Query       string            `json:"query"`
	Headers     map[string]string `json:"headers"`
	ContentType string            `json:"contentType"`
	Body        string            `json:"body"`
}

// server echoes every request, except the paths tests use to provoke a
// particular answer.
func server(t *testing.T) *httptest.Server {
	t.Helper()

	var flaky, retryAfter atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"no such thing"}}`))
			return
		case "/broken":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"it broke"}`))
			return
		case "/flaky":
			if flaky.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		case "/retry-after":
			if retryAfter.Add(1) < 2 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
		case "/conflict":
			w.WriteHeader(http.StatusConflict)
			return
		case "/slow":
			time.Sleep(300 * time.Millisecond)
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("plain text"))
			return
		case "/empty":
			w.WriteHeader(http.StatusNoContent)
			return
		case "/envelope":
			_, _ = w.Write([]byte(`{"data":[{"id":"a"},{"id":"b"}],"meta":{"total":2}}`))
			return
		}

		body, _ := io.ReadAll(r.Body)
		headers := map[string]string{}
		for name := range r.Header {
			headers[name] = r.Header.Get(name)
		}
		_ = json.NewEncoder(w).Encode(echo{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Headers: headers,
			ContentType: r.Header.Get("Content-Type"), Body: string(body),
		})
	})

	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)

	return s
}

// client is a hyper client pointed at a fresh test server.
func client(t *testing.T, options ...hyper.Option) hyper.Client {
	t.Helper()

	return hyper.New(context.Background(), append([]hyper.Option{hyper.Base(server(t).URL), hyper.Untraced()}, options...)...)
}

// seen sends a request and returns what the server saw.
func seen(t *testing.T, response *hyper.Response) echo {
	t.Helper()

	result, err := response.JSON[echo]()
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return result
}
