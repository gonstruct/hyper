package example

import (
	"fmt"

	"github.com/gonstruct/hyper"
)

func Api() *hyper.Request {
	return hyper.NewRequest(
		hyper.WithBase("https://example.com"),
		hyper.WithHeaders(
			hyper.WithHeader("Authorization", "Bearer token"),
			hyper.WithHeader("Content-Type", "application/json"),
		),
		hyper.WithHeaders(hyper.H{
			"X-Custom-Header": "value",
		}),
	)
}

func ComplexExample() {
	response := Api().Get("/info",
		hyper.WithQueries(
			hyper.WithQuery("lang", "en"),
			hyper.WithQuery("format", "json"),
		),
		hyper.WithQueries(hyper.Q{
			"version": "1.0",
		}),
	)
	if !response.Ok() {
		panic(fmt.Errorf("request failed: %w", response.Error()))
	}

	response = Api().Post("/submit", hyper.WithJson(hyper.J{
		"data": "example",
	}))
	if !response.Ok() {
		panic(fmt.Errorf("request failed: %w", response.Error()))
	}

	response = Api().Put("/update", hyper.WithJson(
		hyper.WithJsonField("field1", "value1"),
		hyper.WithJsonField("field2", 42),
	))
	if !response.Ok() {
		panic(fmt.Errorf("request failed: %w", response.Error()))
	}

}
