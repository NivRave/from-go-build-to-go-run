package main

import (
	"slices"
	"strconv"
	"testing"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/009-maps

// keys holds 1000 distinct keys, built once so key formatting is not part of
// the measurement. 1000 matches the "1,000 DB rows" example on slide p03.
var keys = func() []string {
	ks := make([]string, 1000)
	for i := range ks {
		ks[i] = "row-" + strconv.Itoa(i)
	}
	return ks
}()

// BenchmarkInsertNoHint grows the map from empty, rehashing into bigger
// tables several times along the way.
func BenchmarkInsertNoHint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		m := make(map[string]int)
		for i, k := range keys {
			m[k] = i
		}
	}
}

// BenchmarkInsertHint sizes the map up front, so no growth happens.
func BenchmarkInsertHint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		m := make(map[string]int, len(keys))
		for i, k := range keys {
			m[k] = i
		}
	}
}

// Slide p02: every map lookup hashes the key. For a handful of short keys a
// linear scan of a slice can be as fast or faster.
var (
	small    = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS"}
	smallSet = func() map[string]bool {
		m := make(map[string]bool, len(small))
		for _, k := range small {
			m[k] = true
		}
		return m
	}()
	found bool // package-level sink so the lookups are not optimised away
)

func BenchmarkLookup8Slice(b *testing.B) {
	for b.Loop() {
		for _, k := range small {
			found = slices.Contains(small, k)
		}
	}
}

func BenchmarkLookup8Map(b *testing.B) {
	for b.Loop() {
		for _, k := range small {
			found = smallSet[k]
		}
	}
}
