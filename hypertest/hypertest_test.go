package hypertest_test

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

func TestFakeAnswersAndRecords(t *testing.T) {
	fake := hypertest.New(t)
	fake.On(hyper.POST, "/v1/generations").Reply(http.StatusAccepted, generation{ID: "gen_1", Status: "queued"})
	fake.On(hyper.GET, "/v1/generations/*").Sequence(
		hypertest.Reply(http.StatusOK, generation{ID: "gen_1", Status: "running"}),
		hypertest.Reply(http.StatusOK, generation{ID: "gen_1", Status: "succeeded"}),
	)
	fake.On(hyper.POST, "/v1/generations").
		When(func(sent hypertest.Sent) bool { return sent.JSON("model") == "banned" }).
		Reply(http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": "not enabled"}})

	studio := hyper.New(context.Background(), hyper.Base("https://api.test"), hyper.Transport(fake), hyper.Untraced())

	created, err := studio.Post("/v1/generations", map[string]any{"model": "nano"}, hyper.Header("Idempotency-Key", "k")).JSON[generation]()
	if err != nil || created.ID != "gen_1" {
		t.Fatalf("create: %+v %v", created, err)
	}

	first, _ := studio.Get("/v1/generations/gen_1").JSON[generation]()
	second, _ := studio.Get("/v1/generations/gen_1").JSON[generation]()
	if first.Status != "running" || second.Status != "succeeded" {
		t.Fatalf("sequence: %s then %s", first.Status, second.Status)
	}

	// The first matching expectation wins, so the conditional one is listed
	// after the general one only if its condition is what distinguishes it.
	fake.AssertSent(t, hyper.POST, "/v1/generations", func(sent hypertest.Sent) bool {
		return sent.Header("Idempotency-Key") == "k" && sent.JSON("model") == "nano"
	})
	fake.AssertNotSent(t, hyper.DELETE, "/v1/generations/*")
	fake.AssertCount(t, 3)
}

func TestConditionalExpectationBeforeGeneral(t *testing.T) {
	fake := hypertest.New(t)
	fake.On(hyper.POST, "/v1/generations").
		When(func(sent hypertest.Sent) bool { return sent.JSON("model") == "banned" }).
		Reply(http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": "not enabled"}})
	fake.On(hyper.POST, "/v1/generations").Reply(http.StatusAccepted, generation{ID: "gen_1"})

	studio := hyper.New(context.Background(), hyper.Base("https://api.test"), hyper.Transport(fake), hyper.Untraced())

	response := studio.Post("/v1/generations", map[string]any{"model": "banned"})
	if response.Status() != http.StatusUnprocessableEntity || response.Field("error.message") != "not enabled" {
		t.Fatalf("conditional expectation did not answer: %d", response.Status())
	}
	if ok := studio.Post("/v1/generations", map[string]any{"model": "nano"}).Ok(); !ok {
		t.Fatal("general expectation did not answer")
	}
}

func TestUnexpectedRequestFailsTheTest(t *testing.T) {
	recorder := &recordingT{TB: t}
	fake := hypertest.New(recorder)

	err := hyper.New(context.Background(), hyper.Transport(fake), hyper.Untraced()).Get("https://api.test/nothing").Err()
	if err == nil || !recorder.failed {
		t.Fatal("an unexpected request should fail the test and the call")
	}
}

type recordingT struct {
	testing.TB
	failed bool
}

func (r *recordingT) Errorf(string, ...any) { r.failed = true }
