package main

import (
	"fmt"
	"runtime"
	"testing"
)

// The same 200 CPU-bound jobs at different pool sizes. Run with:
//
//	go test -run='^$' -bench=Pool ./examples/042-worker-pools
//
// Throughput improves up to about GOMAXPROCS workers and then flattens:
// extra workers have no extra cores to run on.
func BenchmarkPool(b *testing.B) {
	sizes := []int{1, 2, 4, runtime.GOMAXPROCS(0), 64}
	for _, workers := range sizes {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			for b.Loop() {
				Run(workers, 200, cpuWork)
			}
		})
	}
}
