package hyper_test

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
)

func TestOkStatusAndHeaders(t *testing.T) {
	response := client(t).Get("/text")

	if !response.Ok() || response.Status() != http.StatusOK {
		t.Fatalf("expected a 200, got %d (%v)", response.Status(), response.Error())
	}
	if response.Header("Content-Type") != "text/plain" || response.Headers().Get("Content-Type") != "text/plain" {
		t.Errorf("headers not readable: %v", response.Headers())
	}
	if response.Method != hyper.GET || !bytes.HasSuffix([]byte(response.URL), []byte("/text")) {
		t.Errorf("the response does not say what was sent: %s %s", response.Method, response.URL)
	}
}

func TestNoContentIsOk(t *testing.T) {
	response := client(t).Delete("/empty")
	if !response.Ok() || response.Status() != http.StatusNoContent || response.Err() != nil {
		t.Fatalf("204 should be fine: %d %v", response.Status(), response.Err())
	}
	if data, err := response.Bytes(); err != nil || len(data) != 0 {
		t.Errorf("expected an empty body, got %q %v", data, err)
	}
}

func TestBytesTextAndField(t *testing.T) {
	c := client(t)

	data, err := c.Get("/text").Bytes()
	if err != nil || string(data) != "plain text" {
		t.Errorf("bytes: %q %v", data, err)
	}
	text, err := c.Get("/text").Text()
	if err != nil || text != "plain text" {
		t.Errorf("text: %q %v", text, err)
	}
	response := c.Get("/envelope")
	if response.Field("meta.total") != "2" || response.Field("data.1.id") != "b" || response.Field("nope") != "" {
		t.Errorf("Field: %q %q %q", response.Field("meta.total"), response.Field("data.1.id"), response.Field("nope"))
	}
}

func TestJSONDecodesTheRootOrAKey(t *testing.T) {
	c := client(t)

	whole, err := c.Get("/envelope").JSON[map[string]any]()
	if err != nil || whole["meta"] == nil {
		t.Errorf("root: %v %v", whole, err)
	}

	items, err := c.Get("/envelope").JSON[[]struct {
		ID string `json:"id"`
	}]("data")
	if err != nil || len(items) != 2 || items[1].ID != "b" {
		t.Errorf("key: %v %v", items, err)
	}

	total, err := c.Get("/envelope").JSON[int]("meta.total")
	if err != nil || total != 2 {
		t.Errorf("nested key: %d %v", total, err)
	}
}

func TestDecodeErrors(t *testing.T) {
	c := client(t)
	var decode *hyper.DecodeError

	if _, err := c.Get("/envelope").JSON[[]string]("nope"); !errors.As(err, &decode) {
		t.Errorf("missing key: expected DecodeError, got %v", err)
	}
	if _, err := c.Get("/text").JSON[map[string]any](); !errors.As(err, &decode) || decode.Response == nil {
		t.Errorf("wrong shape: expected DecodeError with the response, got %v", err)
	}
	if _, err := c.Get("/envelope").JSON[int]("data"); !errors.As(err, &decode) {
		t.Errorf("wrong type: expected DecodeError, got %v", err)
	}
}

func TestStatusErrors(t *testing.T) {
	c := client(t)

	for _, tc := range []struct {
		path    string
		code    int
		message string
	}{
		{"/missing", http.StatusNotFound, "no such thing"},
		{"/broken", http.StatusInternalServerError, "it broke"},
		{"/conflict", http.StatusConflict, ""},
	} {
		response := c.Get(tc.path)
		if response.Ok() || response.Status() != tc.code {
			t.Errorf("%s: expected %d, got %d", tc.path, tc.code, response.Status())
		}

		var status *hyper.StatusError
		if !errors.As(response.Error(), &status) || status.Code != tc.code || status.Response != response {
			t.Errorf("%s: expected a StatusError carrying the response, got %v", tc.path, response.Error())
			continue
		}
		if got := status.Response.Field("error.message") + status.Response.Field("message"); got != tc.message {
			t.Errorf("%s: message %q", tc.path, got)
		}
		if _, err := response.JSON[map[string]any](); !errors.Is(err, status) {
			t.Errorf("%s: JSON should hand back the status error, got %v", tc.path, err)
		}
		if _, err := response.Bytes(); !errors.Is(err, status) {
			t.Errorf("%s: Bytes should hand back the status error, got %v", tc.path, err)
		}
	}
}

func TestStatusErrorReads(t *testing.T) {
	err := client(t).Get("/missing").Err()
	if !bytes.Contains([]byte(err.Error()), []byte("GET")) || !bytes.Contains([]byte(err.Error()), []byte("404")) {
		t.Errorf("the error should name the method and status: %v", err)
	}
}

func TestSinkStreamsTheBodyOut(t *testing.T) {
	var sink bytes.Buffer
	response := client(t).Get("/text", hyper.Sink(&sink))

	if err := response.Err(); err != nil || sink.String() != "plain text" {
		t.Fatalf("sink: %q %v", sink.String(), err)
	}
	if _, err := response.Bytes(); !errors.Is(err, hyper.ErrNoBody) {
		t.Errorf("Bytes after a sink: %v", err)
	}
	if _, err := response.JSON[string](); !errors.Is(err, hyper.ErrNoBody) {
		t.Errorf("JSON after a sink: %v", err)
	}
}

func TestAFailedStatusStillFillsTheSink(t *testing.T) {
	var sink bytes.Buffer
	response := client(t).Get("/missing", hyper.Sink(&sink))

	if response.Ok() || sink.Len() == 0 {
		t.Errorf("the error body should reach the sink: %q", sink.String())
	}
}
