package main

import "testing"

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/007-arrays-vs-slices
//
// Numbers vary by machine; compare B/op, allocs/op and the relative ns/op.

const n = 10_000

// sink keeps results alive so the compiler can't discard the work or keep
// the slices on the stack.
var sink []int

// BenchmarkAppendGrow starts from nil and lets append reallocate as it goes.
func BenchmarkAppendGrow(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var s []int
		for i := range n {
			s = append(s, i)
		}
		sink = s
	}
}

// BenchmarkAppendPrealloc sizes the backing array once (slide p05).
func BenchmarkAppendPrealloc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := make([]int, 0, n)
		for i := range n {
			s = append(s, i)
		}
		sink = s
	}
}

// big is 64 KiB: passing it by value copies all of it on every call.
type big [1 << 16]byte

var (
	bigArray big
	bigSlice = bigArray[:]
	total    int
)

//go:noinline
func firstByteArray(a big) int { return int(a[0]) }

//go:noinline
func firstByteSlice(s []byte) int { return int(s[0]) }

func BenchmarkPassArrayByValue(b *testing.B) {
	for b.Loop() {
		total += firstByteArray(bigArray)
	}
}

func BenchmarkPassSlice(b *testing.B) {
	for b.Loop() {
		total += firstByteSlice(bigSlice)
	}
}

// TestEmptySliceDoesNotAllocate backs the README note on slide p08: []int{}
// is non-nil, but its zero-length backing array is the runtime's shared
// zero-size base, so creating one costs no heap allocation even when it
// escapes.
func TestEmptySliceDoesNotAllocate(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		sink = []int{}
	})
	if allocs != 0 {
		t.Errorf("[]int{} allocated %.0f times per run, want 0", allocs)
	}
	if sink == nil {
		t.Error("[]int{} is nil, want non-nil")
	}
}
