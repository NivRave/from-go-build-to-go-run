package main

import "testing"

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/025-mastering-defer
//
// Since Go 1.14 most defers are "open-coded": the compiler inlines the
// deferred call at each return, so BenchmarkDefer costs about the same as
// BenchmarkDirect. A defer inside a loop can't be open-coded (the compiler
// can't know how many will be pending), so it falls back to a runtime defer
// record, which is measurably slower. Open coding also needs at most 8
// defers in the function.

var counter int

//go:noinline
func work() { counter++ }

//go:noinline
func direct() { work() }

//go:noinline
func deferred() { defer work() }

//go:noinline
func deferredInLoop(n int) {
	for range n {
		defer work()
	}
}

func BenchmarkDirect(b *testing.B) {
	for b.Loop() {
		direct()
	}
}

func BenchmarkDefer(b *testing.B) {
	for b.Loop() {
		deferred()
	}
}

// One iteration, so it does the same work as the two above; the only
// difference is that the defer sits inside a loop.
func BenchmarkDeferInLoop(b *testing.B) {
	for b.Loop() {
		deferredInLoop(1)
	}
}
