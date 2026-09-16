// One line per request. Method, URL, input. The input's type says how it
// travels:
//
//	a struct or a map     JSON body
//	hyper.Query           query string
//	hyper.Form            application/x-www-form-urlencoded
//	hyper.Bytes           raw body, content type from the option
//	nil                   nothing
//
// The verb returns a Response. JSON[T] decodes it; Ok, Status, Error,
// Bytes, Text and Field read it other ways.
package main

import (
	"context"
	"fmt"

	"github.com/gonstruct/hyper"
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NewTask struct {
	Title string `json:"title"`
	Due   string `json:"due"`
}

type Task struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func main() {
	api := hyper.New(context.Background(), hyper.Base("https://api.example.com"), hyper.BearerToken("token"))

	// Folded: the response's error comes back from JSON when there is one.
	// "data" is the key to decode; without it the root is decoded.
	projects, err := api.Get("/projects", hyper.Query{"archived": false}).JSON[[]Project]("data")
	if err != nil {
		panic(err)
	}
	fmt.Println(len(projects))

	// Step by step, when the status matters before the body does.
	response := api.Post("/tasks", NewTask{Title: "Write the README", Due: "2026-10-01"})
	if !response.Ok() {
		panic(response.Error())
	}
	task, err := response.JSON[Task]()
	if err != nil {
		panic(err)
	}

	task, err = api.Get("/tasks/" + task.ID).JSON[Task]()
	task, err = api.Patch("/tasks/"+task.ID, map[string]any{"status": "done"}).JSON[Task]()
	fmt.Println(task.Status, err)

	// Nothing to read: Err is the response's error and nothing else.
	if err := api.Delete("/tasks/" + task.ID).Err(); err != nil {
		panic(err)
	}

	// A status read as data, no opt-out needed.
	if api.Get("/tasks/"+task.ID).Status() == 404 {
		fmt.Println("gone")
	}

	// Headers and anything rarer trail as options.
	task, err = api.Post("/tasks", NewTask{Title: "Ship it"}, hyper.Header("Idempotency-Key", "ship-1")).JSON[Task]()
	fmt.Println(task.ID, err)
}
