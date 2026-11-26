package main

import "testing"

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/014-pointers-vs-value-receivers
//
// Slide p05: copying a small struct is not slower than passing a pointer,
// and passing a pointer down the call stack does not allocate. What does
// allocate is returning a pointer from a function that is not inlined.

type Point struct{ X, Y, Z float64 } // 24 bytes

var sinkF float64
var sinkP *Point

//go:noinline
func lengthSqValue(p Point) float64 { return p.X*p.X + p.Y*p.Y + p.Z*p.Z }

//go:noinline
func lengthSqPointer(p *Point) float64 { return p.X*p.X + p.Y*p.Y + p.Z*p.Z }

//go:noinline
func newPointValue(x float64) Point { return Point{x, x, x} }

//go:noinline
func newPointPointer(x float64) *Point { return &Point{x, x, x} }

func BenchmarkPassByValue(b *testing.B) {
	b.ReportAllocs()
	p := Point{1, 2, 3}
	for b.Loop() {
		sinkF = lengthSqValue(p)
	}
}

func BenchmarkPassByPointer(b *testing.B) {
	b.ReportAllocs()
	p := Point{1, 2, 3}
	for b.Loop() {
		sinkF = lengthSqPointer(&p) // p does not escape: 0 allocs
	}
}

func BenchmarkReturnValue(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		p := newPointValue(1)
		sinkF = p.X
	}
}

func BenchmarkReturnPointer(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkP = newPointPointer(1) // the Point outlives the call: 1 alloc
	}
}
