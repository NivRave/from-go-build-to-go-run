package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

// atomic.Int64.Add vs a mutex-protected increment. Compare without and with
// contention:
//
//	go test -run='^$' -bench=. -cpu=1,8 ./examples/038-atomic
//
// With -cpu=1 nothing contends: the uncontended mutex is itself just an
// atomic CAS to lock and another to unlock, so the gap is small. With -cpu=8
// every operation has to pull the cache line from another core, and both
// get slower.

func BenchmarkIncrement(b *testing.B) {
	b.Run("AtomicInt64", func(b *testing.B) {
		var n atomic.Int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				n.Add(1)
			}
		})
	})
	b.Run("Mutex", func(b *testing.B) {
		var mu sync.Mutex
		var n int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				n++
				mu.Unlock()
			}
		})
	})
}
