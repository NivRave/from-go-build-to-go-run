// Command slice-performance-and-memory-safety demonstrates destructive
// in-place filtering, the append "ghost" bug and the full slice expression
// that prevents it, the backing-array memory leak, and nil vs empty slices
// in JSON.
//
// Slides: slides/008-slice-performance-and-memory-safety.pdf
//
// Run:
//
//	go run ./examples/008-slice-performance-and-memory-safety
//	go test -run='^$' -bench=. -benchmem ./examples/008-slice-performance-and-memory-safety
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
)

func main() {
	section("1. In-place filtering clobbers its input (slides p02-p03)")
	filtering()

	section("2. The append ghost bug (slide p04)")
	appendGhost()

	section("3. The defensive fix: 3-index slicing (slide p05)")
	threeIndex()

	section("4. The massive backing-array leak (slide p06)")
	backingArrayLeak()

	section("5. nil vs empty: the API contract (slide p07)")
	nilVsEmptyJSON()
}

// ---- 1. Filtering ----

func filtering() {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := FilterInPlace(nums)
	fmt.Println("FilterInPlace:       result", evens, " input now", nums)

	// slices.DeleteFunc is the standard-library equivalent. It also reuses
	// the input, and since Go 1.22 it zeroes the abandoned tail.
	nums = []int{1, 2, 3, 4, 5, 6}
	evens = slices.DeleteFunc(nums, isOdd)
	fmt.Println("slices.DeleteFunc:   result", evens, " input now", nums)

	// When the original must stay intact, pay for one copy first.
	original := []int{1, 2, 3, 4, 5, 6}
	evens = FilterInPlace(slices.Clone(original))
	fmt.Println("Clone, then filter:  result", evens, " input now", original)
}

// ---- 2. Append ghost ----

func appendGhost() {
	parent := []int{1, 2, 3, 4}
	child := parent[:2] // len 2, cap 4: room to grow into parent's elements
	child = append(child, 99)
	fmt.Println("child := parent[:2]:   parent", parent, " child", child, "(the 3 is gone)")

	parent = []int{1, 2, 3, 4}
	child = parent[:2:2] // len 2, cap 2: append must reallocate
	child = append(child, 99)
	fmt.Println("child := parent[:2:2]: parent", parent, " child", child)
}

// ---- 3. Three-index slicing ----

// threeIndex is slide p05 with mt.Printf fixed to fmt.Printf and the s2
// comments corrected. A full slice expression a[low:high:max] sets
// cap = max - low.
func threeIndex() {
	array := [6]int{0, 1, 2, 3, 4, 5} // Original array: {0, 1, 2, 3, 4, 5}, cap: 6
	s1 := array[1:4]                  // Standard 2-index slice, len(s1) = 3, cap(s1) = 5
	s2 := array[1:4:4]                // 3-index slice, len(s2) = 3, cap(s2) = 3
	fmt.Printf("s1: len %d cap %d; s2: len %d cap %d\n", len(s1), cap(s1), len(s2), cap(s2))

	// Appending to s1
	s1 = append(s1, 10, 11) // Modifies the original array memory as it has capacity
	fmt.Printf("s1 after append to s1: %v\n", s1)
	fmt.Printf("s2 after append to s1: %v\n", s2)
	fmt.Printf("array after s1 append: %v\n", array) // array elements 4 and 5 were overwritten

	// Appending to s2
	s2 = append(s2, 20) // Forces new allocation because capacity is only 3
	fmt.Printf("s1 after append to s2: %v\n", s1)
	fmt.Printf("s2 after append to s2: %v\n", s2)
	fmt.Printf("array after s2 append: %v\n", array) // Original array remains unchanged by s2's append
}

// ---- 4. Backing-array leak ----

// kept is package-level so that whatever it points at stays reachable, like
// a cache entry or a field of a long-lived struct.
var kept []byte

// readHugeData stands in for a large payload. The slides use 1 GB; 64 MB
// keeps the demo within the repository's memory budget.
func readHugeData() []byte {
	return make([]byte, 64<<20)
}

// getSmallDataFromHugeData is slide p06: the returned 10-byte slice keeps
// the whole backing array alive.
func getSmallDataFromHugeData(hugeData []byte) []byte {
	return hugeData[:10] // The 64 MB backing array stays in memory!
}

// getSmallDataCopy copies the 10 bytes into their own array. bytes.Clone
// (Go 1.20) replaces the slide's append([]byte{}, hugeData[:10]...).
func getSmallDataCopy(hugeData []byte) []byte {
	return bytes.Clone(hugeData[:10])
}

func backingArrayLeak() {
	report := func(label string) {
		fmt.Printf("%-24s HeapInuse = %3d MB, len(kept) = %d\n", label, heapInuseMB(), len(kept))
	}
	report("baseline:")

	kept = getSmallDataFromHugeData(readHugeData())
	report("kept hugeData[:10]:")

	kept = getSmallDataCopy(readHugeData())
	report("kept bytes.Clone(...):")

	kept = nil // release the demo's memory
}

// heapInuseMB forces a full collection and reports the heap in use, in MB.
func heapInuseMB() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse >> 20
}

// ---- 5. JSON ----

// Response is a typical API payload whose frontend calls items.map(...).
type Response struct {
	Items []string `json:"items"`
}

func nilVsEmptyJSON() {
	var s1 []int  // JSON: null
	s2 := []int{} // JSON: []
	for _, v := range []any{s1, s2, Response{}, Response{Items: []string{}}} {
		js, err := json.Marshal(v)
		if err != nil {
			fmt.Println("marshal:", err)
			return
		}
		fmt.Printf("%-34s -> %s\n", fmt.Sprintf("%#v", v), js)
	}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
