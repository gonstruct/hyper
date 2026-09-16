package hyper_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gonstruct/hyper"
)

func TestAStructOrMapIsAJSONBody(t *testing.T) {
	type input struct {
		Title string `json:"title"`
	}
	c := client(t)

	got := seen(t, c.Post("/x", input{Title: "write"}))
	if got.ContentType != "application/json" || got.Body != `{"title":"write"}` {
		t.Errorf("struct: %+v", got)
	}

	got = seen(t, c.Post("/x", map[string]any{"a": 1}))
	if got.ContentType != "application/json" || got.Body != `{"a":1}` {
		t.Errorf("map: %+v", got)
	}

	// A JSON body never overrides an explicit Content-Type header.
	got = seen(t, c.Post("/x", input{Title: "write"}, hyper.Header("Content-Type", "application/vnd.api+json")))
	if got.ContentType != "application/vnd.api+json" {
		t.Errorf("explicit content type lost: %q", got.ContentType)
	}
}

func TestNilIsNoBody(t *testing.T) {
	got := seen(t, client(t).Post("/x", nil))
	if got.Body != "" || got.ContentType != "" {
		t.Errorf("expected no body and no content type: %+v", got)
	}
}

func TestJSONOptionForcesEncoding(t *testing.T) {
	// A string input would otherwise be... a string, which JSON encodes as a
	// quoted string. JSON says so explicitly.
	got := seen(t, client(t).Post("/x", nil, hyper.JSON("hello")))
	if got.Body != `"hello"` || got.ContentType != "application/json" {
		t.Errorf("JSON option: %+v", got)
	}
}

func TestForm(t *testing.T) {
	got := seen(t, client(t).Post("/x", hyper.Form{"grant_type": "client_credentials", "scope": []string{"a", "b"}}))
	if got.ContentType != "application/x-www-form-urlencoded" {
		t.Errorf("content type: %q", got.ContentType)
	}
	if !strings.Contains(got.Body, "grant_type=client_credentials") || !strings.Contains(got.Body, "scope=a&scope=b") {
		t.Errorf("body: %q", got.Body)
	}
}

func TestBytesAndContentType(t *testing.T) {
	got := seen(t, client(t).Put("/x", hyper.Bytes("png-bytes"), hyper.ContentType("image/png")))
	if got.ContentType != "image/png" || got.Body != "png-bytes" || got.Headers["Content-Length"] != "9" {
		t.Errorf("bytes: %+v", got)
	}
}

func TestStream(t *testing.T) {
	got := seen(t, client(t).Put("/x", nil, hyper.Stream(strings.NewReader("streamed"), 8), hyper.ContentType("video/mp4")))
	if got.Body != "streamed" || got.ContentType != "video/mp4" || got.Headers["Content-Length"] != "8" {
		t.Errorf("stream: %+v", got)
	}
}

func TestMultipart(t *testing.T) {
	got := seen(t, client(t).Post("/x", hyper.Multipart{
		Fields: hyper.Form{"purpose": "reference"},
		Files:  []hyper.File{{Field: "file", Name: "a.png", Body: strings.NewReader("png")}},
	}))
	if !strings.HasPrefix(got.ContentType, "multipart/form-data; boundary=") {
		t.Errorf("content type: %q", got.ContentType)
	}
	for _, want := range []string{`name="purpose"`, "reference", `name="file"; filename="a.png"`, "png"} {
		if !strings.Contains(got.Body, want) {
			t.Errorf("expected %q in the multipart body", want)
		}
	}
}

func TestTwoBodiesIsAnError(t *testing.T) {
	c := client(t)

	for name, response := range map[string]*hyper.Response{
		"form and bytes":  c.Post("/x", hyper.Form{"a": 1}, hyper.Bytes("b")),
		"json and stream": c.Post("/x", map[string]int{"a": 1}, hyper.Stream(strings.NewReader("s"), 1)),
	} {
		if !errors.Is(response.Err(), hyper.ErrConflictingBodies) {
			t.Errorf("%s: expected ErrConflictingBodies, got %v", name, response.Err())
		}
	}
}
