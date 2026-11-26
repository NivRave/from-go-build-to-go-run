package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

// TestSlowDatabaseQuery runs in a synctest bubble, where time is virtual:
// the 100ms query and the 2s timeout cost no real time, and the elapsed
// durations are exact. t.Context() (Go 1.24) is the bubble's own context;
// it is cancelled when the test ends.
func TestSlowDatabaseQuery(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		cancelAfter time.Duration // 0: never cancel by hand
		wantErr     error
		wantElapsed time.Duration
	}{
		{"query fits in the budget", 2 * time.Second, 0, nil, queryLatency},
		{"deadline fires first", 50 * time.Millisecond, 0, context.DeadlineExceeded, 50 * time.Millisecond},
		{"caller cancels first", time.Second, 30 * time.Millisecond, context.Canceled, 30 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), tt.timeout)
				defer cancel()
				if tt.cancelAfter > 0 {
					time.AfterFunc(tt.cancelAfter, cancel)
				}

				start := time.Now()
				res, err := SlowDatabaseQuery(ctx)
				elapsed := time.Since(start)

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if err == nil && res == "" {
					t.Error("empty result on success")
				}
				if elapsed != tt.wantElapsed {
					t.Errorf("returned after %v, want exactly %v", elapsed, tt.wantElapsed)
				}
			})
		})
	}
}
