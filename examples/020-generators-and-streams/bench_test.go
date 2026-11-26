package main

import "testing"

// Sum 1..n three ways. Run with:
//
//	go test -bench=. -benchmem ./examples/020-generators-and-streams
//
// CollectAll allocates O(n) bytes per op; both streaming versions allocate a
// constant amount no matter how large n is.
const n = 100_000

func BenchmarkCollectAll(b *testing.B) {
	for b.Loop() {
		all := make([]int, 0) // grows by repeated append, like slide p02
		for v := 1; v <= n; v++ {
			all = append(all, v)
		}
		sum := 0
		for _, v := range all {
			sum += v
		}
		_ = sum
	}
}

func BenchmarkClosureGenerator(b *testing.B) {
	for b.Loop() {
		next, sum := IntGenerator(1, n), 0
		for v, ok := next(); ok; v, ok = next() {
			sum += v
		}
		_ = sum
	}
}

func BenchmarkIterSeq(b *testing.B) {
	for b.Loop() {
		sum := 0
		for v := range Ints(1, n) {
			sum += v
		}
		_ = sum
	}
}
