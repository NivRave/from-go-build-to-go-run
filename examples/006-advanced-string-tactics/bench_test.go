package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/006-advanced-string-tactics
//
// Numbers vary by machine; compare the allocs/op and B/op columns.

// sink and sinkStr keep results alive, so the compiler can't discard the
// work or build a short result in a stack buffer.
var (
	sink    bool
	sinkStr string
	sinkInt int
)

var (
	mixedA = "Content-Type: Application/JSON; Charset=UTF-8"
	mixedB = "CONTENT-TYPE: APPLICATION/JSON; CHARSET=UTF-8"
	lowerA = "content-type: application/json; charset=utf-8"
	lowerB = "content-type: application/json; charset=utf-8"
)

// ToLower must build a new string for each input that has upper-case letters,
// so this is the slide's "two new strings" case.
func BenchmarkToLowerEqualMixed(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = strings.ToLower(mixedA) == strings.ToLower(mixedB) //nolint:staticcheck // intentional: benchmarks the slide p04 idiom
	}
}

func BenchmarkEqualFoldMixed(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = strings.EqualFold(mixedA, mixedB)
	}
}

// Already-lowercase ASCII: ToLower returns its input without allocating, so
// the slide's "creates two new strings" doesn't happen here.
func BenchmarkToLowerEqualLower(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = strings.ToLower(lowerA) == strings.ToLower(lowerB) //nolint:staticcheck // intentional: benchmarks the slide p04 idiom
	}
}

func BenchmarkEqualFoldLower(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = strings.EqualFold(lowerA, lowerB)
	}
}

// Four ways to build "host:port" (slide p07).
var (
	host = "db.internal.example.com"
	port = 5432
)

func BenchmarkJoinSprintf(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = fmt.Sprintf("%s:%d", host, port)
	}
}

func BenchmarkJoinPlus(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = host + ":" + strconv.Itoa(port)
	}
}

func BenchmarkJoinStringsJoin(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = strings.Join([]string{host, strconv.Itoa(port)}, ":")
	}
}

func BenchmarkJoinBuilder(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var sb strings.Builder
		sb.Grow(len(host) + 6)
		sb.WriteString(host)
		sb.WriteByte(':')
		sb.WriteString(strconv.Itoa(port))
		sinkStr = sb.String()
	}
}

// payload is large enough that a copy of it can't use the compiler's 32-byte
// stack buffer.
var payload = strings.Repeat("Senior Go ", 100)

// byteSum only reads b. noinline keeps the benchmarks honest: the call can't
// be merged into the loop.
//
//go:noinline
func byteSum(b []byte) int {
	n := 0
	for _, c := range b {
		n += int(c)
	}
	return n
}

// The []byte is never written and doesn't escape, so the compiler skips the
// copy: `go build -gcflags=-m` reports "zero-copy string->[]byte conversion".
func BenchmarkBytesConversionReadOnly(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = byteSum([]byte(payload))
	}
}

// Writing to the result forces a real copy, which is what slide p05 is about.
func BenchmarkBytesConversionMutated(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		buf := []byte(payload)
		buf[0] = 's'
		sinkInt = byteSum(buf)
	}
}

// The unsafe view shares the string's memory: no copy, but nothing may ever
// write through it.
func BenchmarkUnsafeView(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = byteSum(unsafe.Slice(unsafe.StringData(payload), len(payload)))
	}
}
