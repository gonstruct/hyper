# hyper by example

Every example compiles and runs against a made-up tasks API at
`https://api.example.com`. A request is one line:

```go
response := client.Verb(url, input, options...)
```

The verb returns a Response. Read it whichever way the call wants:

```go
projects, err := api.Get("/projects").JSON[[]Project]("data")   // folded

response := api.Get("/projects")                                 // step by step
if !response.Ok() {
    return response.Error()
}
projects, err := response.JSON[[]Project]("data")
```

| Example | Shows |
|---|---|
| `01-requests` | The verbs, what the input argument means, reading the response |
| `02-client` | The client: context, base URL, token, timeout, retry, transport |
| `03-errors` | What Error() can be and how to read it |
| `04-bodies` | Form, multipart, bytes, streams in and out |
| `05-provider` | An API client the way an app writes one |
| `06-testing` | Faking the transport and asserting what was sent |
