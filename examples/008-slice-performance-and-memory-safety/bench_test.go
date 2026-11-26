package main

import (
	"runtime"
	"testing"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/008-slice-performance-and-memory-safety
//
// Numbers vary by machine. Compare allocs/op for the filters, and the
// relative ns/op (time per full GC) for the GC benchmarks.

const filterN = 10_000

var (
	filterSrc  = makeInts(filterN)
	filterSink []int
)

func makeInts(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// BenchmarkFilterInPlace refills a reusable buffer every iteration (the
// filter destroys it) and then filters it in place: no allocation.
func BenchmarkFilterInPlace(b *testing.B) {
	b.ReportAllocs()
	buf := make([]int, filterN)
	for b.Loop() {
		copy(buf, filterSrc)
		filterSink = FilterInPlace(buf)
	}
}

// BenchmarkFilterAlloc leaves its input alone and pays one allocation.
func BenchmarkFilterAlloc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		filterSink = FilterAlloc(filterSrc)
	}
}

// GC cost (slide p08). Each benchmark keeps 1M elements reachable and times
// a full runtime.GC(). The GC has to trace every pointer it can reach, so
// the cost depends on how many pointers the live heap contains, not on
// whether you used values or pointers as such.

const gcN = 1_000_000

// Item has a pointer inside it (the string's data pointer), so a []Item
// backing array must still be scanned, element by element.
type Item struct {
	ID   int
	Name string
}

// Point has no pointers. A []Point is allocated "noscan": the GC never looks
// inside it.
type Point struct {
	X, Y int
}

var (
	liveItems    []Item
	liveItemPtrs []*Item
	livePoints   []Point
)

func BenchmarkGCSliceOfValues(b *testing.B) {
	liveItems = make([]Item, gcN)
	for i := range liveItems {
		liveItems[i] = Item{ID: i, Name: "item"}
	}
	b.Cleanup(func() { liveItems = nil })
	for b.Loop() {
		runtime.GC()
	}
}

func BenchmarkGCSliceOfPointers(b *testing.B) {
	liveItemPtrs = make([]*Item, gcN)
	for i := range liveItemPtrs {
		liveItemPtrs[i] = &Item{ID: i, Name: "item"} // 1M separate heap objects
	}
	b.Cleanup(func() { liveItemPtrs = nil })
	for b.Loop() {
		runtime.GC()
	}
}

func BenchmarkGCSliceNoPointers(b *testing.B) {
	livePoints = make([]Point, gcN)
	for i := range livePoints {
		livePoints[i] = Point{X: i, Y: i}
	}
	b.Cleanup(func() { livePoints = nil })
	for b.Loop() {
		runtime.GC()
	}
}
