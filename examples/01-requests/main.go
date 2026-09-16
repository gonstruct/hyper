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

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type GenerationRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type Generation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func main() {
	studio := hyper.New(context.Background(), hyper.Base("https://api.tjp.com"), hyper.BearerToken("sk_..."))

	// Folded: the response's error comes back from JSON when there is one.
	// "data" is the key to decode; without it the root is decoded.
	models, err := studio.Get("/v1/models", hyper.Query{"enabled": true}).JSON[[]Model]("data")
	if err != nil {
		panic(err)
	}
	fmt.Println(len(models))

	// Step by step, when the status matters before the body does.
	response := studio.Post("/v1/generations", GenerationRequest{Model: "nano-banana", Prompt: "a still life"})
	if !response.Ok() {
		panic(response.Error())
	}
	generation, err := response.JSON[Generation]()
	if err != nil {
		panic(err)
	}

	generation, err = studio.Get("/v1/generations/" + generation.ID).JSON[Generation]()
	generation, err = studio.Patch("/v1/generations/"+generation.ID, map[string]any{"status": "canceled"}).JSON[Generation]()
	fmt.Println(generation.Status, err)

	// Nothing to read: Err is the response's error and nothing else.
	if err := studio.Delete("/v1/uploads/upl_123").Err(); err != nil {
		panic(err)
	}

	// A status read as data, no opt-out needed.
	if studio.Get("/v1/uploads/upl_123").Status() == 404 {
		fmt.Println("gone")
	}

	// Headers and anything rarer trail as options.
	generation, err = studio.Post("/v1/generations",
		GenerationRequest{Model: "nano-banana", Prompt: "a still life"},
		hyper.Header("Idempotency-Key", "run-42-step-1"),
	).JSON[Generation]()
	fmt.Println(generation.ID, err)
}
