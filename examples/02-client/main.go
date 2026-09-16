// A client is built once per scope with everything that is true for every
// request to a provider. It carries the context, so no call repeats it. A
// client is a value: a call never mutates it.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gonstruct/hyper"
)

type Credits struct {
	Balance int `json:"balance"`
}

func main() {
	ctx := context.Background()

	studio := hyper.New(ctx,
		hyper.Base("https://api.tjp.com"),
		hyper.BearerToken("sk_..."),
		hyper.Accept("application/json"),
		hyper.Timeout(30*time.Second),                              // per attempt
		hyper.Retry(3, hyper.Backoff(time.Second, 10*time.Second)), // transient only: no response, 429, 5xx; honours Retry-After
		hyper.Transport(http.DefaultTransport),                     // or an OpenTelemetry transport; hyper does not know or care
	)

	// URLs resolve against Base; an absolute URL is used as-is.
	credits, err := studio.Get("/v1/credits").JSON[Credits]()
	if err != nil {
		panic(err)
	}
	fmt.Println(credits.Balance)

	// A call's options add to the client's: headers merge, a timeout or
	// retry on the call replaces the client's for that call.
	credits, err = studio.Get("/v1/credits", hyper.Timeout(5*time.Second), hyper.Retry(0, nil)).JSON[Credits]()

	// A different context for one call, or a derived client for a scope.
	deadline, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	credits, err = studio.WithContext(deadline).Get("/v1/credits").JSON[Credits]()
	fmt.Println(credits.Balance, err)

	// No client at all, for a one-off. The context is the first argument
	// because there is nowhere else for it.
	repository, err := hyper.Get(ctx, "https://api.github.com/repos/golang/go").JSON[map[string]any]()
	if err != nil {
		panic(err)
	}
	fmt.Println(repository["name"])
}
