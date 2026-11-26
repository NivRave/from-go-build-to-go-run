// Command pointers demonstrates where data lives: which values the compiler
// keeps on the stack, which ones it moves to the heap, and how a goroutine
// stack grows instead of overflowing.
//
// Slides: slides/015-pointers.pdf
//
// Run:
//
//	go run ./examples/015-pointers
//	go build -gcflags=-m ./examples/015-pointers
//	go test -run='^$' -bench=. -benchmem ./examples/015-pointers
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

func main() {
	section("1. Returning a pointer vs a value (slide p04)")
	returnPointerVsValue()

	section("2. Passing a pointer down vs storing it (slides p04, p05)")
	passDownVsStore()

	section("3. Interface boxing (slide p05)")
	interfaceBoxing()

	section("4. Slices: stack or heap? (slide p08)")
	slices()

	section("5. The stack grows; it does not overflow (slide p02)")
	stackGrowth()
}

// runs is how many times each function is called per measurement.
const runs = 1000

// heapAllocs calls f runs times and returns how many heap allocations the
// loop made, from the Mallocs counter of runtime.MemStats. Only the loop is
// inside the measurement; nothing is printed until afterwards. The GC is
// paused while measuring, because a GC cycle makes a few allocations of its
// own that would blur the count. The loops allocate under 10 MB in total.
func heapAllocs(f func()) uint64 {
	f() // warm up, so one-time setup is not counted
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range runs {
		f()
	}
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs
}

func report(label string, f func()) {
	fmt.Printf("%-34s %4d heap allocations in %d calls\n", label, heapAllocs(f), runs)
}

// The //go:noinline directives keep each function a real call, so the
// escape decision is made for the function itself and not undone by
// inlining it into the caller (Part 016 shows that effect).

type User struct {
	Name  string
	Email string
	Age   int
}

// Package-level sinks keep results alive so the compiler cannot delete the
// calls. They are written and never read, which is the point.
//
//nolint:unused // intentional: write-only sinks
var (
	sinkInt  int
	global   *User
	sinkAny  any
	sinkInts []int
)

//go:noinline
func newUserPtr() *User {
	u := User{Name: "Ada", Age: 36}
	return &u // u must outlive the call: moved to heap
}

//go:noinline
func newUserVal() User {
	u := User{Name: "Ada", Age: 36}
	return u // copied into the caller's frame
}

func returnPointerVsValue() {
	report("newUserPtr() *User", func() { sinkInt = newUserPtr().Age })
	report("newUserVal() User", func() { sinkInt = newUserVal().Age })
}

//go:noinline
func process(u *User) int {
	return u.Age + len(u.Name) // reads through u, keeps nothing
}

//go:noinline
func keep(u *User) {
	global = u // u now outlives the call
}

func passDownVsStore() {
	report("process(&u): pointer passed down", func() {
		u := User{Name: "Ada", Age: 36}
		sinkInt = process(&u)
	})
	report("keep(&u): pointer stored globally", func() {
		u := User{Name: "Ada", Age: 36}
		keep(&u)
	})
}

// Report is a 64-byte struct. Converting it to an interface needs a copy
// that the interface can point to.
type Report struct {
	ID     int
	Values [7]int
}

//go:noinline
func logValue(v any) {
	sinkAny = v // like fmt.Println's arguments, v escapes
}

//go:noinline
func logReport(r Report) {
	sinkInt = r.ID + r.Values[0]
}

func interfaceBoxing() {
	r := Report{ID: 1}
	report("logReport(r Report)", func() { logReport(r) })
	report("logValue(r) taking any", func() { logValue(r) })
}

// n is a package variable, so the compiler cannot treat it as a constant.
var n = 1000

//go:noinline
func localSmall() int {
	s := make([]int, 8) // constant size, does not escape: stack
	s[0] = 1
	return s[0] + len(s)
}

// localLarge's slice does not escape, but its size is only known at run
// time. Since Go 1.25 the compiler reserves a small (32-byte) stack buffer
// for such a make and uses the heap only when the size does not fit, which
// 1000 ints (8 KB) does not. -m still reports "does not escape".
//
//go:noinline
func localLarge(size int) int {
	s := make([]int, size)
	s[0] = 1
	return s[0] + len(s)
}

//go:noinline
func returned() []int {
	return make([]int, 8) // outlives the call: heap
}

func slices() {
	report("make([]int, 8), local", func() { sinkInt = localSmall() })
	report("make([]int, n) n=1000, local", func() { sinkInt = localLarge(n) })
	report("make([]int, 8), returned", func() { sinkInts = returned() })
}

const depth = 100_000

var deepest runtime.MemStats

// recurse uses about 100 bytes of stack per call, so 100,000 calls need
// roughly 10 MB, far more than the few KB a goroutine starts with.
//
//go:noinline
func recurse(d int) int {
	var pad [8]int
	pad[d%8] = d
	if d == 0 {
		runtime.ReadMemStats(&deepest) // measured at the bottom
		return pad[0]
	}
	return recurse(d-1) + pad[d%8]
}

func stackGrowth() {
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	sinkInt = recurse(depth)

	fmt.Printf("StackInuse before:          %6.2f MB\n", float64(before.StackInuse)/(1<<20))
	fmt.Printf("StackInuse at depth %d: %6.2f MB\n", depth, float64(deepest.StackInuse)/(1<<20))
	fmt.Println("no stack overflow: the runtime copied the stack to a bigger one as it grew")
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
