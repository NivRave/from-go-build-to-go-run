// Command maps demonstrates everyday map behaviour: comma-ok lookups, maps
// shared between functions, a correct mutex-guarded map, the rules for keys,
// and why deleting entries does not give memory back.
//
// Slides: slides/009-maps.pdf
//
// Run:
//
//	go run ./examples/009-maps
//	go run ./examples/009-maps -crash
//	go test -run='^$' -bench=. -benchmem ./examples/009-maps
package main

import (
	"flag"
	"fmt"
	"math"
	"runtime"
)

func main() {
	crash := flag.Bool("crash", false, "run unsynchronised concurrent map writes (ends in a fatal error)")
	flag.Parse()

	if *crash {
		concurrentWritesCrash()
		return
	}

	section(`1. "Comma ok": zero vs missing (slide p04)`)
	commaOK()

	section("2. Maps are shared with the functions you pass them to (slide p05)")
	sharedMaps()

	section("3. A correct SafeMap (slide p06)")
	safeMapDemo()

	section("4. Key rules (slide p07)")
	keyRules()

	section("5. Deleting does not shrink a map (slide p08)")
	retention()
}

func commaOK() {
	m := map[string]int{"zero": 0}

	// A plain lookup cannot tell the two apart: both print 0.
	fmt.Println(`m["zero"]    =`, m["zero"])
	fmt.Println(`m["missing"] =`, m["missing"])

	// The second result says whether the key is actually present.
	for _, k := range []string{"zero", "missing"} {
		if v, ok := m[k]; ok {
			fmt.Printf("%-7s -> present, value %d\n", k, v)
		} else {
			fmt.Printf("%-7s -> not present\n", k)
		}
	}
}

// update writes through the map it was given. A map value is a pointer to
// the runtime's map structure, so the caller sees the new key.
func update(m map[string]int) {
	m["done"] = 1
}

// replace points its own copy of the parameter at a brand-new map. The caller
// still holds the old pointer, so nothing written afterwards is visible to it.
func replace(m map[string]int) {
	fmt.Println("inside replace, before: len =", len(m))
	m = make(map[string]int)
	m["replaced"] = 1
	fmt.Println("inside replace, after: ", m)
}

func sharedMaps() {
	myMap := map[string]int{"start": 0}

	update(myMap)
	fmt.Println("after update: ", myMap)

	replace(myMap)
	fmt.Println("after replace:", myMap)
}

func safeMapDemo() {
	s := NewSafeMap()
	hammer(s, 100, 100)
	fmt.Println("keys:", s.Len())
	fmt.Println("hits:", s.Get("hits"))
}

// Point and Route are comparable, so they can be map keys.
type Point struct{ X, Y int }

type Route [2]string // method, path

func keyRules() {
	// Struct keys compare field by field: a new Point{1, 2} finds the entry.
	grid := map[Point]string{{1, 2}: "treasure"}
	fmt.Println("struct key:", grid[Point{1, 2}])

	// Array keys compare element by element. Slices could not be used here.
	routes := map[Route]string{{"GET", "/users"}: "listUsers"}
	fmt.Println("array key: ", routes[Route{"GET", "/users"}])

	// An interface key type compiles, but hashing a dynamic value that is not
	// comparable panics at run time.
	fmt.Println("any key:   ", anyKeyPanic())

	// NaN != NaN, so every insert creates a new entry that no lookup can find.
	nan := math.NaN()
	m := map[float64]int{}
	m[nan] = 1
	m[nan] = 2
	m[nan] = 3
	_, found := m[nan]
	fmt.Printf("NaN keys:   len = %d, lookup found = %t\n", len(m), found)

	// clear (Go 1.21) is the only way to remove entries you cannot look up.
	clear(m)
	fmt.Printf("after clear: len = %d\n", len(m))
}

func anyKeyPanic() (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint("recovered: ", r)
		}
	}()
	m := map[any]int{}
	m[[]int{1, 2}] = 1
	return "no panic"
}

func retention() {
	const n = 1_000_000

	m := make(map[int]int)
	for i := range n {
		m[i] = i
	}
	printHeap("after inserting 1M entries", len(m))

	for i := range n {
		delete(m, i)
	}
	runtime.GC()
	printHeap("after deleting all + GC", len(m))

	// clear empties the map but keeps its tables, exactly like delete.
	clear(m)
	runtime.GC()
	printHeap("after clear(m) + GC", len(m))

	// Dropping the last reference is what lets the GC free the tables.
	m = nil
	runtime.GC()
	printHeap("after m = nil + GC", len(m))
}

// printHeap prints the live heap. Passing len(m) after runtime.GC keeps the
// map reachable until the measurement is taken, so the GC cannot collect it
// early just because the variable is no longer used.
func printHeap(label string, entries int) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("%-28s len = %7d  HeapInuse = %5.1f MB\n", label, entries, float64(ms.HeapInuse)/(1<<20))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
