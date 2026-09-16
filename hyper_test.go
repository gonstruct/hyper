package hyper_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gonstruct/hyper"
)

type echo struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Query       string            `json:"query"`
	Headers     map[string]string `json:"headers"`
	ContentType string            `json:"contentType"`
	Body        string            `json:"body"`
}

// server answers every request with a description of it, except the paths
// tests use to provoke failures.
func server(t *testing.T) *httptest.Server {
	t.Helper()

	var flaky atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"no such thing"}}`))
			return
		case "/flaky":
			if flaky.Add(1) < 3 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		case "/slow":
			time.Sleep(200 * time.Millisecond)
		case "/text":
			_, _ = w.Write([]byte("plain text"))
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

func client(t *testing.T, options ...hyper.Option) hyper.Client {
	t.Helper()

	return hyper.New(context.Background(), append([]hyper.Option{hyper.Base(server(t).URL), hyper.Untraced()}, options...)...)
}

func TestGetWithQuery(t *testing.T) {
	seen, err := client(t).Get("/things", hyper.Query{"enabled": true, "tags": []string{"a", "b"}}).JSON[echo]()
	if err != nil {
		t.Fatal(err)
	}
	if seen.Method != "GET" || seen.Path != "/things" || !strings.Contains(seen.Query, "enabled=true") || !strings.Contains(seen.Query, "tags=a&tags=b") {
		t.Fatalf("unexpected request: %+v", seen)
	}
	if seen.Headers["Accept"] != "application/json" {
		t.Fatalf("expected the JSON accept header, got %q", seen.Headers["Accept"])
	}
}

func TestPostJSON(t *testing.T) {
	type input struct {
		Model string `json:"model"`
	}
	seen, err := client(t, hyper.BearerToken("sk_test")).Post("/generations", input{Model: "nano"}, hyper.Header("Idempotency-Key", "k1")).JSON[echo]()
	if err != nil {
		t.Fatal(err)
	}
	if seen.Method != "POST" || seen.ContentType != "application/json" || seen.Body != `{"model":"nano"}` {
		t.Fatalf("unexpected request: %+v", seen)
	}
	if seen.Headers["Authorization"] != "Bearer sk_test" || seen.Headers["Idempotency-Key"] != "k1" {
		t.Fatalf("headers not sent: %+v", seen.Headers)
	}
}

func TestFormBytesAndStream(t *testing.T) {
	c := client(t)

	seen, err := c.Post("/token", hyper.Form{"grant_type": "client_credentials"}).JSON[echo]()
	if err != nil || seen.ContentType != "application/x-www-form-urlencoded" || seen.Body != "grant_type=client_credentials" {
		t.Fatalf("form: %+v %v", seen, err)
	}

	seen, err = c.Put("/upload", hyper.Bytes("png-bytes"), hyper.ContentType("image/png")).JSON[echo]()
	if err != nil || seen.Method != "PUT" || seen.ContentType != "image/png" || seen.Body != "png-bytes" {
		t.Fatalf("bytes: %+v %v", seen, err)
	}

	seen, err = c.Put("/upload", nil, hyper.Stream(strings.NewReader("streamed"), 8), hyper.ContentType("video/mp4")).JSON[echo]()
	if err != nil || seen.Body != "streamed" || seen.Headers["Content-Length"] != "8" {
		t.Fatalf("stream: %+v %v", seen, err)
	}
}

func TestMultipart(t *testing.T) {
	seen, err := client(t).Post("/files", hyper.Multipart{
		Fields: hyper.Form{"purpose": "reference"},
		Files:  []hyper.File{{Field: "file", Name: "a.png", Body: strings.NewReader("png")}},
	}).JSON[echo]()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seen.ContentType, "multipart/form-data") || !strings.Contains(seen.Body, `name="purpose"`) || !strings.Contains(seen.Body, `filename="a.png"`) {
		t.Fatalf("multipart: %+v", seen)
	}
}

func TestTwoBodiesIsAnError(t *testing.T) {
	err := client(t).Post("/x", hyper.Form{"a": 1}, hyper.Bytes("b")).Err()
	if !errors.Is(err, hyper.ErrConflictingBodies) {
		t.Fatalf("expected ErrConflictingBodies, got %v", err)
	}
}

func TestEnvelopeTextBytesAndField(t *testing.T) {
	c := client(t)

	items, err := c.Get("/envelope").JSON[[]map[string]string]("data")
	if err != nil || len(items) != 2 || items[1]["id"] != "b" {
		t.Fatalf("envelope: %v %v", items, err)
	}
	if _, err := c.Get("/envelope").JSON[[]map[string]string]("nope"); err == nil {
		t.Fatal("a missing key should be a decode error")
	}
	if c.Get("/envelope").Field("meta.total") != "2" {
		t.Fatal("Field did not read the key")
	}

	text, err := c.Get("/text").Text()
	if err != nil || text != "plain text" {
		t.Fatalf("text: %q %v", text, err)
	}
	data, err := c.Get("/text").Bytes()
	if err != nil || string(data) != "plain text" {
		t.Fatalf("bytes: %q %v", data, err)
	}
}

func TestStatusError(t *testing.T) {
	response := client(t).Get("/missing")

	if response.Ok() || response.Status() != http.StatusNotFound {
		t.Fatalf("expected a 404, got %d", response.Status())
	}

	var status *hyper.StatusError
	if !errors.As(response.Error(), &status) || status.Code != 404 || status.Response.Field("error.message") != "no such thing" {
		t.Fatalf("expected a StatusError with the body, got %v", response.Error())
	}

	if _, err := response.JSON[map[string]any](); !errors.As(err, &status) {
		t.Fatal("JSON should hand back the status error")
	}
}

func TestDecodeError(t *testing.T) {
	_, err := client(t).Get("/text").JSON[map[string]any]()

	var decode *hyper.DecodeError
	if !errors.As(err, &decode) {
		t.Fatalf("expected a DecodeError, got %v", err)
	}
}

func TestTransportErrorAndTimeout(t *testing.T) {
	err := hyper.Get(context.Background(), "http://127.0.0.1:1/nothing", hyper.Untraced()).Err()
	if !errors.Is(err, hyper.ErrTransport) {
		t.Fatalf("expected ErrTransport, got %v", err)
	}

	err = client(t).Get("/slow", hyper.Timeout(50*time.Millisecond)).Err()
	if !errors.Is(err, hyper.ErrTransport) {
		t.Fatalf("expected a timeout as ErrTransport, got %v", err)
	}
}

func TestRetryTransient(t *testing.T) {
	seen, err := client(t, hyper.Retry(3, hyper.Backoff(time.Millisecond, 5*time.Millisecond))).Get("/flaky").JSON[echo]()
	if err != nil || seen.Path != "/flaky" {
		t.Fatalf("expected the third attempt to succeed: %+v %v", seen, err)
	}

	if err := client(t).Get("/flaky").Err(); err == nil {
		t.Fatal("without Retry the first 503 is the answer")
	}
}

func TestSinkStreamsTheBody(t *testing.T) {
	var sink bytes.Buffer
	response := client(t).Get("/text", hyper.Sink(&sink))
	if err := response.Err(); err != nil || sink.String() != "plain text" {
		t.Fatalf("sink: %q %v", sink.String(), err)
	}
	if _, err := response.Bytes(); !errors.Is(err, hyper.ErrNoBody) {
		t.Fatal("a sunk body should not be readable twice")
	}
}

func TestClientIsAValue(t *testing.T) {
	base := client(t, hyper.Header("X-Base", "1"))
	derived := base.With(hyper.Header("X-Derived", "2"))

	seen, _ := base.Get("/x").JSON[echo]()
	if seen.Headers["X-Derived"] != "" {
		t.Fatal("With mutated the receiver")
	}
	seen, _ = derived.Get("/x").JSON[echo]()
	if seen.Headers["X-Base"] != "1" || seen.Headers["X-Derived"] != "2" {
		t.Fatalf("derived client lost headers: %+v", seen.Headers)
	}
}
