package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

var errFlaky = errors.New("flaky")

// noJitter makes the schedule exact, so the test can assert it.
func noJitter(d time.Duration) time.Duration { return d }

// Inside synctest.Test the clock is fake: time.Sleep and timers return as
// soon as every goroutine in the bubble is blocked, so these tests take
// microseconds of real time however long the backoff is.

func TestBackoffSchedule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := Backoff{Base: 100 * time.Millisecond, Max: 1 * time.Second, Jitter: noJitter}
		start := time.Now()
		var calls []time.Duration
		err := b.Retry(t.Context(), 5, func(context.Context) error {
			calls = append(calls, time.Since(start))
			return errFlaky
		})

		// Waits of 100, 200, 400, 800 ms between five attempts.
		want := []time.Duration{0, 100 * time.Millisecond, 300 * time.Millisecond, 700 * time.Millisecond, 1500 * time.Millisecond}
		if !slices.Equal(calls, want) {
			t.Errorf("attempts at %v, want %v", calls, want)
		}
		if elapsed := time.Since(start); elapsed != want[len(want)-1] {
			t.Errorf("returned after %v: it waited after the final attempt", elapsed)
		}
		if !errors.Is(err, errFlaky) {
			t.Errorf("err = %v, want it to wrap the last error", err)
		}
	})
}

func TestBackoffCapsAtMax(t *testing.T) {
	t.Parallel()
	b := Backoff{Base: 100 * time.Millisecond, Max: 500 * time.Millisecond}
	got := []time.Duration{b.capFor(0), b.capFor(1), b.capFor(2), b.capFor(3), b.capFor(40)}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 500 * time.Millisecond, 500 * time.Millisecond}
	if !slices.Equal(got, want) {
		t.Errorf("caps = %v, want %v", got, want)
	}
}

func TestBackoffFullJitter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := Backoff{Base: 100 * time.Millisecond, Max: time.Second}
		var waits []time.Duration
		b.OnRetry = func(_ int, _ error, wait time.Duration) { waits = append(waits, wait) }
		_ = b.Retry(t.Context(), 4, func(context.Context) error { return errFlaky })

		if len(waits) != 3 {
			t.Fatalf("got %d waits, want 3", len(waits))
		}
		for i, w := range waits {
			if limit := b.capFor(i); w < 0 || w >= limit {
				t.Errorf("wait %d = %v, want within [0, %v)", i, w, limit)
			}
		}
	})
}

func TestRetryStopsWhenContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
		defer cancel()
		b := Backoff{Base: 100 * time.Millisecond, Max: time.Second, Jitter: noJitter}
		start, calls := time.Now(), 0
		err := b.Retry(ctx, 10, func(context.Context) error {
			calls++
			return errFlaky
		})

		// Attempts at 0 and 100 ms; the 200 ms wait is cut short at 250 ms.
		if calls != 2 {
			t.Errorf("calls = %d, want 2", calls)
		}
		if elapsed := time.Since(start); elapsed != 250*time.Millisecond {
			t.Errorf("returned after %v, want 250ms", elapsed)
		}
		if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errFlaky) {
			t.Errorf("err = %v, want both the deadline and the last error", err)
		}
	})
}

func TestRetrySucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		err := RetryWithBackoff(t.Context(), 5, func(context.Context) error {
			calls++
			if calls < 3 {
				return errFlaky
			}
			return nil
		})
		if err != nil || calls != 3 {
			t.Errorf("err = %v after %d calls, want nil after 3", err, calls)
		}
	})
}

func TestBreakerOpenTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var changes []string
		cb := NewCircuitBreaker[int](Settings{
			Name:        "test",
			Timeout:     30 * time.Second,
			ReadyToTrip: func(c Counts) bool { return c.ConsecutiveFailures >= 2 },
			OnStateChange: func(_ string, from, to State) {
				changes = append(changes, from.String()+"->"+to.String())
			},
		})
		called := 0
		failing := func() (int, error) { called++; return 0, errFlaky }
		working := func() (int, error) { called++; return 42, nil }

		_, _ = cb.Execute(failing)
		_, _ = cb.Execute(failing)
		if cb.State() != StateOpen {
			t.Fatalf("state = %v after 2 failures, want open", cb.State())
		}

		// While open, fn is not called at all.
		if _, err := cb.Execute(working); !errors.Is(err, ErrOpenState) || called != 2 {
			t.Fatalf("open breaker: err = %v, fn called %d times; want ErrOpenState and 2", err, called)
		}

		time.Sleep(30*time.Second - time.Nanosecond)
		if cb.State() != StateOpen {
			t.Fatalf("state = %v just before Timeout, want open", cb.State())
		}
		time.Sleep(time.Nanosecond)
		if cb.State() != StateHalfOpen {
			t.Fatalf("state = %v at Timeout, want half-open", cb.State())
		}

		// A failed probe re-opens; after another Timeout a good probe closes.
		_, _ = cb.Execute(failing)
		time.Sleep(30 * time.Second)
		if v, err := cb.Execute(working); err != nil || v != 42 {
			t.Fatalf("probe = %v, %v; want 42, nil", v, err)
		}

		want := []string{"closed->open", "open->half-open", "half-open->open", "open->half-open", "half-open->closed"}
		if !slices.Equal(changes, want) {
			t.Errorf("state changes = %v, want %v", changes, want)
		}
	})
}

func TestBreakerHalfOpenLimitsProbes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cb := NewCircuitBreaker[int](Settings{
			Timeout:     time.Second,
			ReadyToTrip: func(c Counts) bool { return c.ConsecutiveFailures >= 1 },
		})
		_, _ = cb.Execute(func() (int, error) { return 0, errFlaky })
		time.Sleep(time.Second)

		// The first probe blocks; a second concurrent call is rejected.
		release := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = cb.Execute(func() (int, error) { <-release; return 1, nil })
		}()
		synctest.Wait() // until the probe goroutine is blocked inside fn
		if _, err := cb.Execute(func() (int, error) { return 2, nil }); !errors.Is(err, ErrTooManyRequests) {
			t.Errorf("second probe: err = %v, want ErrTooManyRequests", err)
		}
		close(release)
		<-done
		if cb.State() != StateClosed {
			t.Errorf("state = %v after a good probe, want closed", cb.State())
		}
	})
}

func TestBreakerIgnoresCallerCancellation(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker[int](Settings{
		ReadyToTrip:  func(c Counts) bool { return c.ConsecutiveFailures >= 1 },
		IsSuccessful: func(err error) bool { return err == nil || errors.Is(err, context.Canceled) },
	})
	_, _ = cb.Execute(func() (int, error) { return 0, context.Canceled })
	if cb.State() != StateClosed {
		t.Errorf("state = %v; the caller's cancellation should not trip the breaker", cb.State())
	}
}

// TestRetriesDontDoubleCharge runs the real HTTP path against the fake
// provider: a 503, then a response lost after the charge, then success.
func TestRetriesDontDoubleCharge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		key         string
		wantCharges int64
	}{
		{"without key, the lost response is charged again", "", 2},
		{"with key, the provider replays the stored result", "order-42", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := NewFlakyProvider(0, 0, fail, lose, succeed)
			srv := httptest.NewServer(p)
			defer srv.Close()
			client := &AIClient{BaseURL: srv.URL, HTTP: srv.Client(), Timeout: 5 * time.Second}

			b := Backoff{Base: time.Millisecond, Max: time.Millisecond}
			var got string
			err := b.Retry(t.Context(), 4, func(ctx context.Context) error {
				var err error
				got, err = client.CallAIIdempotent(ctx, tt.key, "report")
				return err
			})
			if err != nil || got != "summary of report" {
				t.Fatalf("Retry = %q, %v", got, err)
			}
			if n := p.requests.Load(); n != 3 {
				t.Errorf("provider requests = %d, want 3", n)
			}
			if n := p.charges.Load(); n != tt.wantCharges {
				t.Errorf("charges = %d, want %d", n, tt.wantCharges)
			}
		})
	}
}
