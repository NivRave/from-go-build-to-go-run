// Command escape-analysis demonstrates how the compiler decides whether a
// value escapes to the heap: inlining, allocation size, interface boxing,
// and what a pointer is used for.
//
// Slides: slides/016-escape-analysis.pdf
//
// Run:
//
//	go run ./examples/016-escape-analysis
//	go build -gcflags=-m -o bin/ ./examples/016-escape-analysis
//	go build -gcflags=-m=2 -o bin/ ./examples/016-escape-analysis
//	go test -bench=. -benchmem ./examples/016-escape-analysis
package main

import (
	"fmt"
	"testing"
)

func main() {
	section("1. Inlining decides escapes (slides p03, p07, p08)")
	inlining()

	section("2. The size rule (slide p04)")
	sizeRule()

	section("3. Interface boxing (slides p05, p06)")
	boxing()

	section("4. What the pointer is used for (slide p03)")
	pointerUse()
}

// allocsPerCall uses testing.AllocsPerRun, the same tool the tests use. It
// works outside tests too: it runs f, reads runtime.MemStats around the
// loop, and returns the average number of heap allocations per call.
func allocsPerCall(f func()) float64 {
	return testing.AllocsPerRun(20, f)
}

func report(label string, f func()) {
	fmt.Printf("%-44s %v allocs/call\n", label, allocsPerCall(f))
}

// Package-level sinks keep results alive so the compiler cannot delete the
// calls. They are written and never read, which is the point.
//
//nolint:unused // intentional: write-only sinks
var (
	sinkInt  int
	sinkCfg  *Config
	sinkAny  any
	global   *User
	sinkInts []int
)

// --- 1. Inlining ------------------------------------------------------------

// HeapAlloc is the slide p07 function. Its body says x escapes, but it is
// small enough to inline, and once inlined into a caller that only reads
// the result, nothing outlives the caller's frame.
func HeapAlloc() *int {
	x := 42
	return &x
}

type Config struct {
	Timeout int
	Retries int
}

// GetConfigPointer is the slide p08 function: inlinable, so in practice
// 0 allocs, not the slide's 1.
func GetConfigPointer() *Config {
	return &Config{Timeout: 30, Retries: 3}
}

// GetConfigPointerNoInline is the same function, kept out of line. Now the
// Config must outlive the call, so it is allocated on the heap.
//
//go:noinline
func GetConfigPointerNoInline() *Config {
	return &Config{Timeout: 30, Retries: 3}
}

func GetConfigValue() Config {
	return Config{Timeout: 30, Retries: 3}
}

func inlining() {
	report("HeapAlloc() *int, inlined", func() { sinkInt = *HeapAlloc() })
	report("GetConfigPointer(), inlined", func() { sinkInt = GetConfigPointer().Timeout })
	report("GetConfigPointerNoInline(), //go:noinline", func() { sinkInt = GetConfigPointerNoInline().Timeout })
	report("GetConfigValue() Config", func() { sinkInt = GetConfigValue().Timeout })

	// Inlining only helps if the caller keeps the pointer local. Storing it
	// in a package variable makes even the inlined version allocate.
	report("GetConfigPointer(), result stored globally", func() { sinkCfg = GetConfigPointer() })
}

// --- 2. Size ----------------------------------------------------------------

// small and big are package variables, so the compiler cannot treat them as
// constants.
var small, big = 4, 100

//go:noinline
func constSmall() int {
	s := make([]int, 8) // 64 bytes, constant size: stack
	return len(s) + s[0]
}

//go:noinline
func constHuge() int {
	s := make([]int, 1_000_000) // 8 MB: over the 64 KB limit for make, heap
	return len(s) + s[0]
}

//go:noinline
func variable(n int) int {
	s := make([]int, n) // Go 1.25+: 32-byte stack buffer, heap if n*8 > 32
	return len(s) + s[0]
}

func sizeRule() {
	report("make([]int, 8)", func() { sinkInt = constSmall() })
	report("make([]int, 1_000_000)", func() { sinkInt = constHuge() })
	report("make([]int, n), n = 4 at run time", func() { sinkInt = variable(small) })
	report("make([]int, n), n = 100 at run time", func() { sinkInt = variable(big) })
}

// --- 3. Boxing --------------------------------------------------------------

// BigStruct is too large to fit in an interface's data word, so storing one
// in an interface needs a heap copy.
type BigStruct struct {
	A, B, C, D int
}

var bigStruct = BigStruct{1, 2, 3, 4}

// Package variables, so the values are only known at run time.
var smallValue, largeValue = 7, 1000

func boxing() {
	report("sinkAny = bigStruct", func() { sinkAny = bigStruct })
	report("sinkAny = 42 (constant)", func() { sinkAny = 42 })
	report("sinkAny = smallValue (7 at run time)", func() { sinkAny = smallValue })
	report("sinkAny = largeValue (1000 at run time)", func() { sinkAny = largeValue })

	// The slide p05/p06 snippet: -m says "42 escapes to heap", but the
	// constant is boxed from read-only data, so nothing is allocated.
	var x = 42
	fmt.Println("fmt.Println(x) prints:", x)
}

// --- 4. Pointer use ---------------------------------------------------------

type User struct {
	Name string
	Age  int
}

// Process only reads through u, so &u does not escape at the call site.
//
//go:noinline
func Process(u *User) int {
	return u.Age
}

//go:noinline
func store(u *User) {
	global = u
}

func pointerUse() {
	report("Process(&u)", func() {
		u := User{Name: "Ada", Age: 36}
		sinkInt = Process(&u)
	})
	report("store(&u)", func() {
		u := User{Name: "Ada", Age: 36}
		store(&u)
	})
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
