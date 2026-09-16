package tests_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
	"github.com/gonstruct/hyper/hypertest"
)

type generation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func faked(t *testing.T, fake *hypertest.Fake) hyper.Client {
	t.Helper()

	return hyper.New(context.Background(), hyper.Base("https://api.test"), hyper.Transport(fake), hyper.Untraced())
}

func TestTheFakeAnswersByMethodAndPattern(t *testing.T) {
	fake := hypertest.New(t)
	fake.On(hyper.POST, "/v1/generations").Reply(http.StatusAccepted, generation{ID: "gen_1", Status: "queued"})
	fake.On(hyper.GET, "https://api.test/v1/generations/*").Reply(http.StatusOK, generation{ID: "gen_1", Status: "done"})

	studio := faked(t, fake)

	created, err := studio.Post("/v1/generations", map[string]any{"model": "nano"}).JSON[generation]()
	if err != nil || created.ID != "gen_1" {
		t.Fatalf("create: %+v %v", created, err)
	}
	fetched, err := studio.Get("/v1/generations/gen_1").JSON[generation]()
	if err != nil || fetched.Status != "done" {
		t.Fatalf("fetch: %+v %v", fetched, err)
	}
	if fake.Sent()[0].Header("Content-Type") != "application/json" {
		t.Error("the fake should see the real request headers")
	}
}

func TestASequenceAnswersInOrderAndThenFails(t *testing.T) {
	fake := hypertest.New(&recordingT{TB: t})
	fake.On(hyper.GET, "/v1/generations/*").Sequence(
		hypertest.Reply(http.StatusOK, generation{Status: "running"}),
		hypertest.Reply(http.StatusOK, generation{Status: "succeeded"}),
	)
	studio := faked(t, fake)

	first, _ := studio.Get("/v1/generations/gen_1").JSON[generation]()
	second, _ := studio.Get("/v1/generations/gen_1").JSON[generation]()
	if first.Status != "running" || second.Status != "succeeded" {
		t.Errorf("sequence: %s then %s", first.Status, second.Status)
	}
	if err := studio.Get("/v1/generations/gen_1").Err(); err == nil {
		t.Error("a third call should fail once the sequence is exhausted")
	}
}

func TestAConditionNarrowsAnExpectation(t *testing.T) {
	fake := hypertest.New(t)
	fake.On(hyper.POST, "/v1/generations").
		When(func(sent hypertest.Sent) bool { return sent.JSON("model") == "banned" }).
		Reply(http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": "not enabled"}})
	fake.On(hyper.POST, "/v1/generations").Reply(http.StatusAccepted, generation{ID: "gen_1"})
	studio := faked(t, fake)

	response := studio.Post("/v1/generations", map[string]any{"model": "banned"})
	if response.Status() != http.StatusUnprocessableEntity || response.Field("error.message") != "not enabled" {
		t.Errorf("the conditional expectation did not answer: %d", response.Status())
	}
	if !studio.Post("/v1/generations", map[string]any{"model": "nano"}).Ok() {
		t.Error("the general expectation did not answer")
	}
}

func TestRepliesCanCarryHeadersAndRawBodies(t *testing.T) {
	fake := hypertest.New(t)
	fake.On(hyper.GET, "/text").Sequence(hypertest.Respond(http.StatusOK, "plain", hyper.H{"Content-Type": "text/plain"}))
	fake.On(hyper.GET, "/bytes").Reply(http.StatusOK, []byte{1, 2, 3})
	fake.On(hyper.DELETE, "/thing").Reply(http.StatusNoContent, nil)
	studio := faked(t, fake)

	response := studio.Get("/text")
	if text, _ := response.Text(); text != "plain" || response.Header("Content-Type") != "text/plain" {
		t.Errorf("text reply: %q %q", text, response.Header("Content-Type"))
	}
	if data, _ := studio.Get("/bytes").Bytes(); len(data) != 3 {
		t.Errorf("bytes reply: %v", data)
	}
	if err := studio.Delete("/thing").Err(); err != nil {
		t.Errorf("nil reply: %v", err)
	}
}

func TestAssertionsReadWhatWasSent(t *testing.T) {
	fake := hypertest.New(t)
	fake.On(hyper.POST, "/v1/generations").Reply(http.StatusAccepted, generation{ID: "gen_1"})
	studio := faked(t, fake)

	studio.Post("/v1/generations", map[string]any{"model": "nano"}, hyper.Header("Idempotency-Key", "k"), hyper.Query{"dry": true})

	fake.AssertSent(t, hyper.POST, "/v1/generations", func(sent hypertest.Sent) bool {
		return sent.Header("Idempotency-Key") == "k" && sent.JSON("model") == "nano" && sent.Query("dry") == "true"
	})
	fake.AssertNotSent(t, hyper.GET, "/v1/generations/*")
	fake.AssertCount(t, 1)

	recorder := &recordingT{TB: t}
	fake.AssertSent(recorder, hyper.DELETE, "/v1/generations")
	fake.AssertNotSent(recorder, hyper.POST, "/v1/generations")
	fake.AssertCount(recorder, 2)
	if recorder.failures != 3 {
		t.Errorf("each false assertion should fail, got %d failures", recorder.failures)
	}
}

func TestAnUnexpectedRequestFailsTheTestAndTheCall(t *testing.T) {
	recorder := &recordingT{TB: t}
	fake := hypertest.New(recorder)

	err := faked(t, fake).Get("/nothing").Err()
	if err == nil || recorder.failures == 0 {
		t.Error("an unexpected request should fail the test and the call")
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
