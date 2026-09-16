package hyper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/tidwall/gjson"
)

// fake is the network, faked: an http.RoundTripper handed to a client
// through Transport, so there is nothing global and tests run in parallel.
// It answers requests from what a test put in and records what was sent.
type fake struct {
	t            testing.TB
	mutex        sync.Mutex
	expectations []*Expectation
	sent         []Sent
}

// Expectation is one rule: a method, a URL pattern, an optional condition on
// the request, and what to reply.
type Expectation struct {
	method  Method
	pattern string
	when    func(Sent) bool
	replies []Answer
	served  int
}

// Answer is one reply. Body is JSON unless it is a []byte, a string or nil.
type Answer struct {
	Status  int
	Body    any
	Headers H
}

// Sent is a request the fake received, decoded enough to assert on.
type Sent struct {
	Method  Method
	URL     string
	Path    string
	Headers http.Header
	Body    []byte
}

// Fake makes a fake network. Given the test, a request it has no answer for
// fails the test with the request printed; without it, only the call fails.
func Fake(t ...testing.TB) *fake {
	instance := &fake{}
	if len(t) > 0 {
		instance.t = t[0]
	}

	return instance
}

// On registers an expectation. Patterns are matched against the full URL and
// against the path alone, so both forms work; * matches one path segment.
func (f *fake) On(method Method, pattern string) *Expectation {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	expectation := &Expectation{method: method, pattern: pattern}
	f.expectations = append(f.expectations, expectation)

	return expectation
}

// When adds a condition on the request the expectation applies to.
func (e *Expectation) When(when func(Sent) bool) *Expectation {
	e.when = when

	return e
}

// Reply sets the one answer this expectation gives, every time.
func (e *Expectation) Reply(status int, body any) *Expectation {
	e.replies = []Answer{{Status: status, Body: body}}
	e.served = -1

	return e
}

// Sequence sets the answers in order. A request past the last one fails the test.
func (e *Expectation) Sequence(replies ...Answer) *Expectation {
	e.replies = replies

	return e
}

// Reply is an Answer, for Sequence.
func Reply(status int, body any) Answer {
	return Answer{Status: status, Body: body}
}

// Respond is an Answer with headers.
func Respond(status int, body any, headers H) Answer {
	return Answer{Status: status, Body: body, Headers: headers}
}

// RoundTrip is what hyper calls.
func (f *fake) RoundTrip(request *http.Request) (*http.Response, error) {
	sent, err := record(request)
	if err != nil {
		return nil, err
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.sent = append(f.sent, sent)

	for _, expectation := range f.expectations {
		if !expectation.matches(sent) {
			continue
		}

		reply, ok := expectation.next()
		if !ok {
			f.failf("hyper fake: %s %s was sent more times than the sequence has answers", sent.Method, sent.URL)
			return nil, fmt.Errorf("hyper fake: sequence for %s %s exhausted", sent.Method, sent.URL)
		}

		return reply.response(request), nil
	}

	f.failf("hyper fake: unexpected request %s %s\n%s", sent.Method, sent.URL, sent.Body)

	return nil, fmt.Errorf("hyper fake: no answer for %s %s", sent.Method, sent.URL)
}

func (f *fake) failf(format string, args ...any) {
	if f.t != nil {
		f.t.Errorf(format, args...)
	}
}

func (e *Expectation) matches(sent Sent) bool {
	if e.method != sent.Method {
		return false
	}
	if !matchPattern(e.pattern, sent.URL) && !matchPattern(e.pattern, sent.Path) {
		return false
	}
	if e.when != nil && !e.when(sent) {
		return false
	}

	return true
}

func (e *Expectation) next() (Answer, bool) {
	if len(e.replies) == 0 {
		return Answer{}, false
	}
	if e.served < 0 {
		return e.replies[0], true
	}
	if e.served >= len(e.replies) {
		return Answer{}, false
	}
	reply := e.replies[e.served]
	e.served++

	return reply, true
}

func matchPattern(pattern, target string) bool {
	if pattern == target {
		return true
	}

	patternParts := strings.Split(strings.TrimSuffix(pattern, "/"), "/")
	targetParts := strings.Split(strings.TrimSuffix(strings.SplitN(target, "?", 2)[0], "/"), "/")
	if len(patternParts) != len(targetParts) {
		return false
	}
	for index := range patternParts {
		if patternParts[index] != "*" && patternParts[index] != targetParts[index] {
			return false
		}
	}

	return true
}

func record(request *http.Request) (Sent, error) {
	sent := Sent{
		Method:  Method(request.Method),
		URL:     request.URL.String(),
		Path:    request.URL.Path,
		Headers: request.Header.Clone(),
	}
	if request.Body != nil {
		data, err := io.ReadAll(request.Body)
		if err != nil {
			return sent, err
		}
		sent.Body = data
	}

	return sent, nil
}

func (reply Answer) response(request *http.Request) *http.Response {
	header := http.Header{}
	for name, value := range reply.Headers {
		header.Set(name, value)
	}

	var data []byte
	switch typed := reply.Body.(type) {
	case nil:
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		data, _ = json.Marshal(typed)
		if header.Get("Content-Type") == "" {
			header.Set("Content-Type", "application/json")
		}
	}

	return &http.Response{
		StatusCode:    reply.Status,
		Status:        fmt.Sprintf("%d %s", reply.Status, http.StatusText(reply.Status)),
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		Request:       request,
	}
}

// Header reads one request header.
func (sent Sent) Header(name string) string { return sent.Headers.Get(name) }

// JSON reads one key of a JSON request body, as a string.
func (sent Sent) JSON(path string) string { return gjson.GetBytes(sent.Body, path).String() }

// Query reads one query parameter.
func (sent Sent) Query(name string) string {
	parts := strings.SplitN(sent.URL, "?", 2)
	if len(parts) < 2 {
		return ""
	}
	for _, pair := range strings.Split(parts[1], "&") {
		key, value, _ := strings.Cut(pair, "=")
		if key == name {
			return value
		}
	}

	return ""
}

// Sent lists every request the fake received.
func (f *fake) Sent() []Sent {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return append([]Sent(nil), f.sent...)
}

// AssertSent fails unless a matching request was sent. Conditions narrow it.
func (f *fake) AssertSent(t testing.TB, method Method, pattern string, conditions ...func(Sent) bool) {
	t.Helper()

	for _, sent := range f.Sent() {
		if f.matched(sent, method, pattern, conditions) {
			return
		}
	}
	t.Errorf("hyper fake: expected %s %s to have been sent", method, pattern)
}

// AssertNotSent fails if a matching request was sent.
func (f *fake) AssertNotSent(t testing.TB, method Method, pattern string, conditions ...func(Sent) bool) {
	t.Helper()

	for _, sent := range f.Sent() {
		if f.matched(sent, method, pattern, conditions) {
			t.Errorf("hyper fake: expected %s %s not to have been sent", method, pattern)
			return
		}
	}
}

// AssertCount fails unless exactly count requests were sent.
func (f *fake) AssertCount(t testing.TB, count int) {
	t.Helper()

	if sent := len(f.Sent()); sent != count {
		t.Errorf("hyper fake: expected %d requests, %d were sent", count, sent)
	}
}

func (f *fake) matched(sent Sent, method Method, pattern string, conditions []func(Sent) bool) bool {
	if sent.Method != method || (!matchPattern(pattern, sent.URL) && !matchPattern(pattern, sent.Path)) {
		return false
	}
	for _, condition := range conditions {
		if !condition(sent) {
			return false
		}
	}

	return true
}
