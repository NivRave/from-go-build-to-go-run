package main

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

var errDown = errors.New("service down")

func TestBreakerOpensAfterThreshold(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		br := NewBreaker(3, time.Second)
		calls := 0
		failing := func() error { calls++; return errDown }

		for i := range 3 {
			if got := br.State(); got != Closed {
				t.Fatalf("before failure %d: state = %v, want closed", i+1, got)
			}
			if err := br.Do(failing); !errors.Is(err, errDown) {
				t.Fatalf("call %d: err = %v, want errDown", i+1, err)
			}
		}
		if got := br.State(); got != Open {
			t.Fatalf("after 3 failures: state = %v, want open", got)
		}
		if err := br.Do(failing); !errors.Is(err, ErrOpen) {
			t.Errorf("open breaker: err = %v, want ErrOpen", err)
		}
		if calls != 3 {
			t.Errorf("fn called %d times, want 3 (open must not call fn)", calls)
		}
	})
}

func TestBreakerSuccessResetsCount(t *testing.T) {
	t.Parallel()
	br := NewBreaker(2, time.Second)
	_ = br.Do(func() error { return errDown })
	_ = br.Do(func() error { return nil }) // resets the consecutive count
	_ = br.Do(func() error { return errDown })
	if got := br.State(); got != Closed {
		t.Errorf("state = %v, want closed: failures were not consecutive", got)
	}
}

func TestBreakerHalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		probeErr  error
		wantState State
	}{
		{"probe succeeds", nil, Closed},
		{"probe fails", errDown, Open},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				br := NewBreaker(1, time.Second)
				_ = br.Do(func() error { return errDown })

				time.Sleep(999 * time.Millisecond)
				if got := br.State(); got != Open {
					t.Fatalf("just before the timeout: state = %v, want open", got)
				}
				time.Sleep(time.Millisecond)
				if got := br.State(); got != HalfOpen {
					t.Fatalf("at the timeout: state = %v, want half-open", got)
				}

				_ = br.Do(func() error { return tt.probeErr })
				if got := br.State(); got != tt.wantState {
					t.Errorf("after the probe: state = %v, want %v", got, tt.wantState)
				}
			})
		})
	}
}

func TestBreakerSingleProbe(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		br := NewBreaker(1, time.Second)
		_ = br.Do(func() error { return errDown })
		time.Sleep(time.Second)

		release := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {
			_ = br.Do(func() error { <-release; return nil }) // the probe, held open
		})
		synctest.Wait() // the probe goroutine is now blocked inside fn

		if err := br.Do(func() error { return nil }); !errors.Is(err, ErrOpen) {
			t.Errorf("second caller during the probe: err = %v, want ErrOpen", err)
		}
		close(release)
		wg.Wait()
		if got := br.State(); got != Closed {
			t.Errorf("after a successful probe: state = %v, want closed", got)
		}
	})
}

func Example_breakerDemo() {
	srv := newTestServer()
	defer srv.Close()
	breakerDemo(deadURL(), srv.URL)
	// Output:
	// call 1: dial error                state=closed    requests sent=1
	// call 2: dial error                state=closed    requests sent=2
	// call 3: dial error                state=open      requests sent=3
	// call 4: circuit breaker is open   state=open      requests sent=3
	// call 5: circuit breaker is open   state=open      requests sent=3
	// after the open timeout: state=half-open
	// call 6: ok                        state=closed    requests sent=4
}
