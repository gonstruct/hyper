// A response carries one error, and what is behind it is a type:
//
//	errors.Is(err, hyper.ErrTransport)   nothing came back: DNS, refused, timeout, cancelled
//	errors.As(err, *hyper.StatusError)   a 4xx or 5xx; the response is attached
//	errors.As(err, *hyper.DecodeError)   only from JSON: a 2xx whose body was not the type
//
// Ok() is true for a 2xx that arrived. Error() is that error or nil. JSON and
// Err return it, so a folded call is still one check.
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/gonstruct/hyper"
)

type Generation struct {
	ID string `json:"id"`
}

func main() {
	studio := hyper.New(context.Background(), hyper.Base("https://api.tjp.com"), hyper.BearerToken("sk_..."))

	response := studio.Post("/v1/generations", map[string]any{"model": "nano-banana", "prompt": "a still life"})

	var status *hyper.StatusError
	switch err := response.Error(); {
	case errors.As(err, &status):
		// The provider answered and said no. Field reads one key of the
		// body without decoding all of it.
		fmt.Println(status.Code, response.Field("error.message"))
	case errors.Is(err, hyper.ErrTransport):
		fmt.Println("never reached the provider:", err)
	default:
		generation, err := response.JSON[Generation]()
		fmt.Println(generation.ID, err)
	}

	// The same, folded, for the caller that only wants to pass it on.
	generation, err := studio.Post("/v1/generations", map[string]any{"model": "nano-banana"}).JSON[Generation]()
	if err != nil {
		fmt.Println(describe(err))
	}
	fmt.Println(generation.ID)
}

// The app translates once, at its edge, into its own words. The status error
// carries the response, so the provider's message is still there.
func describe(err error) string {
	var status *hyper.StatusError
	if errors.As(err, &status) {
		return fmt.Sprintf("tjp said %d: %s", status.Code, status.Response.Field("error.message"))
	}
	return err.Error()
}
