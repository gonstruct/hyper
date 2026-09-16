package example

import (
	"fmt"

	"github.com/gonstruct/hyper"
)

type Test struct {
	Name string `json:"name"`
}

func BasicExample() {
	response := hyper.Get("https://example.com")
	if !response.Ok() {
		panic(fmt.Errorf("request failed: %w", response.Error()))
	}

	test, err := hyper.Json[Test](response)
	if err != nil {
		panic(fmt.Errorf("failed to parse response: %w", err))
	}

	fmt.Printf("Response: %+v\n", test)
}
