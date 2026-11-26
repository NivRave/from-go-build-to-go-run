package main

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

// TestTickerLoopDropsTicks runs tickerLoop in a synctest bubble. Time is
// virtual, so the 550ms run takes no real time and the tick counts are
// exact: a 100ms ticker under a 550ms context, with handlers of different
// speeds. Handler durations are chosen so no handler finishes at the same
// instant as a tick or the deadline.
func TestTickerLoopDropsTicks(t *testing.T) {
	tests := []struct {
		name         string
		work         time.Duration
		wantTicks    int
		wantReturnAt time.Duration
	}{
		// Ticks at 100..500ms; the deadline at 550ms ends the loop.
		{"instant handler", 0, 5, 550 * time.Millisecond},
		// Handles 100 (busy to 220), 200 (to 340), 300 (to 460), 400 (to
		// 580). At 580 the 500 tick is pending but ctx is done.
		{"120ms handler", 120 * time.Millisecond, 4, 580 * time.Millisecond},
		// Handles 100 (busy to 350): the 200 tick waits, 300 is dropped.
		// Handles 200 (busy to 600), then sees ctx is done.
		{"250ms handler", 250 * time.Millisecond, 2, 600 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 550*time.Millisecond)
				defer cancel()

				start := time.Now()
				got := tickerLoop(ctx, 100*time.Millisecond, func(time.Time) { time.Sleep(tt.work) })
				returnedAt := time.Since(start)

				if got != tt.wantTicks {
					t.Errorf("handled %d ticks, want %d", got, tt.wantTicks)
				}
				if returnedAt != tt.wantReturnAt {
					t.Errorf("returned at %v, want %v", returnedAt, tt.wantReturnAt)
				}
			})
		})
	}
}

// TestResetDiscardsStaleValue checks the Go 1.23 guarantee that section 4
// relies on: after Reset, a value from the previous expiry is never
// received.
func TestResetDiscardsStaleValue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		timer := time.NewTimer(10 * time.Millisecond)
		time.Sleep(30 * time.Millisecond) // fired, not received

		start := time.Now()
		timer.Reset(50 * time.Millisecond)
		<-timer.C
		if got := time.Since(start); got != 50*time.Millisecond {
			t.Errorf("received after %v, want 50ms (a stale value leaked through Reset)", got)
		}
	})
}
