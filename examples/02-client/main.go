// A client is built once per scope with everything that is true for every
// request to an API. It carries the context, so no call repeats it. A client
// is a value: a call never mutates it.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gonstruct/hyper"
)

type Account struct {
	Plan string `json:"plan"`
}

func main() {
	ctx := context.Background()

	api := hyper.New(ctx,
		hyper.Base("https://api.example.com"),
		hyper.BearerToken("token"),
		hyper.Accept("application/json"),
		hyper.Timeout(30*time.Second),                              // per attempt
		hyper.Retry(3, hyper.Backoff(time.Second, 10*time.Second)), // transient only: no response, 429, 5xx; honours Retry-After
		hyper.Transport(http.DefaultTransport),                     // or your own; hyper wraps it with tracing
	)

	// URLs resolve against Base; an absolute URL is used as-is.
	account, err := api.Get("/account").JSON[Account]()
	if err != nil {
		panic(err)
	}
	fmt.Println(account.Plan)

	// A call's options add to the client's: headers merge, a timeout or
	// retry on the call replaces the client's for that call.
	account, err = api.Get("/account", hyper.Timeout(5*time.Second), hyper.Retry(0, nil)).JSON[Account]()

	// A different context for one call, or a derived client for a scope.
	deadline, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	account, err = api.WithContext(deadline).Get("/account").JSON[Account]()
	fmt.Println(account.Plan, err)

	// No client at all, for a one-off. The context is the first argument
	// because there is nowhere else for it.
	repository, err := hyper.Get(ctx, "https://api.github.com/repos/golang/go").JSON[map[string]any]()
	if err != nil {
		panic(err)
	}
	fmt.Println(repository["name"])
}
