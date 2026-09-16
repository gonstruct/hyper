package hyper

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

var testingClients []testClient

func getTestingClient(method, url string) *http.Client {
	for _, client := range testingClients {
		if client.method == method && client.urlMatcher == url {
			return client.client
		}
	}

	return nil
}

type testClient struct {
	client *http.Client

	method     string
	urlMatcher string
	do         func(req *http.Request) (*http.Response, error)
}

type testRoundTripper struct {
	method     string
	urlMatcher string
	do         func(req *http.Request) (*http.Response, error)
}

func (t *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == t.method && req.URL.String() == t.urlMatcher {
		return t.do(req)
	}

	fmt.Printf("No test client found for %s %s\n", req.Method, req.URL.String())

	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`))),
		Header:     make(http.Header),
	}, nil
}

func Fake(fakers map[string]func(req *http.Request) (*http.Response, error)) {
	if testingClients == nil {
		testingClients = []testClient{}
	}

	for methodUrl, do := range fakers {
		var method, urlMatcher string
		n, _ := fmt.Sscanf(methodUrl, "%s %s", &method, &urlMatcher)
		if n != 2 {
			continue
		}

		fmt.Println("[hyper] adding testing client for", method, urlMatcher)

		testingClients = append(testingClients, testClient{
			client: &http.Client{Transport: &testRoundTripper{
				method:     method,
				urlMatcher: urlMatcher,
				do:         do,
			}},
			method:     method,
			urlMatcher: urlMatcher,
			do:         do,
		})
	}
}
