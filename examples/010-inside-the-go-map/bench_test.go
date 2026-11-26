package main

import (
	"runtime"
	"strings"
	"testing"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/010-inside-the-go-map

const growN = 10_000

// Insert growth: without a hint the map starts as a single 8-slot group and
// rehashes into bigger tables as it fills; with a hint it is sized once.
func BenchmarkInsertNoHint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		m := make(map[int]int)
		for i := range growN {
			m[i] = i
		}
	}
}

func BenchmarkInsertHint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		m := make(map[int]int, growN)
		for i := range growN {
			m[i] = i
		}
	}
}

// Lookup cost depends on the key: the whole key is hashed, and on a match
// compared in full. An 8-byte key and a 1 KB key that differ only in their
// last byte make that cost visible.
var hit bool

func benchLookup(b *testing.B, keyLen int) {
	m := make(map[string]bool, 64)
	keys := make([]string, 64)
	for i := range keys {
		keys[i] = strings.Repeat("k", keyLen-1) + string(rune('A'+i))
		m[keys[i]] = true
	}
	// A fresh copy of each key, so lookups cannot short-circuit on the
	// string pointer and must compare the bytes.
	probe := make([]string, len(keys))
	for i, k := range keys {
		probe[i] = strings.Clone(k)
	}
	for b.Loop() {
		for _, k := range probe {
			hit = m[k]
		}
	}
}

func BenchmarkLookupKey8B(b *testing.B)  { benchLookup(b, 8) }
func BenchmarkLookupKey1KB(b *testing.B) { benchLookup(b, 1024) }

// GC cost of a map of values vs a map of pointers, with 1M entries. Each op
// is one full, forced garbage collection with the map live. Both maps are
// scanned (User contains a string), but the pointer map also makes the GC
// visit one million separate User objects.
type User struct {
	Name string
	Age  int
}

const gcN = 1_000_000

func BenchmarkGCMapOfValues(b *testing.B) {
	m := make(map[int]User, gcN)
	for i := range gcN {
		m[i] = User{Name: "gopher", Age: i}
	}
	benchGC(b)
	runtime.KeepAlive(m)
}

func BenchmarkGCMapOfPointers(b *testing.B) {
	m := make(map[int]*User, gcN)
	for i := range gcN {
		m[i] = &User{Name: "gopher", Age: i}
	}
	benchGC(b)
	runtime.KeepAlive(m)
}

func benchGC(b *testing.B) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for b.Loop() {
		runtime.GC()
	}
	runtime.ReadMemStats(&after)
	if gcs := after.NumGC - before.NumGC; gcs > 0 {
		b.ReportMetric(float64(after.PauseTotalNs-before.PauseTotalNs)/float64(gcs), "pause-ns/gc")
	}
}
