// Command inside-the-go-map demonstrates what you can observe of the map
// implementation from the outside: the size of a map value, randomised
// iteration, memory that is never given back, and the real cost per entry of
// a set.
//
// Slides: slides/010-inside-the-go-map.pdf
//
// Run:
//
//	go run ./examples/010-inside-the-go-map
//	go test -run='^$' -bench=. -benchmem ./examples/010-inside-the-go-map
package main

import (
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
	"unsafe"
)

func main() {
	section("1. A map value is one pointer (slide p02)")
	mapHeaderSize()

	section("2. Random iteration order (slide p03)")
	randomIteration()

	section("3. Maps grow but never shrink (slides p05, p09)")
	neverShrinks()

	section("4. Memory per entry of a set (slide p08)")
	setMemory()
}

func mapHeaderSize() {
	var m1 map[string]int
	m2 := make(map[string]int, 100)

	// Both are a single pointer to the runtime's map structure (since Go 1.24
	// a maps.Map, previously an hmap). That is 8 bytes on 64-bit platforms,
	// however much room the map has reserved.
	fmt.Printf("nil map:        Sizeof = %d bytes, len = %d\n", unsafe.Sizeof(m1), len(m1))
	fmt.Printf("map, hint 100:  Sizeof = %d bytes, len = %d\n", unsafe.Sizeof(m2), len(m2))
}

func randomIteration() {
	// The slide's map. Maps with up to 8 entries are a single group of 8
	// slots, filled in insertion order. Each range starts at a random one of
	// the 8 slots and wraps around, so with only 3 entries most starting
	// points still yield Alpha, Beta, Gamma. The order is still not
	// guaranteed: run it a few times and the rotations show up.
	small := map[string]int{"Alpha": 1, "Beta": 2, "Gamma": 3}
	fmt.Println("3 keys (one group):")
	printOrders(small)

	// A bigger map is spread over a table, and every range starts at a random
	// group and slot. The orders differ almost every time.
	greek := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta", "Iota", "Kappa"}
	big := make(map[string]int, len(greek))
	for i, k := range greek {
		big[k] = i + 1
	}
	fmt.Println("10 keys (a table):")
	printOrders(big)

	// If you need an order, impose one. maps.Keys and slices.Sorted are the
	// Go 1.23 iterator helpers.
	fmt.Printf("  sorted: %s\n", strings.Join(slices.Sorted(maps.Keys(big)), " "))
}

func printOrders(m map[string]int) {
	for run := 1; run <= 5; run++ {
		var order []string
		for k := range m {
			order = append(order, k)
		}
		fmt.Printf("  run %d: %s\n", run, strings.Join(order, " "))
	}
}

func neverShrinks() {
	const n = 1_000_000

	m := make(map[int]int)
	for i := range n {
		m[i] = i
	}
	printHeap("after inserting 1M int->int", len(m))

	for i := range n {
		delete(m, i)
	}
	runtime.GC()
	printHeap("after deleting all + GC", len(m))

	clear(m)
	runtime.GC()
	printHeap("after clear(m) + GC", len(m))

	m = nil
	runtime.GC()
	printHeap("after m = nil + GC", len(m))
}

// printHeap reports HeapInuse. Passing len(m) after the GC keeps the map
// reachable until the measurement, so it cannot be collected early.
func printHeap(label string, entries int) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("%-29s len = %7d  HeapInuse = %5.1f MB\n", label, entries, float64(ms.HeapInuse)/(1<<20))
}

func setMemory() {
	const n = 1_000_000
	const width = 7 // "0000000" .. "0999999"

	// All keys are substrings of one 7 MB string, so building the key for an
	// insert allocates nothing and the measured delta is the map alone. Each
	// key is still a full 16-byte string header inside the map.
	var sb strings.Builder
	sb.Grow(n * width)
	for i := range n {
		fmt.Fprintf(&sb, "%0*d", width, i)
	}
	all := sb.String()
	key := func(i int) string { return all[i*width : (i+1)*width] }

	fmt.Printf("unsafe.Sizeof: bool = %d byte, struct{} = %d bytes\n",
		unsafe.Sizeof(true), unsafe.Sizeof(struct{}{}))

	perEntry("map[string]bool", n, func() any {
		m := make(map[string]bool)
		for i := range n {
			m[key(i)] = true
		}
		return m
	})
	perEntry("map[string]struct{}", n, func() any {
		m := make(map[string]struct{})
		for i := range n {
			m[key(i)] = struct{}{}
		}
		return m
	})
	perEntry("map[int64]struct{}", n, func() any {
		m := make(map[int64]struct{})
		for i := range n {
			m[int64(i)] = struct{}{}
		}
		return m
	})
}

// perEntry measures how much live heap build's result holds. The GC after
// building discards the smaller tables the map outgrew, so only the final
// map is counted.
func perEntry(label string, n int, build func() any) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	m := build()
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(m)

	delta := float64(after.HeapAlloc) - float64(before.HeapAlloc)
	fmt.Printf("%-20s %5.1f MB total, %4.1f bytes per entry\n", label, delta/(1<<20), delta/float64(n))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
