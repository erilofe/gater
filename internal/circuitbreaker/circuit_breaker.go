package circuitbreaker

import (
	"log"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

// Settings defines the configurable parameters for the Circuit Breaker.
type Settings struct {
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
}

// NewCircuitBreaker creates a configured CircuitBreaker.
func NewCircuitBreaker(name string, settings Settings) *gobreaker.CircuitBreaker {
	cbSettings := gobreaker.Settings{
		Name:        name,
		MaxRequests: settings.MaxRequests, // How many requests are allowed in half-open state
		Interval:    settings.Interval,    // Time to reset the counts
		Timeout:     settings.Timeout,     // Time to wait before transitioning from open to half-open

		// Fail the circuit breaker if there are more than 3 consecutive failures or if the failure ratio exceeds 60%
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests) // Calculate failure ratio
			return counts.ConsecutiveFailures > 3 || failureRatio > .6
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker '%s' changed state from %v to %v", name, from, to)
		},
	}

	return gobreaker.NewCircuitBreaker(cbSettings)
}

// CircuitBreakerTransport is an HTTP RoundTripper that wraps another RoundTripper
// with a circuit breaker.
type CircuitBreakerTransport struct {
	cb *gobreaker.CircuitBreaker
	rt http.RoundTripper
}

// NewCircuitBreakerTransport creates a new CircuitBreakerTransport.
func NewCircuitBreakerTransport(cb *gobreaker.CircuitBreaker, rt http.RoundTripper) *CircuitBreakerTransport {
	return &CircuitBreakerTransport{
		cb: cb,
		rt: rt,
	}
}

// RoundTrip executes a single HTTP transaction, using the circuit breaker to
// manage failures.
func (t *CircuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.cb.Execute(func() (any, error) {
		resp, err := t.rt.RoundTrip(req) // Perform the actual HTTP request

		// Consider HTTP 5xx responses as errors for the circuit breaker
		if err == nil && resp.StatusCode >= http.StatusInternalServerError {
			return nil, http.ErrHandlerTimeout
		}

		return resp, err
	})

	if err != nil {
		return nil, err
	}

	// Type assertion to convert any back to *http.Response
	return resp.(*http.Response), nil
}
