// Package hyper is an HTTP client for talking to JSON APIs from Go, shaped
// after Laravel's Http client and trimmed to what Go cannot do natively.
//
// A request is one line: the method, the URL, what goes in.
//
//	api := hyper.New(ctx, hyper.Base("https://api.example.com"), hyper.BearerToken(token))
//
//	projects, err := api.Get("/projects", hyper.Query{"archived": false}).JSON[[]Project]("data")
//	task, err := api.Post("/tasks", NewTask{Title: "Write the README"}).JSON[Task]()
//	err := api.Delete("/tasks/" + id).Err()
//
// The verb returns a Response that carries whatever went wrong; JSON, Bytes,
// Text and Err hand that error back, so a folded call is still one check.
// Ok, Status, Header and Field read the response without decoding it.
//
// The context lives in the client. Every request is traced through
// OpenTelemetry when the application has registered a tracer provider, and
// costs nothing when it has not.
package hyper

import (
	"context"
	"net/http"
)

// Method is an HTTP method. The constants are the ones hyper has a verb for.
type Method string

const (
	GET    Method = http.MethodGet
	POST   Method = http.MethodPost
	PUT    Method = http.MethodPut
	PATCH  Method = http.MethodPatch
	DELETE Method = http.MethodDelete
	HEAD   Method = http.MethodHead
)

// Client is everything that is true for every request to one provider: the
// context, the base URL, headers, timeout, retry policy and transport. It is
// a value; a call never changes it.
type Client struct {
	ctx      context.Context
	settings settings
}

// New builds a client. The context is carried by every request the client
// sends, so cancellation and tracing come from the caller's scope.
func New(ctx context.Context, options ...Option) Client {
	return Client{ctx: ctx, settings: newSettings().apply(options...)}
}

// With returns a client with more options applied. The receiver is untouched.
func (c Client) With(options ...Option) Client {
	return Client{ctx: c.ctx, settings: c.settings.clone().apply(options...)}
}

// WithContext returns the same client bound to another context.
func (c Client) WithContext(ctx context.Context) Client {
	return Client{ctx: ctx, settings: c.settings.clone()}
}

// Get sends a GET. Input travels as a query string through the Query option.
func (c Client) Get(url string, options ...Option) *Response {
	return c.Send(GET, url, nil, options...)
}

// Head sends a HEAD.
func (c Client) Head(url string, options ...Option) *Response {
	return c.Send(HEAD, url, nil, options...)
}

// Delete sends a DELETE. A body, when a provider wants one, goes through Send.
func (c Client) Delete(url string, options ...Option) *Response {
	return c.Send(DELETE, url, nil, options...)
}

// Post sends a POST with input as the body. A struct or map is JSON; Form,
// Bytes, Stream and Multipart say otherwise; nil sends no body.
func (c Client) Post(url string, input any, options ...Option) *Response {
	return c.Send(POST, url, input, options...)
}

// Put sends a PUT with input as the body, like Post.
func (c Client) Put(url string, input any, options ...Option) *Response {
	return c.Send(PUT, url, input, options...)
}

// Patch sends a PATCH with input as the body, like Post.
func (c Client) Patch(url string, input any, options ...Option) *Response {
	return c.Send(PATCH, url, input, options...)
}

// Send is the generic form of every verb.
func (c Client) Send(method Method, url string, input any, options ...Option) *Response {
	return send(c.ctx, method, url, input, c.settings.clone().apply(options...))
}

// The package-level verbs are for one-off requests without a client. The
// context is the first argument because there is nowhere else for it.

func Get(ctx context.Context, url string, options ...Option) *Response {
	return New(ctx).Get(url, options...)
}

func Head(ctx context.Context, url string, options ...Option) *Response {
	return New(ctx).Head(url, options...)
}

func Delete(ctx context.Context, url string, options ...Option) *Response {
	return New(ctx).Delete(url, options...)
}

func Post(ctx context.Context, url string, input any, options ...Option) *Response {
	return New(ctx).Post(url, input, options...)
}

func Put(ctx context.Context, url string, input any, options ...Option) *Response {
	return New(ctx).Put(url, input, options...)
}

func Patch(ctx context.Context, url string, input any, options ...Option) *Response {
	return New(ctx).Patch(url, input, options...)
}

func Send(ctx context.Context, method Method, url string, input any, options ...Option) *Response {
	return New(ctx).Send(method, url, input, options...)
}
