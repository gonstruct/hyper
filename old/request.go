package hyper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func NewRequest(options ...requestOption) *Request {
	return new(Request).apply(options...)
}

type Request struct {
	base   string
	method string
	url    string

	queryParams *queryParams
	headers     *headers
	body        body
}

func (request *Request) apply(options ...requestOption) *Request {
	for _, option := range options {
		option(request)
	}
	return request
}

func (request *Request) makeUrl(path string) string {
	u, _ := url.Parse(path)

	if request.base != "" {
		u, _ = url.Parse(request.base)
		u = u.JoinPath(path)
	}

	if request.queryParams != nil {
		u.RawQuery = request.queryParams.Encode()
	}

	return u.String()
}

func (request *Request) do() *Response {
	var body io.Reader
	if request.body != nil {
		body = request.body.Read()
	}

	start := time.Now()
	req, err := http.NewRequest(request.method, request.makeUrl(request.url), body)
	if err != nil {
		return request.respond(nil, request.error(err, "failed to create request"))
	}

	if request.headers != nil {
		req.Header = request.headers.Header
	}

	httpClient := getTestingClient(request.method, req.URL.String())
	if httpClient == nil {
		fmt.Println("[hyper] using default http client")
		httpClient = http.DefaultClient
	} else {
		fmt.Println("[hyper] using testing http client")
	}

	res, err := httpClient.Do(req)
	// TODO: fix this better
	if err != nil || res == nil {
		fmt.Printf("[hyper] %s %s -> error: %v (%s)\n", request.method, req.URL.String(), err, time.Since(start))
	} else {
		fmt.Printf("[hyper] %s %s -> %d (%s)\n", request.method, req.URL.String(), res.StatusCode, time.Since(start))
	}
	return request.respond(res, err)
}

func (request *Request) respond(res *http.Response, err error) *Response {
	// var body []byte

	// if res != nil && res.Body != nil {
	// 	defer res.Body.Close()
	// 	body, err = io.ReadAll(res.Body)
	// }

	return &Response{
		R:       res,
		request: request,
		err:     err,
		// body:    body,
	}
}

// TODO: wrap this in some kind of logging thing
func (r *Request) error(err error, msg ...string) error {
	var prefix string
	if len(msg) == 1 {
		prefix = msg[0] + " "
	}

	return fmt.Errorf("[hyper] %s(method: %s, url: %s): %w", prefix, r.method, r.makeUrl(r.url), err)
}
