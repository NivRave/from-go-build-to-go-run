package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"testing/synctest"
	"time"
)

func TestRunSum(t *testing.T) {
	for _, workers := range []int{1, 2, 4, runtime.GOMAXPROCS(0), 100} {
		for _, n := range []int{0, 1, 1000} {
			t.Run(fmt.Sprintf("workers=%d/jobs=%d", workers, n), func(t *testing.T) {
				t.Parallel()
				stats := Run(workers, n, process)
				if want := n * (n + 1); stats.Sum != want {
					t.Errorf("sum = %d, want %d", stats.Sum, want)
				}
				if stats.Results != n {
					t.Errorf("results = %d, want %d", stats.Results, n)
				}
				handled := 0
				for _, c := range stats.PerWorker {
					handled += c
				}
				if handled != n || len(stats.PerWorker) != workers {
					t.Errorf("PerWorker = %v: %d workers handled %d jobs, want %d workers and %d jobs",
						stats.PerWorker, len(stats.PerWorker), handled, workers, n)
				}
			})
		}
	}
}

func TestFanInSum(t *testing.T) {
	for _, workers := range []int{1, 3, 16} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Parallel()
			if got, want := FanIn(workers, 500), 500*501; got != want {
				t.Errorf("FanIn sum = %d, want %d", got, want)
			}
		})
	}
}

// TestRunContextStopsEarly runs in a synctest bubble, so the 10ms jobs and
// the 45ms budget take no real time. Four workers finish four jobs each by
// 40ms (16 results). At 50ms each worker finishes a fifth job and finds both
// the results send and ctx.Done() ready, so 16 to 20 results get through.
// synctest.Test also fails if any goroutine is still blocked when the
// bubble ends, which makes this a leak test as well.
func TestRunContextStopsEarly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Millisecond)
		defer cancel()

		start := time.Now()
		stats, err := RunContext(ctx, 4, 1000, ioWork)
		elapsed := time.Since(start)

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", err)
		}
		if stats.Results < 16 || stats.Results > 20 {
			t.Errorf("results = %d, want 16 to 20", stats.Results)
		}
		if elapsed != 50*time.Millisecond {
			t.Errorf("returned after %v, want 50ms (when the in-flight jobs finish)", elapsed)
		}
	})
}

func TestRunContextCompletes(t *testing.T) {
	stats, err := RunContext(t.Context(), 4, 1000, process)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if want := 1000 * 1001; stats.Sum != want {
		t.Errorf("sum = %d, want %d", stats.Sum, want)
	}
}
