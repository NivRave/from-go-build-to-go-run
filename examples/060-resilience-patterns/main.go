// Command resilience-patterns composes context timeouts, retries with
// backoff and jitter, idempotency keys and a circuit breaker into one policy
// around a deliberately flaky (fake) AI provider.
//
// Slides: slides/060-resilience-patterns.pdf
//
// Run:
//
//	go run ./examples/060-resilience-patterns
//	go test ./examples/060-resilience-patterns
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

// Metrics, named the Prometheus way: counters end in _total, and the
// breaker's current state is a gauge (slide p08).
var (
	retriesTotal             atomic.Int64
	breakerStateChangesTotal atomic.Int64
	breakerState             atomic.Int64 // gauge: 0 closed, 1 half-open, 2 open
)

func main() {
	section("1. Ghost load: select + time.After (slides p02, p03)")
	ghostLoad()

	section("2. The fix: the deadline travels in the context")
	contextPropagation()

	section("3. Retries with backoff, jitter and an idempotency key (slides p04-p06)")
	retryIdempotent()

	section("4. Circuit breaker (slide p07)")
	circuitBreaker()

	section("5. Metrics (slide p08)")
	printMetrics()
}

// startProvider runs p behind a real HTTP server on 127.0.0.1.
func startProvider(p *FlakyProvider) (*httptest.Server, *AIClient) {
	srv := httptest.NewServer(p)
	return srv, &AIClient{BaseURL: srv.URL, HTTP: srv.Client(), Timeout: 200 * time.Millisecond}
}

func ghostLoad() {
	p := NewFlakyProvider(0, time.Second, hang)
	srv, client := startProvider(p)
	defer srv.Close()

	_, err := client.CallFlakyAI(context.Background(), 200*time.Millisecond)
	fmt.Println("CallFlakyAI returned:", err)
	fmt.Println("provider requests in flight right after:", p.inFlight.Load())

	p.waitIdle(3 * time.Second)
	fmt.Printf("...and later: in flight %d, cancelled %d, charged %d\n",
		p.inFlight.Load(), p.cancelled.Load(), p.charges.Load())
	fmt.Println("the provider did the work and billed for it after the caller had given up")
}

func contextPropagation() {
	p := NewFlakyProvider(0, time.Second, hang)
	srv, client := startProvider(p)
	defer srv.Close()

	_, err := client.CallAI(context.Background(), "quarterly report")
	fmt.Println("CallAI returned:", err)
	fmt.Println("errors.Is(err, ErrAITimeout):", errors.Is(err, ErrAITimeout))

	// The server notices the closed connection asynchronously, so give it a
	// moment before reading the counters.
	p.waitIdle(3 * time.Second)
	fmt.Printf("provider: in flight %d, cancelled %d, charged %d\n",
		p.inFlight.Load(), p.cancelled.Load(), p.charges.Load())
}

func retryIdempotent() {
	policy := DefaultBackoff
	policy.OnRetry = func(attempt int, err error, _ time.Duration) {
		retriesTotal.Add(1)
		fmt.Printf("  attempt %d failed: %v; retrying\n", attempt, err)
	}

	// fail = 503, lose = the provider charged but the response was lost (502).
	for _, withKey := range []bool{false, true} {
		p := NewFlakyProvider(10*time.Millisecond, 0, fail, lose, succeed)
		srv, client := startProvider(p)

		// One key per logical operation, reused by every attempt.
		key := ""
		if withKey {
			key = rand.Text()
		}
		fmt.Printf("with Idempotency-Key: %v\n", withKey)
		var summary string
		err := policy.Retry(context.Background(), 4, func(ctx context.Context) error {
			var err error
			summary, err = client.CallAIIdempotent(ctx, key, "quarterly report")
			return err
		})
		fmt.Printf("  result: %q, err: %v\n", summary, err)
		fmt.Printf("  provider requests: %d, charges: %d\n", p.requests.Load(), p.charges.Load())
		srv.Close()
	}
}

func circuitBreaker() {
	// Three failures, then the provider recovers.
	p := NewFlakyProvider(0, 0, fail, fail, fail)
	srv, client := startProvider(p)
	defer srv.Close()

	cb := NewCircuitBreaker[string](Settings{
		Name:        "AI-Service",
		MaxRequests: 1,
		Timeout:     300 * time.Millisecond,
		ReadyToTrip: func(c Counts) bool { return c.ConsecutiveFailures >= 3 },
		OnStateChange: func(name string, from, to State) {
			breakerStateChangesTotal.Add(1)
			breakerState.Store(int64(to))
			fmt.Printf("  [%s] %s -> %s\n", name, from, to)
		},
		// The caller cancelling is not the provider's fault.
		IsSuccessful: func(err error) bool { return err == nil || errors.Is(err, context.Canceled) },
	})

	// SafeCall from slide p07, without the type assertion that panics when
	// Execute returns (nil, ErrOpenState).
	safeCall := func(ctx context.Context) (string, error) {
		return cb.Execute(func() (string, error) { return client.CallAI(ctx, "quarterly report") })
	}

	for i := 1; i <= 5; i++ {
		_, err := safeCall(context.Background())
		fmt.Printf("call %d: err = %v\n", i, err)
	}
	fmt.Printf("provider saw %d of 5 calls; the rest failed fast\n", p.requests.Load())

	time.Sleep(350 * time.Millisecond) // longer than Settings.Timeout
	res, err := safeCall(context.Background())
	fmt.Printf("after the open timeout: %q, err = %v\n", res, err)
}

func printMetrics() {
	fmt.Println("retries_total", retriesTotal.Load())
	fmt.Println("breaker_state_changes_total", breakerStateChangesTotal.Load())
	fmt.Println("breaker_state", breakerState.Load())
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}

// Compile-time check that the provider is an http.Handler.
var _ http.Handler = (*FlakyProvider)(nil)
