package tests_test

import (
	"context"
	"testing"
	"time"

	"github.com/gonstruct/hyper"
)

func TestAClientIsAValue(t *testing.T) {
	base := client(t, hyper.Header("X-Base", "1"))
	derived := base.With(hyper.Header("X-Derived", "2"))

	if got := seen(t, base.Get("/x")).Headers; got["X-Derived"] != "" {
		t.Errorf("With mutated the receiver: %+v", got)
	}
	if got := seen(t, derived.Get("/x")).Headers; got["X-Base"] != "1" || got["X-Derived"] != "2" {
		t.Errorf("the derived client lost headers: %+v", got)
	}

	// A call's options do not leak into the next call either.
	base.Get("/x", hyper.Header("X-Once", "1"))
	if got := seen(t, base.Get("/x")).Headers; got["X-Once"] != "" {
		t.Errorf("a call's option leaked into the client: %+v", got)
	}
}

func TestWithContextRebindsTheClient(t *testing.T) {
	c := client(t)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	if err := c.WithContext(cancelled).Get("/x").Err(); err == nil {
		t.Error("a cancelled context should fail the call")
	}
	if err := c.Get("/x").Err(); err != nil {
		t.Errorf("the original client should be untouched: %v", err)
	}
}

func TestTheCallOverridesTheClientsTimeout(t *testing.T) {
	c := client(t, hyper.Timeout(50*time.Millisecond))

	if err := c.Get("/slow").Err(); err == nil {
		t.Error("the client's timeout should fire")
	}
	if err := c.Get("/slow", hyper.Timeout(2*time.Second)).Err(); err != nil {
		t.Errorf("the call's timeout should win: %v", err)
	}
}

func TestACustomTransportIsUsed(t *testing.T) {
	transport := &countingTransport{}
	c := client(t, hyper.Transport(transport))

	c.Get("/x")
	c.Get("/x")
	if transport.calls != 2 {
		t.Errorf("expected the custom transport to carry both calls, carried %d", transport.calls)
	}
}
