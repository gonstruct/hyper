package tests_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// countingTransport records what passes through it.
type countingTransport struct {
	calls   int
	headers []http.Header
}

func (transport *countingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls++
	transport.headers = append(transport.headers, request.Header.Clone())

	return http.DefaultTransport.RoundTrip(request)
}

// withTracing registers an in-memory tracer provider for the duration of the
// test, the way an application would at boot.
func withTracing(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		otel.SetTextMapPropagator(previousPropagator)
		_ = provider.Shutdown(context.Background())
	})

	return exporter
}

func TestRequestsAreTracedWhenTheAppTraces(t *testing.T) {
	exporter := withTracing(t)
	transport := &countingTransport{}
	s := server(t)

	// Not Untraced: this is the default path.
	c := hyper.New(context.Background(), hyper.Base(s.URL), hyper.Transport(transport))
	if err := c.Get("/traced").Err(); err != nil {
		t.Fatal(err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected one client span, got %d", len(spans))
	}
	if transport.headers[0].Get("Traceparent") == "" {
		t.Error("the trace context should be propagated to the provider")
	}
}

func TestRetriesAreOneSpanPerAttempt(t *testing.T) {
	exporter := withTracing(t)
	s := server(t)

	c := hyper.New(context.Background(), hyper.Base(s.URL), hyper.Retry(3, fast()))
	if err := c.Get("/flaky").Err(); err != nil {
		t.Fatal(err)
	}

	if spans := exporter.GetSpans(); len(spans) != 3 {
		t.Errorf("expected a span per attempt, got %d", len(spans))
	}
}

func TestUntracedSendsNothing(t *testing.T) {
	exporter := withTracing(t)
	transport := &countingTransport{}
	s := server(t)

	c := hyper.New(context.Background(), hyper.Base(s.URL), hyper.Transport(transport), hyper.Untraced())
	if err := c.Get("/quiet").Err(); err != nil {
		t.Fatal(err)
	}

	if spans := exporter.GetSpans(); len(spans) != 0 {
		t.Errorf("expected no spans, got %d", len(spans))
	}
	if transport.headers[0].Get("Traceparent") != "" {
		t.Error("an untraced request should carry no trace context")
	}
}

func TestNothingHappensWithoutATracerProvider(t *testing.T) {
	transport := &countingTransport{}
	s := server(t)

	c := hyper.New(context.Background(), hyper.Base(s.URL), hyper.Transport(transport))
	if err := c.Get("/x").Err(); err != nil {
		t.Fatal(err)
	}
	if transport.headers[0].Get("Traceparent") != "" {
		t.Error("with the no-op provider nothing should be injected")
	}
}
