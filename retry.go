package hyper

import (
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// BackoffFunc gives the delay before retry number attempt, counting from 1.
type BackoffFunc func(attempt int) time.Duration

// RetryFunc decides whether a result deserves another attempt.
type RetryFunc func(response *Response) bool

type retryPolicy struct {
	attempts int
	backoff  BackoffFunc
	when     RetryFunc
}

// Retry sends the request again up to attempts more times when the result
// is transient: no response, 429, or 5xx. Retry-After is honoured when the
// provider sends it; otherwise the backoff decides. Retry(0) turns it off.
func Retry(attempts int, backoff BackoffFunc) Option {
	return optionFunc(func(s *settings) {
		s.retry.attempts = attempts
		s.retry.backoff = backoff
		if s.retry.when == nil {
			s.retry.when = Transient
		}
	})
}

// RetryWhen replaces the rule for what deserves a retry.
func RetryWhen(when RetryFunc) Option {
	return optionFunc(func(s *settings) { s.retry.when = when })
}

// Backoff grows exponentially from min, is capped at max, and is jittered so
// clients that failed together do not retry together.
func Backoff(minimum, maximum time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		delay := float64(minimum) * math.Pow(2, float64(attempt-1))
		if delay > float64(maximum) {
			delay = float64(maximum)
		}

		return time.Duration(delay/2 + rand.Float64()*delay/2)
	}
}

// Transient is the default retry rule.
func Transient(response *Response) bool {
	if errors.Is(response.err, ErrTransport) {
		return true
	}

	return response.status == http.StatusTooManyRequests || response.status >= 500
}

func (policy retryPolicy) should(response *Response) bool {
	if policy.attempts <= 0 || policy.when == nil {
		return false
	}

	return policy.when(response)
}

func (policy retryPolicy) delay(attempt int, response *Response) time.Duration {
	if after, ok := retryAfter(response.header); ok {
		return after
	}
	if policy.backoff == nil {
		return 0
	}

	return policy.backoff(attempt)
}

// retryAfter reads the provider's own wish, in seconds or as a date.
func retryAfter(header http.Header) (time.Duration, bool) {
	if header == nil {
		return 0, false
	}
	value := header.Get("Retry-After")
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return time.Until(at), true
	}

	return 0, false
}
