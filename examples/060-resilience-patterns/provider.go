package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// outcome is what the fake provider does with one request. The schedule is
// fixed, so every run of the demo (and every test) sees the same failures.
type outcome int

const (
	succeed outcome = iota // work for latency, charge, respond 200
	fail                   // respond 503 at once, no charge
	hang                   // work for hangFor, then charge and respond, unless cancelled first
	lose                   // charge, then respond 502: the work happened but the answer was lost
)

// FlakyProvider is a fake "AI provider" for httptest. It charges for every
// completed generation, and deduplicates by Idempotency-Key: a key it has
// already charged for gets the stored response back, free (slide p06).
//
// Requests past the end of the schedule succeed.
type FlakyProvider struct {
	latency  time.Duration
	hangFor  time.Duration
	schedule []outcome

	mu   sync.Mutex
	next int               // index into schedule
	done map[string]string // Idempotency-Key -> response already paid for

	requests  atomic.Int64 // every request received
	charges   atomic.Int64 // generations actually billed
	inFlight  atomic.Int64 // requests currently "generating"
	cancelled atomic.Int64 // generations stopped because the caller went away
}

func NewFlakyProvider(latency, hangFor time.Duration, schedule ...outcome) *FlakyProvider {
	return &FlakyProvider{
		latency:  latency,
		hangFor:  hangFor,
		schedule: schedule,
		done:     make(map[string]string),
	}
}

func (p *FlakyProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.requests.Add(1)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	key := r.Header.Get("Idempotency-Key")

	p.mu.Lock()
	if resp, ok := p.done[key]; ok && key != "" {
		p.mu.Unlock()
		w.Header().Set("Idempotent-Replayed", "true")
		fmt.Fprint(w, resp)
		return
	}
	o := succeed
	if p.next < len(p.schedule) {
		o = p.schedule[p.next]
	}
	p.next++
	p.mu.Unlock()

	resp := "summary of " + string(body)
	switch o {
	case fail:
		http.Error(w, "overloaded", http.StatusServiceUnavailable)
	case lose:
		p.charge(key, resp)
		http.Error(w, "upstream connection reset", http.StatusBadGateway)
	case hang, succeed:
		d := p.latency
		if o == hang {
			d = p.hangFor
		}
		// r.Context() is cancelled when the client disconnects, which is
		// how a caller's context cancellation reaches the server.
		if !p.work(r.Context(), d) {
			return
		}
		p.charge(key, resp)
		fmt.Fprint(w, resp)
	}
}

// work simulates generation. It reports false if ctx was cancelled first.
func (p *FlakyProvider) work(ctx context.Context, d time.Duration) bool {
	p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		p.cancelled.Add(1)
		return false
	}
}

func (p *FlakyProvider) charge(key, resp string) {
	p.charges.Add(1)
	if key != "" {
		p.mu.Lock()
		p.done[key] = resp
		p.mu.Unlock()
	}
}

// waitIdle polls until no request is generating or limit passes. The demo
// uses it to observe what the provider does after the client has given up.
func (p *FlakyProvider) waitIdle(limit time.Duration) bool {
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if p.inFlight.Load() == 0 {
			return true
		}
	}
	return p.inFlight.Load() == 0
}
