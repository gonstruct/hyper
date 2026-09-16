// The fake is a transport handed to the client. No globals, tests run in
// parallel, a request nobody expected fails the test with the request
// printed. Assertions read what was sent, decoded.
package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
)

type Task struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func TestCreateTask(t *testing.T) {
	fake := hyper.Fake(t)

	fake.On(hyper.POST, "https://api.example.com/tasks").
		Reply(http.StatusAccepted, Task{ID: "task_1", Status: "queued"})

	// A sequence: first poll running, second done, a third fails the test.
	fake.On(hyper.GET, "https://api.example.com/tasks/task_1").Sequence(
		hyper.Reply(http.StatusOK, Task{ID: "task_1", Status: "running"}),
		hyper.Reply(http.StatusOK, Task{ID: "task_1", Status: "done"}),
	)

	// A refusal, matched on what was sent. Patterns take * for a segment.
	fake.On(hyper.POST, "https://api.example.com/tasks").
		When(func(sent hyper.Sent) bool { return sent.JSON("title") == "" }).
		Reply(http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": "title is required"}})
	fake.On(hyper.PUT, "https://bucket.example.com/*").Reply(http.StatusOK, nil)

	api := hyper.New(context.Background(), hyper.Base("https://api.example.com"), hyper.BearerToken("test"), hyper.Transport(fake))

	task, err := api.Post("/tasks", map[string]any{"title": "Write the README"}, hyper.Header("Idempotency-Key", "k1")).JSON[Task]()
	if err != nil || task.ID != "task_1" {
		t.Fatalf("got %+v, %v", task, err)
	}

	fake.AssertSent(t, hyper.POST, "/tasks", func(sent hyper.Sent) bool {
		return sent.Header("Idempotency-Key") == "k1" && sent.JSON("title") == "Write the README"
	})
	fake.AssertNotSent(t, hyper.GET, "/tasks/*")
	fake.AssertCount(t, 1)
}

func main() {}
