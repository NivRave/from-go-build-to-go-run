// Command arrays-vs-slices demonstrates why Go code reaches for slices: arrays
// are values that copy on every assignment and call, while slices are small
// headers that share a backing array and grow on demand.
//
// Slides: slides/007-arrays-vs-slices.pdf
//
// Run:
//
//	go run ./examples/007-arrays-vs-slices
//	go test -run='^$' -bench=. -benchmem ./examples/007-arrays-vs-slices
package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"unsafe"
)

func main() {
	section("1. Arrays are values (slides p02-p03)")
	arrayValues()
	arraySizes()

	section("2. Slices share a backing array (slide p06)")
	sharedBacking()

	section("3. Growth trace: append to a nil slice (slide p07)")
	growthTrace()

	section("4. nil vs empty vs make (slides p04, p05, p08)")
	nilVsEmpty()
}

// ---- 1. Arrays are values ----

// mutateArray receives its own copy of all three elements.
func mutateArray(arr [3]int) {
	arr[0] = 100
}

// mutateSlice receives a copy of the slice header, which still points at the
// caller's backing array.
func mutateSlice(s []int) {
	s[0] = 100
}

func arrayValues() {
	a := [3]int{1, 2, 3}
	b := a    // This creates a FULL COPY of the data
	b[0] = 99 // changes only the copy
	fmt.Println("after b := a; b[0] = 99:  a =", a, " b =", b)

	mutateArray(a)
	fmt.Println("after mutateArray(a):     a =", a)

	s := []int{1, 2, 3}
	mutateSlice(s)
	fmt.Println("after mutateSlice(s):     s =", s)

	// The size is part of the type, so [5]int and [10]int are unrelated:
	//
	//	var arr [5]int
	//	printSome(arr) // compile error: cannot use arr (variable of type [5]int) as [10]int value in argument to printSome
}

// arraySizes is not asserted by a test: the figures are for 64-bit
// platforms and halve on 32-bit ones.
func arraySizes() {
	fmt.Println("unsafe.Sizeof([1000]int{}) =", unsafe.Sizeof([1000]int{}), "bytes (copied on every assignment)")
	fmt.Println("unsafe.Sizeof([]int{})     =", unsafe.Sizeof([]int{}), "bytes (pointer, len, cap)")
}

// ---- 2. Shared backing array ----

func sharedBacking() {
	original := []int{1, 2, 3, 4}
	sub := original[1:3] // [2 3], a window onto original's array
	sub[0] = 99          // Changes 'original' too!
	fmt.Println("sub[0] = 99:          original =", original, " sub =", sub)

	// slices.Clone (Go 1.21) copies the elements into a new backing array.
	// It is a shallow copy: pointers inside the elements are shared.
	original = []int{1, 2, 3, 4}
	independent := slices.Clone(original[1:3])
	independent[0] = 99
	fmt.Println("after slices.Clone:   original =", original, " independent =", independent)
}

// ---- 3. Growth trace ----

// growthTrace appends one element at a time and prints each reallocation.
// The exact capacities are an implementation detail (they depend on the Go
// version and on allocator size classes), so no test asserts them.
func growthTrace() {
	var s []int
	prevCap := cap(s)
	for i := range 2001 {
		s = append(s, i)
		c := cap(s)
		switch {
		case c == prevCap:
			continue
		case prevCap == 0:
			fmt.Printf("len %4d: cap %4d -> %4d (first allocation)\n", len(s), prevCap, c)
		default:
			fmt.Printf("len %4d: cap %4d -> %4d (x%.2f)\n", len(s), prevCap, c, float64(c)/float64(prevCap))
		}
		prevCap = c
	}
}

// ---- 4. nil vs empty vs make ----

// Payload shows how encoding/json treats each kind of slice, and what the
// omitzero tag option (Go 1.24) does: it omits a field holding its zero
// value, which for a slice means nil, but keeps an empty, non-nil slice.
type Payload struct {
	Nil           []int `json:"nil"`
	Empty         []int `json:"empty"`
	OmitZeroNil   []int `json:"omitzero_nil,omitzero"`
	OmitZeroEmpty []int `json:"omitzero_empty,omitzero"`
}

func nilVsEmpty() {
	var nilSlice []int             // no backing array at all
	emptySlice := []int{}          // non-nil, length 0, no heap allocation
	madeSlice := make([]int, 0, 5) // pre-allocated room for 5
	madeLen := make([]int, 5, 10)  // slide p05: 5 zeroed elements, room for 10

	for _, c := range []struct {
		name string
		s    []int
	}{
		{"var s []int", nilSlice},
		{"[]int{}", emptySlice},
		{"make([]int, 0, 5)", madeSlice},
		{"make([]int, 5, 10)", madeLen},
	} {
		js, err := json.Marshal(c.s)
		if err != nil {
			fmt.Println("marshal:", err)
			return
		}
		fmt.Printf("%-19s == nil: %-5t len %d cap %2d  %-11s JSON %s\n",
			c.name, c.s == nil, len(c.s), cap(c.s), fmt.Sprint(c.s), js)
	}

	js, err := json.Marshal(Payload{Empty: []int{}, OmitZeroEmpty: []int{}})
	if err != nil {
		fmt.Println("marshal:", err)
		return
	}
	fmt.Printf("Payload: %s\n", js)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
