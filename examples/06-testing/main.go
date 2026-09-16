// The fake is a transport handed to the client. No globals, tests run in
// parallel, a request nobody expected fails the test with the request
// printed. Assertions read what was sent, decoded.
package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
	"github.com/gonstruct/hyper/hypertest"
)

type Generation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func TestGenerate(t *testing.T) {
	fake := hypertest.New(t)

	fake.On(hyper.POST, "https://api.tjp.com/v1/generations").
		Reply(http.StatusAccepted, Generation{ID: "gen_1", Status: "queued"})

	// A sequence: first poll running, second done, a third fails the test.
	fake.On(hyper.GET, "https://api.tjp.com/v1/generations/gen_1").Sequence(
		hypertest.Reply(http.StatusOK, Generation{ID: "gen_1", Status: "running"}),
		hypertest.Reply(http.StatusOK, Generation{ID: "gen_1", Status: "succeeded"}),
	)

	// A refusal, matched on what was sent. Patterns take * for a segment.
	fake.On(hyper.POST, "https://api.tjp.com/v1/generations").
		When(func(sent hypertest.Sent) bool { return sent.JSON("model") == "banned" }).
		Reply(http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": "model not enabled"}})
	fake.On(hyper.PUT, "https://bucket.example.com/*").Reply(http.StatusOK, nil)

	studio := hyper.New(context.Background(), hyper.Base("https://api.tjp.com"), hyper.BearerToken("sk_test"), hyper.Transport(fake))

	generation, err := studio.Post("/v1/generations",
		map[string]any{"model": "nano-banana", "prompt": "a still life"},
		hyper.Header("Idempotency-Key", "step-1"),
	).JSON[Generation]()
	if err != nil || generation.ID != "gen_1" {
		t.Fatalf("got %+v, %v", generation, err)
	}

	fake.AssertSent(t, hyper.POST, "/v1/generations", func(sent hypertest.Sent) bool {
		return sent.Header("Idempotency-Key") == "step-1" && sent.JSON("model") == "nano-banana"
	})
	fake.AssertNotSent(t, hyper.GET, "/v1/generations/*")
	fake.AssertCount(t, 1)
}

func main() {}
