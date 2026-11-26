package main

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

// TestCloseDrainsCancelStops checks demo 4. synctest runs the pipeline on a
// fake clock, so the 10 ms of "work" per item and the 25 ms timeout take no
// real time and the cancel lands at exactly the same point on every run.
func TestCloseDrainsCancelStops(t *testing.T) {
	t.Parallel()
	all := items(20)
	const work = 10 * time.Millisecond

	t.Run("close drains every queued item", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			if got := runPipeline(context.Background(), all, 2, work); got != len(all) {
				t.Errorf("processed %d items, want all %d", got, len(all))
			}
		})
	})

	t.Run("cancel stops early", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			got := runPipeline(ctx, all, 2, work)
			if got >= len(all) {
				t.Fatalf("processed %d items, want fewer than %d", got, len(all))
			}
			// Two workers finish items at 10 and 20 ms; the ones started at
			// 20 ms are completed at 30 ms even though ctx ended at 25 ms.
			if want := 6; got != want {
				t.Errorf("processed %d items, want %d", got, want)
			}
		})
	})
}

func TestProduceStopsOnCancel(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		out := produce(ctx, items(100))
		<-out // the producer is running
		cancel()
		n := 0
		for range out { // must end: the producer closes out when it stops
			n++
		}
		if n >= 99 {
			t.Errorf("received %d more items after cancel, want the producer to stop early", n)
		}
	})
}
