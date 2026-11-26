// Command iteration demonstrates Go's single loop keyword: the three for
// forms, range over ints, functions, strings, slices, maps and channels, and
// the traps that come with each.
//
// Slides: slides/019-iteration.pdf
//
// Run:
//
//	go run ./examples/019-iteration
package main

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

func main() {
	section("1. The faces of for (slides p02-p03)")
	forForms()

	section("2. Ranging over a string: byte offsets and runes (slide p03)")
	rangeString()

	section("3. The copy trap (slide p04)")
	copyTrap()

	section("4. Loop variables are per iteration since Go 1.22 (slide p05)")
	loopVariables()

	section("5. Map order is randomised (slide p06)")
	mapOrder()
	sortedKeys()

	section("6. In-place filter, slices.DeleteFunc, and range over a channel (slides p07-p08)")
	filterInPlace()
	rangeChannel()
}

// ---- 1. for forms ----------------------------------------------------------

// Countdown is a custom iterator (Go 1.23): any func(yield func(T) bool) can
// be ranged over. It stops early if the loop body breaks.
func Countdown(from int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := from; i > 0; i-- {
			if !yield(i) {
				return
			}
		}
	}
}

func forForms() {
	var got []int
	for i := 0; i < 3; i++ { //nolint:intrange // intentional: shows the classic three-clause form
		got = append(got, i)
	}
	fmt.Println("C-style:         ", got)

	got = nil
	n := 1
	for n < 10 { // the "while" form
		got = append(got, n)
		n *= 2
	}
	fmt.Println("while-style:     ", got)

	tries := 0
	for {
		tries++
		if tries == 3 {
			break
		}
	}
	fmt.Println("infinite + break: stopped after", tries, "tries")

	got = nil
	for i := range 3 { // Go 1.22
		got = append(got, i)
	}
	fmt.Println("range over int:  ", got)

	got = nil
	for v := range Countdown(3) { // Go 1.23
		got = append(got, v)
	}
	fmt.Println("range over func: ", got)
}

// ---- 2. strings -------------------------------------------------------------

func rangeString() {
	s := "héllo, 世界"
	// range over a string decodes UTF-8: i is the byte offset where each
	// rune starts, so it jumps by 2 after 'é' and by 3 after each CJK rune.
	// Invalid UTF-8 would yield U+FFFD (the replacement character).
	var pairs []string
	for i, r := range s {
		pairs = append(pairs, fmt.Sprintf("%d:%q", i, r))
	}
	fmt.Println(strings.Join(pairs, " "))
	fmt.Println("len (bytes):            ", len(s))
	fmt.Println("utf8.RuneCountInString: ", utf8.RuneCountInString(s))
}

// ---- 3. copy trap -------------------------------------------------------------

// User is the struct from slide p04. The slide's composite literal is missing
// its trailing commas, which is a syntax error in Go:
//
//	users := []User{
//		{
//			ID: 1,
//			Active: false
//		}
//	}
//	// compile error: syntax error: unexpected newline in composite literal; possibly missing comma or }
type User struct {
	ID     int
	Active bool
}

func copyTrap() {
	users := []User{
		{ID: 1, Active: false},
		{ID: 2, Active: false},
	}

	for _, u := range users {
		u.Active = true // u is a copy of the element; the slice is untouched
		_ = u
	}
	fmt.Printf("value loop: %+v\n", users)

	for i := range users {
		users[i].Active = true // index into the slice: modifies the element
	}
	fmt.Printf("index loop: %+v\n", users)
}

// ---- 4. loop variables ----------------------------------------------------------

func loopVariables() {
	// The slide's `for i, v := range` doesn't compile because i is unused:
	//
	//	compile error: declared and not used: i
	var out []*int
	for _, v := range []int{1, 2, 3} {
		out = append(out, &v) // each iteration has its own v (Go 1.22+)
	}
	fmt.Println("values:", *out[0], *out[1], *out[2])
	fmt.Println("all distinct:", out[0] != out[1] && out[1] != out[2] && out[0] != out[2])
	// The semantics come from the `go` line in go.mod, not from the
	// toolchain: the same code in a module that says `go 1.21` prints
	// "values: 3 3 3" even when built with Go 1.26.
}

// ---- 5. map order ---------------------------------------------------------------

var greek = map[string]int{"Alpha": 1, "Beta": 2, "Gamma": 3, "Delta": 4, "Epsilon": 5}

// mapOrder ranges over the same map three times. The order is unspecified
// and deliberately randomised, so the lines may differ from each other and
// from run to run. (fmt.Println(m) would print sorted keys, which can hide
// this.)
func mapOrder() {
	for run := 1; run <= 3; run++ {
		var keys []string
		for k := range greek {
			keys = append(keys, k)
		}
		fmt.Printf("range #%d: %s\n", run, strings.Join(keys, " "))
	}
}

func sortedKeys() {
	// Go 1.23: maps.Keys returns an iterator, slices.Sorted collects it.
	var pairs []string
	for _, k := range slices.Sorted(maps.Keys(greek)) {
		pairs = append(pairs, fmt.Sprintf("%s:%d", k, greek[k]))
	}
	fmt.Println("sorted:  ", strings.Join(pairs, " "))
}

// ---- 6. filtering and channels ------------------------------------------------------

func isEven(x int) bool { return x%2 == 0 }

func filterInPlace() {
	// The manual in-place filter from slide p07: no allocation, but the tail
	// of the backing array still holds the old values. For pointers that
	// keeps objects alive.
	nums := []int{1, 2, 3, 4, 5, 6}
	n := 0
	for _, x := range nums {
		if isEven(x) {
			nums[n] = x
			n++
		}
	}
	nums = nums[:n]
	fmt.Println("manual filter:     ", nums, "backing array:", nums[:cap(nums)])

	// slices.DeleteFunc does the same in place and, since Go 1.22, zeroes the
	// tail so the garbage collector can reclaim what it pointed to.
	nums2 := []int{1, 2, 3, 4, 5, 6}
	nums2 = slices.DeleteFunc(nums2, func(x int) bool { return !isEven(x) })
	fmt.Println("slices.DeleteFunc: ", nums2, "backing array:", nums2[:cap(nums2)])
}

func rangeChannel() {
	queue := make(chan string, 3)
	go func() {
		defer close(queue) // without close, the range below never ends
		for _, msg := range []string{"build", "test", "deploy"} {
			queue <- msg
		}
	}()

	for msg := range queue { // exits when the channel is closed and drained
		fmt.Println("Processing:", msg)
	}

	// The manual form is `v, ok := <-ch`; ok is false once closed and empty.
	v, ok := <-queue
	fmt.Printf("after close: v=%q ok=%v\n", v, ok)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
