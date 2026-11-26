package main

import (
	"bytes"
	"strings"
	"testing"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/005-string-rune-and-byte
//
// The numbers vary by machine; the allocs/op column is the point.

// pieces is how many times "Go" is appended in the concatenation benchmarks.
const pieces = 100

// BenchmarkConcatPlus builds the string with +=, which allocates a new,
// longer string on every iteration and copies everything so far (slide p08).
func BenchmarkConcatPlus(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := ""
		for range pieces {
			s += "Go"
		}
		_ = s
	}
}

// BenchmarkBuilder lets strings.Builder grow its buffer by doubling.
func BenchmarkBuilder(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var sb strings.Builder
		for range pieces {
			sb.WriteString("Go")
		}
		_ = sb.String()
	}
}

// BenchmarkBuilderGrow pre-sizes the buffer, so there is exactly one
// allocation, and String() returns it without copying.
func BenchmarkBuilderGrow(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var sb strings.Builder
		sb.Grow(pieces * len("Go"))
		for range pieces {
			sb.WriteString("Go")
		}
		_ = sb.String()
	}
}

// BenchmarkRepeat is the right tool when the pieces are all the same.
func BenchmarkRepeat(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = strings.Repeat("Go", pieces)
	}
}

// text is long enough (1100 bytes) that []byte(text) can't use the
// compiler's 32-byte stack buffer and has to allocate on the heap.
var text = strings.Repeat("Go is fun. ", 100)

// BenchmarkCountViaBytes converts to []byte first, paying a copy (slide p07).
func BenchmarkCountViaBytes(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = bytes.Count([]byte(text), []byte("Go"))
	}
}

// BenchmarkCountOnString works on the string directly: no copy at all.
func BenchmarkCountOnString(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = strings.Count(text, "Go")
	}
}

// BenchmarkRangeBytesConversion shows one of the conversions the compiler
// optimises away: `range []byte(s)` iterates the string's own bytes.
func BenchmarkRangeBytesConversion(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for _, c := range []byte(text) {
			if c == 'G' {
				n++
			}
		}
		_ = n
	}
}
