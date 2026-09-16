# hyper

An HTTP client for talking to JSON APIs from Go, shaped after Laravel's
`Http` client and trimmed to what Go cannot do natively. A request is one
line: the method, the URL, what goes in.

```go
api := hyper.New(ctx, hyper.Base("https://api.example.com"), hyper.BearerToken(token))

projects, err := api.Get("/projects", hyper.Query{"archived": false}).JSON[[]Project]("data")
task, err     := api.Post("/tasks", NewTask{Title: "Write the README"}).JSON[Task]()
err           := api.Delete("/tasks/" + id).Err()
```

Requires Go 1.27: `JSON` is a generic method.

## Reading a response

The verb returns a `Response` that always exists and carries whatever went
wrong. Read it folded, or step by step when the status matters first:

```go
response := api.Post("/tasks", task)
if !response.Ok() {
    return response.Error()
}
created, err := response.JSON[Task]()
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

`hyper.NewFake` is a fake transport: it answers requests from what a test put
in and records what was sent. No globals, so tests run in parallel, and a
request nobody expected fails the test with the request printed.

```go
fake := hyper.NewFake(t)
fake.On(hyper.POST, "/tasks").Reply(http.StatusAccepted, Task{ID: "task_1"})
fake.On(hyper.GET, "/tasks/*").Sequence(
    hyper.Reply(http.StatusOK, Task{Status: "running"}),
    hyper.Reply(http.StatusOK, Task{Status: "done"}),
)

api := hyper.New(ctx, hyper.Base("https://api.example.com"), hyper.Transport(fake))

fake.AssertSent(t, hyper.POST, "/tasks", func(sent hyper.Sent) bool {
    return sent.Header("Idempotency-Key") == "k1" && sent.JSON("title") == "Write the README"
})
```

## Examples

`examples/` walks from the smallest call to a full provider client and its
tests. Every example compiles.

## Status

Early. The API in this README is the one intended to last; anything not in
it may change.
