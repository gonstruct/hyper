package hyper_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
)

type task struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func faked(t *testing.T, fake http.RoundTripper) hyper.Client {
	t.Helper()

	return hyper.New(context.Background(), hyper.Base("https://api.example.com"), hyper.Transport(fake), hyper.Untraced())
}

func TestTheFakeAnswersByMethodAndPattern(t *testing.T) {
	fake := hyper.Fake(t)
	fake.On(hyper.POST, "/tasks").Reply(http.StatusAccepted, task{ID: "task_1", Status: "queued"})
	fake.On(hyper.GET, "https://api.example.com/tasks/*").Reply(http.StatusOK, task{ID: "task_1", Status: "done"})

	api := faked(t, fake)

	created, err := api.Post("/tasks", map[string]any{"title": "write"}).JSON[task]()
	if err != nil || created.ID != "task_1" {
		t.Fatalf("create: %+v %v", created, err)
	}
	fetched, err := api.Get("/tasks/task_1").JSON[task]()
	if err != nil || fetched.Status != "done" {
		t.Fatalf("fetch: %+v %v", fetched, err)
	}
	if fake.Sent()[0].Header("Content-Type") != "application/json" {
		t.Error("the fake should see the real request headers")
	}
}

func TestASequenceAnswersInOrderAndThenFails(t *testing.T) {
	fake := hyper.Fake(&recordingT{TB: t})
	fake.On(hyper.GET, "/tasks/*").Sequence(
		hyper.Reply(http.StatusOK, task{Status: "running"}),
		hyper.Reply(http.StatusOK, task{Status: "succeeded"}),
	)
	api := faked(t, fake)

	first, _ := api.Get("/tasks/task_1").JSON[task]()
	second, _ := api.Get("/tasks/task_1").JSON[task]()
	if first.Status != "running" || second.Status != "succeeded" {
		t.Errorf("sequence: %s then %s", first.Status, second.Status)
	}
	if err := api.Get("/tasks/task_1").Err(); err == nil {
		t.Error("a third call should fail once the sequence is exhausted")
	}
}

func TestAConditionNarrowsAnExpectation(t *testing.T) {
	fake := hyper.Fake(t)
	fake.On(hyper.POST, "/tasks").
		When(func(sent hyper.Sent) bool { return sent.JSON("title") == "forbidden" }).
		Reply(http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": "title not allowed"}})
	fake.On(hyper.POST, "/tasks").Reply(http.StatusAccepted, task{ID: "task_1"})
	api := faked(t, fake)

	response := api.Post("/tasks", map[string]any{"title": "forbidden"})
	if response.Status() != http.StatusUnprocessableEntity || response.Field("error.message") != "title not allowed" {
		t.Errorf("the conditional expectation did not answer: %d", response.Status())
	}
	if !api.Post("/tasks", map[string]any{"title": "write"}).Ok() {
		t.Error("the general expectation did not answer")
	}
}

func TestRepliesCanCarryHeadersAndRawBodies(t *testing.T) {
	fake := hyper.Fake(t)
	fake.On(hyper.GET, "/text").Sequence(hyper.Respond(http.StatusOK, "plain", hyper.H{"Content-Type": "text/plain"}))
	fake.On(hyper.GET, "/bytes").Reply(http.StatusOK, []byte{1, 2, 3})
	fake.On(hyper.DELETE, "/thing").Reply(http.StatusNoContent, nil)
	api := faked(t, fake)

	response := api.Get("/text")
	if text, _ := response.Text(); text != "plain" || response.Header("Content-Type") != "text/plain" {
		t.Errorf("text reply: %q %q", text, response.Header("Content-Type"))
	}
	if data, _ := api.Get("/bytes").Bytes(); len(data) != 3 {
		t.Errorf("bytes reply: %v", data)
	}
	if err := api.Delete("/thing").Err(); err != nil {
		t.Errorf("nil reply: %v", err)
	}
}

func TestAssertionsReadWhatWasSent(t *testing.T) {
	fake := hyper.Fake(t)
	fake.On(hyper.POST, "/tasks").Reply(http.StatusAccepted, task{ID: "task_1"})
	api := faked(t, fake)

	api.Post("/tasks", map[string]any{"title": "write"}, hyper.Header("Idempotency-Key", "k"), hyper.Query{"dry": true})

	fake.AssertSent(t, hyper.POST, "/tasks", func(sent hyper.Sent) bool {
		return sent.Header("Idempotency-Key") == "k" && sent.JSON("title") == "write" && sent.Query("dry") == "true"
	})
	fake.AssertNotSent(t, hyper.GET, "/tasks/*")
	fake.AssertCount(t, 1)

	recorder := &recordingT{TB: t}
	fake.AssertSent(recorder, hyper.DELETE, "/tasks")
	fake.AssertNotSent(recorder, hyper.POST, "/tasks")
	fake.AssertCount(recorder, 2)
	if recorder.failures != 3 {
		t.Errorf("each false assertion should fail, got %d failures", recorder.failures)
	}
}

func TestAnUnexpectedRequestFailsTheTestAndTheCall(t *testing.T) {
	recorder := &recordingT{TB: t}
	fake := hyper.Fake(recorder)

	err := faked(t, fake).Get("/nothing").Err()
	if err == nil || recorder.failures == 0 {
		t.Error("an unexpected request should fail the test and the call")
	}

	// Without a test handed over, only the call fails.
	quiet := hyper.Fake()
	if err := faked(t, quiet).Get("/nothing").Err(); err == nil {
		t.Error("an unexpected request should still fail the call")
	}
}

// recordingT counts failures instead of failing, so the fake's own
// behaviour on a failing test can be asserted.
type recordingT struct {
	testing.TB

	failures int
}

func (r *recordingT) Errorf(string, ...any) { r.failures++ }
func (r *recordingT) Helper()               {}
