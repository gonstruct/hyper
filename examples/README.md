# hyper by example

The package, by example. Every example compiles. A request is one line:

```go
response := client.Verb(url, input, options...)
```

The verb returns a Response. Read it whichever way the call wants:

```go
models, err := studio.Get("/v1/models").JSON[[]Model]("data")   // folded

response := studio.Get("/v1/models")                             // step by step
if !response.Ok() {
    return response.Error()
}
models, err := response.JSON[[]Model]("data")
```

The context lives in the client. Options trail and most calls have none.

| Example | Shows |
|---|---|
| `01-requests` | The verbs, what the input argument means, reading the response |
| `02-client` | The client: context, base URL, token, timeout, retry, transport |
| `03-errors` | What Error() can be and how to read it |
| `04-bodies` | Form, multipart, bytes, streams in and out |
| `05-provider` | A provider client the way an app writes one |
| `06-testing` | Faking the transport and asserting what was sent |
