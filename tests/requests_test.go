package tests_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gonstruct/hyper"
)

func TestEveryVerbSendsItsMethod(t *testing.T) {
	c := client(t)

	cases := map[string]*hyper.Response{
		"GET":     c.Get("/x"),
		"HEAD":    c.Head("/x"),
		"DELETE":  c.Delete("/x"),
		"POST":    c.Post("/x", nil),
		"PUT":     c.Put("/x", nil),
		"PATCH":   c.Patch("/x", nil),
		"OPTIONS": c.Send("OPTIONS", "/x", nil),
	}

	for method, response := range cases {
		if !response.Ok() {
			t.Errorf("%s: %v", method, response.Error())
			continue
		}
		if method == "HEAD" {
			continue // a HEAD has no body to echo
		}
		if got := seen(t, response).Method; got != method {
			t.Errorf("expected %s, the server saw %s", method, got)
		}
	}
}

func TestURLsResolveAgainstTheBase(t *testing.T) {
	s := server(t)

	for _, base := range []string{s.URL, s.URL + "/"} {
		for _, path := range []string{"/things", "things"} {
			c := hyper.New(context.Background(), hyper.Base(base), hyper.Untraced())
			if got := seen(t, c.Get(path)).Path; got != "/things" {
				t.Errorf("base %q path %q: server saw %q", base, path, got)
			}
		}
	}

	// An absolute URL ignores the base.
	c := hyper.New(context.Background(), hyper.Base("http://nowhere.invalid"), hyper.Untraced())
	if got := seen(t, c.Get(s.URL+"/absolute")).Path; got != "/absolute" {
		t.Errorf("absolute URL was rewritten: %q", got)
	}
}

func TestQueryStrings(t *testing.T) {
	c := client(t)

	got := seen(t, c.Get("/things", hyper.Query{"enabled": true, "tags": []string{"a", "b"}, "n": 3}))
	for _, want := range []string{"enabled=true", "tags=a&tags=b", "n=3"} {
		if !strings.Contains(got.Query, want) {
			t.Errorf("expected %q in %q", want, got.Query)
		}
	}

	// A query already on the URL is kept and merged.
	got = seen(t, c.Get("/things?page=2", hyper.Query{"per_page": 10}))
	if !strings.Contains(got.Query, "page=2") || !strings.Contains(got.Query, "per_page=10") {
		t.Errorf("queries did not merge: %q", got.Query)
	}

	// A client can carry default query parameters.
	got = seen(t, client(t, hyper.Query{"version": "2"}).Get("/things"))
	if got.Query != "version=2" {
		t.Errorf("client query not sent: %q", got.Query)
	}
}

func TestHeaders(t *testing.T) {
	c := client(t, hyper.Header("X-Client", "1"), hyper.Headers(hyper.H{"X-Shared": "client"}))

	got := seen(t, c.Get("/x", hyper.Header("X-Call", "2"), hyper.Headers(hyper.H{"X-Shared": "call"})))
	if got.Headers["X-Client"] != "1" || got.Headers["X-Call"] != "2" {
		t.Errorf("headers did not merge: %+v", got.Headers)
	}
	if got.Headers["X-Shared"] != "call" {
		t.Errorf("the call's header should replace the client's, got %q", got.Headers["X-Shared"])
	}
	if got.Headers["Accept"] != "application/json" {
		t.Errorf("expected the JSON accept header by default, got %q", got.Headers["Accept"])
	}
}

func TestAuthAndAccept(t *testing.T) {
	c := client(t)

	if got := seen(t, c.Get("/x", hyper.BearerToken("sk_1"))).Headers["Authorization"]; got != "Bearer sk_1" {
		t.Errorf("bearer: %q", got)
	}
	if got := seen(t, c.Get("/x", hyper.BasicAuth("user", "pass"))).Headers["Authorization"]; got != "Basic dXNlcjpwYXNz" {
		t.Errorf("basic: %q", got)
	}
	if got := seen(t, c.Get("/x", hyper.Accept("text/plain"))).Headers["Accept"]; got != "text/plain" {
		t.Errorf("accept: %q", got)
	}
}

func TestPackageLevelVerbsNeedNoClient(t *testing.T) {
	s := server(t)
	ctx := context.Background()

	if got := seen(t, hyper.Get(ctx, s.URL+"/one", hyper.Untraced())).Method; got != "GET" {
		t.Errorf("Get: %s", got)
	}
	if got := seen(t, hyper.Post(ctx, s.URL+"/two", map[string]int{"a": 1}, hyper.Untraced())).Body; got != `{"a":1}` {
		t.Errorf("Post: %s", got)
	}
	if got := seen(t, hyper.Put(ctx, s.URL+"/three", nil, hyper.Untraced())).Method; got != "PUT" {
		t.Errorf("Put: %s", got)
	}
	if got := seen(t, hyper.Patch(ctx, s.URL+"/four", nil, hyper.Untraced())).Method; got != "PATCH" {
		t.Errorf("Patch: %s", got)
	}
	if err := hyper.Delete(ctx, s.URL+"/five", hyper.Untraced()).Err(); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if err := hyper.Head(ctx, s.URL+"/six", hyper.Untraced()).Err(); err != nil {
		t.Errorf("Head: %v", err)
	}
	if got := seen(t, hyper.Send(ctx, hyper.POST, s.URL+"/seven", nil, hyper.Untraced())).Method; got != "POST" {
		t.Errorf("Send: %s", got)
	}
}
