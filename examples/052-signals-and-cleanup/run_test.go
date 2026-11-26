package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// TestRunWithCancelledContext is slide p07's test done properly: inject the
// "signal" as a cancelled context, wait for run to return, then assert. The
// synctest bubble makes the 500 ms zombie timeout take no real time.
func TestRunWithCancelledContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		var out bytes.Buffer
		done := make(chan error, 1)
		start := time.Now()
		go func() { done <- run(ctx, nil, &out) }()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("run: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("run did not return within 1s")
		}
		if elapsed := time.Since(start); elapsed != 500*time.Millisecond {
			t.Errorf("run took %v, want exactly the 500ms zombie timeout", elapsed)
		}

		got := out.String()
		for _, want := range []string{
			"signal: context canceled",
			"workers: 2 of 3 stopped",
			"zombie timeout: gave up after 500ms",
			"teardown body ran 1 time(s)",
			"run: deferred calls ran",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("output is missing %q:\n%s", want, got)
			}
		}
		if n := strings.Count(got, "teardown: closing db"); n != 1 {
			t.Errorf("db closed %d times, want 1", n)
		}
	})
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var out bytes.Buffer
	if err := run(t.Context(), []string{"-nope"}, &out); err == nil {
		t.Fatal("run accepted an unknown flag")
	}
}

func TestWorkerPoolWait(t *testing.T) {
	tests := []struct {
		name        string
		release     bool // let the zombie finish before the timeout
		wantStopped int
		wantOK      bool
		wantElapsed time.Duration
	}{
		{"zombie hangs", false, 2, false, time.Second},
		{"zombie released", true, 3, true, 100 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				release := make(chan struct{})
				defer func() {
					if !tt.release {
						close(release)
					}
				}()
				pool := startWorkers(ctx, 3, release)
				cancel()
				if tt.release {
					time.AfterFunc(100*time.Millisecond, func() { close(release) })
				}

				start := time.Now()
				stopped, ok := pool.wait(time.Second)
				if stopped != tt.wantStopped || ok != tt.wantOK {
					t.Errorf("wait = (%d, %t), want (%d, %t)", stopped, ok, tt.wantStopped, tt.wantOK)
				}
				if elapsed := time.Since(start); elapsed != tt.wantElapsed {
					t.Errorf("wait took %v, want %v", elapsed, tt.wantElapsed)
				}
			})
		})
	}
}

// TestTeardownOnce calls teardown from many goroutines at once. Without
// sync.OnceValue the second close(events) would panic; with it, the body runs
// once and every caller sees the first call's result. Run with -race.
func TestTeardownOnce(t *testing.T) {
	var out bytes.Buffer
	res := newResources(&out)

	const callers = 20
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() { errs[i] = res.teardown() })
	}
	wg.Wait()

	if n := res.runs.Load(); n != 1 {
		t.Errorf("teardown body ran %d times, want 1", n)
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d got %v, want nil", i, err)
		}
	}
	// Calling Close directly a second time shows what Once is shielding.
	if err := res.db.Close(); !errors.Is(err, errAlreadyClosed) {
		t.Errorf("second direct Close = %v, want errAlreadyClosed", err)
	}
}

func TestCloseTwicePanics(t *testing.T) {
	if got := closeTwice(); got == nil {
		t.Fatal("closing a channel twice did not panic")
	}
}
