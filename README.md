# hyper

An HTTP client for talking to JSON APIs from Go, shaped after Laravel's
`Http` client and trimmed to what Go cannot do natively. A request is one
line: the method, the URL, what goes in.

```go
studio := hyper.New(ctx, hyper.Base("https://api.example.com"), hyper.BearerToken(key))

models, err     := studio.Get("/v1/models", hyper.Query{"enabled": true}).JSON[[]Model]("data")
generation, err := studio.Post("/v1/generations", request).JSON[Generation]()
err             := studio.Delete("/v1/uploads/" + id).Err()
```

Requires Go 1.27: `JSON` is a generic method.

## Reading a response

The verb returns a `Response` that always exists and carries whatever went
wrong. Read it folded, or step by step when the status matters first:

```go
response := studio.Post("/v1/generations", request)
if !response.Ok() {
    return response.Error()
}
generation, err := response.JSON[Generation]()
```

| Method | Gives |
|---|---|
| `Ok()` | true for a 2xx that arrived |
| `Status()` | the HTTP status, 0 when nothing came back |
| `Header(name)`, `Headers()` | response headers |
| `Error()`, `Err()` | nil, a transport error, or a `*StatusError` |
| `JSON[T](key...)` | the body decoded, or the key of it |
| `Bytes()`, `Text()` | the body as is |
| `Field(path)` | one key of a JSON body as a string, no decoding |

Errors are types, never strings to parse:

```go
errors.Is(err, hyper.ErrTransport)   // nothing came back: DNS, refused, timeout, cancelled
errors.As(err, &statusError)         // 4xx or 5xx, with .Code and .Response attached
errors.As(err, &decodeError)         // a 2xx whose body was not the type asked for
```

## Sending

The input after the URL says how it travels:

| Input | Travels as |
|---|---|
| a struct or a map | JSON body |
| `hyper.Query{...}` | query string |
| `hyper.Form{...}` | `application/x-www-form-urlencoded` |
| `hyper.Bytes(...)` with `hyper.ContentType(...)` | raw body |
| `hyper.Stream(reader, size)` | raw body, streamed, not retried |
| `hyper.Multipart{Fields, Files}` | `multipart/form-data` |
| `nil` | nothing |

Options trail the input, and the same options configure a client:
`Base`, `Header`, `Headers`, `BearerToken`, `BasicAuth`, `Accept`,
`ContentType`, `Timeout`, `Retry`, `RetryWhen`, `Transport`, `Untraced`,
`Sink`. A client is a value; `With` and `WithContext` derive new ones.

`Retry(3, hyper.Backoff(time.Second, 10*time.Second))` retries transient
results only: no response, 429, 5xx. `Retry-After` is honoured.
`Sink(writer)` streams a response body out without buffering it.

## Tracing

Every request goes through `otelhttp`, which reports to the global
OpenTelemetry tracer provider. When the application registers one, every
call is a client span under the caller's span, with retries as one span per
attempt; when it does not, nothing happens. `Untraced()` turns it off,
`Transport(rt)` puts your own transport underneath.

## Testing

`hypertest` is a fake transport. No globals, so tests run in parallel, and a
request nobody expected fails the test with the request printed.

```go
fake := hypertest.New(t)
fake.On(hyper.POST, "/v1/generations").Reply(http.StatusAccepted, Generation{ID: "gen_1"})
fake.On(hyper.GET, "/v1/generations/*").Sequence(
    hypertest.Reply(http.StatusOK, Generation{Status: "running"}),
    hypertest.Reply(http.StatusOK, Generation{Status: "succeeded"}),
)

studio := hyper.New(ctx, hyper.Base("https://api.example.com"), hyper.Transport(fake))

fake.AssertSent(t, hyper.POST, "/v1/generations", func(sent hypertest.Sent) bool {
    return sent.Header("Idempotency-Key") == "k" && sent.JSON("model") == "nano-banana"
})
```

## Examples

`examples/` walks from the smallest call to a full provider client and its
tests. Every example compiles.

## Status

Early. The API in this README is the one intended to last; anything not in
it may change.
