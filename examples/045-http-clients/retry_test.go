package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
)

var errTransient = errors.New("transient")

// Inside synctest.Test the clock is fake: time only moves when every
// goroutine in the bubble is blocked, so backoff waits take no real time and
// the measured gaps are exact.

func TestRetryBackoffSchedule(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var calls []time.Time
		err := Retry(t.Context(), 5, func(context.Context) error {
			calls = append(calls, time.Now())
			return errTransient
		})
		returned := time.Now()

		if len(calls) != 5 {
			t.Fatalf("fn called %d times, want 5", len(calls))
		}
		if !errors.Is(err, errTransient) {
			t.Errorf("err = %v, want it to wrap errTransient", err)
		}
		for i := 1; i < len(calls); i++ {
			gap := calls[i].Sub(calls[i-1])
			ceiling := min(backoffMax, backoffBase<<(i-1))
			if gap < 0 || gap >= ceiling {
				t.Errorf("wait before attempt %d = %v, want in [0, %v)", i+1, gap, ceiling)
			}
		}
		if after := returned.Sub(calls[len(calls)-1]); after != 0 {
			t.Errorf("Retry waited %v after the last attempt, want 0", after)
		}
	})
}

func TestRetryStopsOnSuccess(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		err := Retry(t.Context(), 5, func(context.Context) error {
			calls++
			if calls < 3 {
				return errTransient
			}
			return nil
		})
		if err != nil || calls != 3 {
			t.Errorf("err = %v, calls = %d; want nil, 3", err, calls)
		}
	})
}

func TestRetryHonoursRetryAfter(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var calls []time.Time
		_ = Retry(t.Context(), 3, func(context.Context) error {
			calls = append(calls, time.Now())
			return RetryAfter(errTransient, 3*time.Second)
		})
		for i := 1; i < len(calls); i++ {
			if gap := calls[i].Sub(calls[i-1]); gap != 3*time.Second {
				t.Errorf("gap %d = %v, want exactly 3s from Retry-After", i, gap)
			}
		}
	})
}

func TestRetryPermanentStopsImmediately(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		start := time.Now()
		err := Retry(t.Context(), 5, func(context.Context) error {
			calls++
			return Permanent(errTransient)
		})
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
		if !errors.Is(err, errTransient) {
			t.Errorf("err = %v, want it to wrap errTransient", err)
		}
		if waited := time.Since(start); waited != 0 {
			t.Errorf("waited %v, want 0", waited)
		}
	})
}

func TestRetryCancelledDuringBackoff(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		calls := 0
		start := time.Now()
		err := Retry(ctx, 5, func(context.Context) error {
			calls++
			return RetryAfter(errTransient, 10*time.Second) // far beyond the deadline
		})
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
		if waited := time.Since(start); waited != 50*time.Millisecond {
			t.Errorf("returned after %v, want 50ms (the ctx deadline)", waited)
		}
		if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errTransient) {
			t.Errorf("err = %v, want both DeadlineExceeded and errTransient in its tree", err)
		}
	})
}

func TestRetryCancelledBeforeStart(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls := 0
	err := Retry(ctx, 3, func(context.Context) error { calls++; return nil })
	if calls != 0 || !errors.Is(err, context.Canceled) {
		t.Errorf("calls = %d, err = %v; want 0, context.Canceled", calls, err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		in5s := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
		tests := []struct {
			header string
			want   time.Duration
			ok     bool
		}{
			{"", 0, false},
			{"0", 0, true},
			{"7", 7 * time.Second, true},
			{"-1", 0, false},
			{"soon", 0, false},
			{in5s, 5 * time.Second, true},
		}
		for _, tt := range tests {
			got, ok := parseRetryAfter(tt.header)
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseRetryAfter(%q) = %v, %t; want %v, %t", tt.header, got, ok, tt.want, tt.ok)
			}
		}
	})
}

// TestFetchClassifies uses a real loopback httptest server, so it stays
// outside synctest (network I/O doesn't count as "durably blocked").
func TestFetchClassifies(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/404":
			http.NotFound(w, r)
		case "/429":
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte("hello"))
		}
	}))
	t.Cleanup(srv.Close) // not defer: the parallel subtests run after this function returns

	tests := []struct {
		path          string
		wantBody      string
		wantErr       bool
		wantPermanent bool
		wantAfter     time.Duration // -1: no Retry-After
	}{
		{"/ok", "hello", false, false, -1},
		{"/404", "", true, true, -1},
		{"/429", "", true, false, 2 * time.Second},
		{"/500", "", true, false, -1},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			body, err := fetch(t.Context(), srv.Client(), srv.URL+tt.path)
			if body != tt.wantBody || (err != nil) != tt.wantErr {
				t.Fatalf("fetch = %q, %v", body, err)
			}
			_, permanent := errors.AsType[*permanentError](err)
			if permanent != tt.wantPermanent {
				t.Errorf("permanent = %t, want %t (err = %v)", permanent, tt.wantPermanent, err)
			}
			ra, hasAfter := errors.AsType[*retryAfterError](err)
			if hasAfter != (tt.wantAfter >= 0) || (hasAfter && ra.after != tt.wantAfter) {
				t.Errorf("Retry-After = %v (present %t), want %v", ra, hasAfter, tt.wantAfter)
			}
		})
	}
}

func Example_retryFlaky() {
	srv := newTestServer()
	defer srv.Close()
	retryFlaky(srv.URL)
	// Output:
	// /flaky attempt 1: server returned 503 Service Unavailable (Retry-After: 0s)
	// /flaky attempt 2: server returned 503 Service Unavailable (Retry-After: 0s)
	// /flaky attempt 3: "ok"
	// /flaky result: "ok", err = <nil>
}

func Example_retryDead() {
	retryDead(deadURL())
	// Output:
	// dead endpoint: 3 attempts, err != nil: true, last error is a dial error: true
}
