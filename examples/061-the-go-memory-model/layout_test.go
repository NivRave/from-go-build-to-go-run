package main

import (
	"testing"
	"unsafe"
)

// Sum one field over a million structs. BadStruct is 24 bytes and GoodStruct
// 16 on 64-bit, so the same loop streams 24 MB versus 16 MB through the
// cache. Run with:
//
//	go test -run='^$' -bench=Sum -benchmem ./examples/061-the-go-memory-model
//
// The slice-MB metric shows the size difference; how much of it turns into
// ns/op depends on your CPU's caches and memory bandwidth.
const elems = 1_000_000

var sink int64

func BenchmarkSumBadStruct(b *testing.B) {
	s := make([]BadStruct, elems)
	for i := range s {
		s[i].C = int32(i)
	}
	for b.Loop() {
		var sum int64
		for i := range s {
			sum += int64(s[i].C)
		}
		sink = sum
	}
	reportSize(b, len(s), unsafe.Sizeof(s[0])) // after the loop: b.Loop resets metrics
}

func BenchmarkSumGoodStruct(b *testing.B) {
	s := make([]GoodStruct, elems)
	for i := range s {
		s[i].C = int32(i)
	}
	for b.Loop() {
		var sum int64
		for i := range s {
			sum += int64(s[i].C)
		}
		sink = sum
	}
	reportSize(b, len(s), unsafe.Sizeof(s[0])) // after the loop: b.Loop resets metrics
}

// reportSize reports how many megabytes the slice occupies, next to ns/op.
func reportSize(b *testing.B, n int, size uintptr) {
	b.Helper()
	b.ReportMetric(float64(n)*float64(size)/1e6, "slice-MB")
}

func TestLayoutSavings(t *testing.T) {
	t.Parallel()
	// On every GOARCH the reordered structs are no larger than the originals.
	if unsafe.Sizeof(GoodStruct{}) > unsafe.Sizeof(BadStruct{}) {
		t.Errorf("GoodStruct (%d) is larger than BadStruct (%d)", unsafe.Sizeof(GoodStruct{}), unsafe.Sizeof(BadStruct{}))
	}
	if unsafe.Sizeof(FirstField{}) >= unsafe.Sizeof(LastField{}) {
		t.Errorf("FirstField (%d) should be smaller than LastField (%d)", unsafe.Sizeof(FirstField{}), unsafe.Sizeof(LastField{}))
	}
}
