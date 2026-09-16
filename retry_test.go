package hyper_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gonstruct/hyper"
)

func fast() hyper.BackoffFunc {
	return hyper.Backoff(time.Millisecond, 5*time.Millisecond)
}

func TestTransientResultsAreRetried(t *testing.T) {
	got := seen(t, client(t, hyper.Retry(3, fast())).Get("/flaky"))
	if got.Path != "/flaky" {
		t.Errorf("expected the third attempt to succeed, got %+v", got)
	}
}

func TestWithoutRetryTheFirstAnswerStands(t *testing.T) {
	response := client(t).Get("/flaky")
	if response.Status() != http.StatusServiceUnavailable {
		t.Errorf("expected the 503, got %d", response.Status())
	}
}

func TestRetryOnTheCallReplacesTheClients(t *testing.T) {
	response := client(t, hyper.Retry(3, fast())).Get("/flaky", hyper.Retry(0, nil))
	if response.Status() != http.StatusServiceUnavailable {
		t.Errorf("Retry(0) on the call should turn retries off, got %d", response.Status())
	}
}

func TestRetryAfterIsHonoured(t *testing.T) {
	started := time.Now()
	response := client(t, hyper.Retry(2, fast())).Get("/retry-after")

	if !response.Ok() {
		t.Fatalf("expected the retry to succeed: %v", response.Error())
	}
	if elapsed := time.Since(started); elapsed < 900*time.Millisecond {
		t.Errorf("Retry-After: 1 should have waited about a second, waited %s", elapsed)
	}
}

func TestADefinitiveAnswerIsNotRetried(t *testing.T) {
	var attempts int
	when := func(response *hyper.Response) bool {
		attempts++
		return hyper.Transient(response)
	}
	client(t, hyper.Retry(3, fast()), hyper.RetryWhen(when)).Get("/missing")
	if attempts != 1 {
		t.Errorf("a 404 should be asked about once and not retried, asked %d times", attempts)
	}
}

func TestRetryWhenCanWidenTheRule(t *testing.T) {
	transport := &countingTransport{}
	when := func(response *hyper.Response) bool { return response.Status() == http.StatusConflict }

	client(t, hyper.Transport(transport), hyper.Retry(2, fast()), hyper.RetryWhen(when)).Get("/conflict")
	if transport.calls != 3 {
		t.Errorf("expected the first try plus two retries, got %d attempts", transport.calls)
	}
}

func TestAStreamIsNeverRetried(t *testing.T) {
	var attempts int
	when := func(response *hyper.Response) bool {
		attempts++
		return true
	}
	client(t, hyper.Retry(3, fast()), hyper.RetryWhen(when)).Put("/flaky", nil, hyper.Stream(strings.NewReader("once"), 4))
	if attempts != 0 {
		t.Errorf("a stream body cannot be sent twice; the retry rule should not even be asked, was asked %d times", attempts)
	}
}

func TestTransportErrorsAreRetriedAndTyped(t *testing.T) {
	transport := &failingTransport{}
	err := hyper.New(context.Background(), hyper.Retry(2, fast()), hyper.Transport(transport), hyper.Untraced()).
		Get("http://api.test/nothing").Err()

	if !errors.Is(err, hyper.ErrTransport) {
		t.Errorf("expected ErrTransport, got %v", err)
	}
	if transport.calls != 3 {
		t.Errorf("expected three attempts, got %d", transport.calls)
	}
}

// failingTransport never reaches a server.
type failingTransport struct{ calls int }

func (transport *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls++

	return nil, errors.New("connection refused")
}

func TestTimeoutIsPerAttempt(t *testing.T) {
	err := client(t).Get("/slow", hyper.Timeout(50*time.Millisecond)).Err()
	if !errors.Is(err, hyper.ErrTransport) {
		t.Errorf("a timeout should read as a transport error, got %v", err)
	}

	// With a timeout that fits, the same call succeeds.
	if err := client(t).Get("/slow", hyper.Timeout(2*time.Second)).Err(); err != nil {
		t.Errorf("a fitting timeout should not fire: %v", err)
	}
}

func TestACancelledContextStopsTheBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := server(t)
	c := hyper.New(ctx, hyper.Base(s.URL), hyper.Untraced(), hyper.Retry(5, hyper.Backoff(time.Hour, time.Hour)))

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	err := c.Get("/flaky").Err()
	if !errors.Is(err, hyper.ErrTransport) || time.Since(started) > 5*time.Second {
		t.Errorf("cancelling should end the wait at once with ErrTransport, got %v after %s", err, time.Since(started))
	}
}
