// Command http-clients demonstrates a resilient HTTP client: Client.Timeout
// versus a per-request context deadline, a retry loop with full-jitter
// exponential backoff that honours Retry-After, careful body draining, and a
// hand-rolled circuit breaker that fails fast against a dead endpoint.
//
// Slides: slides/045-http-clients.pdf
//
// Run:
//
//	go run ./examples/045-http-clients
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

func main() {
	srv := newTestServer()
	defer srv.Close()
	dead := deadURL()

	section("1. Client.Timeout fires (slides p02-p03)")
	clientTimeout(srv.URL)

	section("2. A per-request context deadline fires sooner (slide p04)")
	contextTimeout(srv.URL)

	section("3. Retry with backoff, jitter and Retry-After (slides p05-p07)")
	retryFlaky(srv.URL)
	retryDead(dead)

	section("4. A circuit breaker fails fast (slide p08)")
	breakerDemo(dead, srv.URL)
}

// newTestServer simulates the downstream service in-process.
func newTestServer() *httptest.Server {
	var flakyCalls atomic.Int32
	mux := http.NewServeMux()

	// /slow takes 300 ms, but gives up as soon as the client goes away, so
	// the server never does work nobody is waiting for.
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(300 * time.Millisecond):
			fmt.Fprint(w, "finally")
		case <-r.Context().Done():
		}
	})

	// /flaky fails twice with 503 and Retry-After: 0, then succeeds.
	mux.HandleFunc("GET /flaky", func(w http.ResponseWriter, r *http.Request) {
		if n := flakyCalls.Add(1); n <= 2 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	return httptest.NewServer(mux)
}

// deadURL returns the address of a server that has already shut down, so
// connecting to it is refused.
func deadURL() string {
	s := httptest.NewServer(http.NotFoundHandler())
	s.Close()
	return s.URL
}

// isDialError reports whether err came from failing to connect. Printing the
// error text itself would differ per OS ("connection refused" vs
// "actively refused it").
func isDialError(err error) bool {
	opErr, ok := errors.AsType[*net.OpError](err)
	return ok && opErr.Op == "dial"
}

func clientTimeout(base string) {
	client := &http.Client{Timeout: 100 * time.Millisecond}

	start := time.Now()
	resp, err := client.Get(base + "/slow")
	elapsed := time.Since(start)
	if err == nil {
		resp.Body.Close()
		fmt.Println("unexpected success")
		return
	}
	netErr, ok := errors.AsType[net.Error](err)
	fmt.Printf("error type: %T, Timeout() = %t\n", err, ok && netErr.Timeout())
	fmt.Printf("errors.Is(err, context.DeadlineExceeded) = %t\n", errors.Is(err, context.DeadlineExceeded))
	fmt.Printf("gave up after ~%v (the handler needs 300ms)\n", elapsed.Round(10*time.Millisecond))
}

func contextTimeout(base string) {
	client := &http.Client{Timeout: 100 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := fetch(ctx, client, base+"/slow")
	elapsed := time.Since(start)

	fmt.Printf("errors.Is(err, context.DeadlineExceeded) = %t\n", errors.Is(err, context.DeadlineExceeded))
	fmt.Printf("gave up after ~%v: the effective deadline is min(ctx 50ms, Client.Timeout 100ms)\n",
		elapsed.Round(10*time.Millisecond))
}

func retryFlaky(base string) {
	client := &http.Client{Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempt := 0
	var body string
	err := Retry(ctx, 5, func(ctx context.Context) error {
		attempt++
		var err error
		body, err = fetch(ctx, client, base+"/flaky")
		if err != nil {
			fmt.Printf("/flaky attempt %d: %v\n", attempt, err)
			return err
		}
		fmt.Printf("/flaky attempt %d: %q\n", attempt, body)
		return nil
	})
	fmt.Printf("/flaky result: %q, err = %v\n", body, err)
}

func retryDead(dead string) {
	client := &http.Client{Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempts := 0
	err := Retry(ctx, 3, func(ctx context.Context) error {
		attempts++
		_, err := fetch(ctx, client, dead)
		return err
	})
	// Retry wraps the last error with %w, so errors.As still finds the
	// *net.OpError underneath.
	fmt.Printf("dead endpoint: %d attempts, err != nil: %t, last error is a dial error: %t\n",
		attempts, err != nil, isDialError(err))
}

func breakerDemo(dead, base string) {
	client := &http.Client{Timeout: 2 * time.Second}
	br := NewBreaker(3, 80*time.Millisecond)
	sent := 0
	target := dead

	call := func(i int) {
		err := br.Do(func() error {
			sent++
			_, err := fetch(context.Background(), client, target)
			return err
		})
		var result string
		switch {
		case err == nil:
			result = "ok"
		case errors.Is(err, ErrOpen):
			result = err.Error()
		case isDialError(err):
			result = "dial error"
		default:
			result = "error"
		}
		fmt.Printf("call %d: %-25s state=%-9v requests sent=%d\n", i, result, br.State(), sent)
	}

	for i := 1; i <= 5; i++ {
		call(i)
	}

	time.Sleep(100 * time.Millisecond) // longer than the 80ms open timeout
	fmt.Printf("after the open timeout: state=%v\n", br.State())
	target = base + "/ok" // the service has recovered
	call(6)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
