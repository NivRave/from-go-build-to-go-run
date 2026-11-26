package main

import "testing"

// Benchmarks mirroring demos 1-4. Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/015-pointers
//
// allocs/op is the number to read: 1 means the value went to the heap,
// 0 means it stayed on the stack.

func BenchmarkNewUserPtr(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = newUserPtr().Age
	}
}

func BenchmarkNewUserVal(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = newUserVal().Age
	}
}

func BenchmarkProcessPassDown(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		u := User{Name: "Ada", Age: 36}
		sinkInt = process(&u)
	}
}

func BenchmarkKeepGlobal(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		u := User{Name: "Ada", Age: 36}
		keep(&u)
	}
}

func BenchmarkPassStruct(b *testing.B) {
	b.ReportAllocs()
	r := Report{ID: 1}
	for b.Loop() {
		logReport(r)
	}
}

func BenchmarkBoxInterface(b *testing.B) {
	b.ReportAllocs()
	r := Report{ID: 1}
	for b.Loop() {
		logValue(r)
	}
}

func BenchmarkSliceLocalConst(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = localSmall()
	}
}

func BenchmarkSliceLocalVariable(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = localLarge(n)
	}
}

func BenchmarkSliceReturned(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInts = returned()
	}
}
