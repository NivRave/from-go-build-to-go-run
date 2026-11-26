package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

// False sharing (the gap on slide p05). Each goroutine increments only its
// own counter, so there is no logical sharing at all. But adjacent
// atomic.Int64 values sit 8 bytes apart, so eight of them share one 64-byte
// cache line, and every increment forces the line to move between cores.
// Padding each counter to 128 bytes gives every counter its own line (128
// also covers Apple M-series lines and Intel's adjacent-line prefetch).
//
//	go test -run='^$' -bench=Counters ./examples/061-the-go-memory-model

type paddedCounter struct {
	n atomic.Int64
	_ [120]byte // 8 + 120 = 128 bytes
}

const incrementsPerGoroutine = 10_000

func goroutines() int { return min(runtime.GOMAXPROCS(0), 8) }

func BenchmarkCountersAdjacent(b *testing.B) {
	n := goroutines()
	counters := make([]atomic.Int64, n)
	for b.Loop() {
		var wg sync.WaitGroup
		for g := range n {
			wg.Go(func() {
				for range incrementsPerGoroutine {
					counters[g].Add(1)
				}
			})
		}
		wg.Wait()
	}
}

func BenchmarkCountersPadded(b *testing.B) {
	n := goroutines()
	counters := make([]paddedCounter, n)
	for b.Loop() {
		var wg sync.WaitGroup
		for g := range n {
			wg.Go(func() {
				for range incrementsPerGoroutine {
					counters[g].n.Add(1)
				}
			})
		}
		wg.Wait()
	}
}

func TestPaddedCounterSize(t *testing.T) {
	t.Parallel()
	if got := unsafe.Sizeof(paddedCounter{}); got != 128 {
		t.Errorf("paddedCounter is %d bytes, want 128", got)
	}
}

// TestPublication runs the three correct publication patterns many times
// concurrently. Under go test -race it fails if any of them has a data race;
// without -race it still checks that the reader always sees the payload.
func TestPublication(t *testing.T) {
	t.Parallel()
	for name, publish := range map[string]func() int{
		"channel": publishChannel,
		"mutex":   publishMutex,
		"atomic":  publishAtomic,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var wg sync.WaitGroup
			for range 200 {
				wg.Go(func() {
					if got := publish(); got != answer {
						t.Errorf("%s: read %d, want %d", name, got, answer)
					}
				})
			}
			wg.Wait()
		})
	}
}

func Example_memoryModel() {
	memoryModel()
	// Output:
	// channel: 42
	// mutex:   42
	// atomic:  42
}
