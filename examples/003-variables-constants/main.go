// Command variables-constants demonstrates Go's three declaration forms
// (const, var, :=), how untyped constants adapt to their use, and the
// shadowing bug that := makes easy to write.
//
// Slides: slides/003-variables-constants.pdf
//
// Run:
//
//	go run ./examples/003-variables-constants
package main

import (
	"fmt"
	"time"
)

// A grouped package-level var block, as on slide p04. := is not allowed at
// package level, and these have no value yet, so var is the right tool: each
// starts at its type's zero value.
var (
	databaseURL string
	timeout     time.Duration
	retries     int
)

func main() {
	section("1. The Go trinity: const, var, := (slides p02-p03)")
	trinity()

	section("2. Untyped constants adapt to their use (slide p05)")
	untypedConstants()

	section("3. Grouped package-level var block (slide p04)")
	packageVars()

	section("4. Shadowing: the silent bug (slide p07)")
	shadowing()
	shadowedErr()

	section("5. Inference is not dynamic (slide p06)")
	staticTyping()
}

// show prints a declaration, the type the compiler gave it, and its value.
// %#v makes the empty string visible as "".
func show(decl string, v any) {
	fmt.Printf("%-29s %-8T %#v\n", decl, v, v)
}

func trinity() {
	const maxRetries = 10 // compile-time constant; untyped, so it defaults to int
	var name string       // no value yet: starts at the zero value ""
	pi := 3.14            // short declaration: the type is inferred once, as float64

	user := "niv_rave"
	count, active := 10, true

	show("const maxRetries = 10", maxRetries)
	show("var name string", name)
	show("pi := 3.14", pi)
	show(`user := "niv_rave"`, user)
	show("count, active := 10, true", count)
	show("  (and active)", active)
}

func untypedConstants() {
	// big has no type yet. It takes one at each use, as long as the value
	// fits, so the same constant becomes an int64 and a float64 here.
	const big = 1 << 40
	var asInt int64 = big
	var asFloat float64 = big
	fmt.Printf("int64:   %T %v\n", asInt, asInt)
	fmt.Printf("float64: %T %v\n", asFloat, asFloat)

	// Untyped constants have arbitrary precision until they are used. huge
	// doesn't fit any Go integer type, but huge>>98 does, so this compiles.
	const huge = 1 << 100
	fmt.Println("huge >> 98 =", huge>>98)
}

func packageVars() {
	fmt.Printf("databaseURL = %q\n", databaseURL)
	fmt.Printf("timeout     = %v\n", timeout)
	fmt.Printf("retries     = %d\n", retries)

	// Inside a function, an unused local var doesn't compile. The slide's
	// Connect() would fail as soon as the "..." is removed:
	//
	//	var currentAttemp = 1 // compile error: declared and not used: currentAttemp
}

// shadowing is slide p07 verbatim. The two spaces in "6  Inside block" come
// from Println adding a space before an argument that already starts with one.
func shadowing() {
	x := 10
	if true {
		x := 6                          // This is a NEW x, only inside this block!
		fmt.Println(x, " Inside block") // Prints 6
	}
	fmt.Println(x, "Outside block") // Prints 10
}

const defaultTimeout = 30 * time.Second

// parseTimeoutBuggy is the realistic version of the same bug. Inside the if,
// := declares a NEW timeout and a NEW err, so the outer ones are never
// assigned: a valid value is ignored and an invalid one is silently dropped.
func parseTimeoutBuggy(raw string) (time.Duration, error) {
	timeout := defaultTimeout
	var err error
	if raw != "" {
		timeout, err := time.ParseDuration(raw) // BUG: shadows both outer variables
		if err == nil {
			fmt.Printf("  (inside the if, timeout = %v)\n", timeout)
		}
	}
	return timeout, err
}

// parseTimeout is the fix: declare nothing new inside the block, assign with =.
func parseTimeout(raw string) (time.Duration, error) {
	timeout := defaultTimeout
	if raw != "" {
		var err error
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return 0, fmt.Errorf("timeout %q: %w", raw, err)
		}
	}
	return timeout, nil
}

func shadowedErr() {
	for _, raw := range []string{"5s", "five"} {
		d, err := parseTimeoutBuggy(raw)
		fmt.Printf("buggy(%q) = %v, %v\n", raw, d, err)
	}
	for _, raw := range []string{"5s", "five"} {
		d, err := parseTimeout(raw)
		fmt.Printf("fixed(%q) = %v, %v\n", raw, d, err)
	}
}

func staticTyping() {
	tmp := 6 // Translates to var tmp int
	fmt.Printf("tmp := 6  -> %T %v\n", tmp, tmp)

	// tmp = 3.14 // compile error: cannot use 3.14 (untyped float constant) as int value in assignment (truncated)

	tmp = 7 // any int is fine: the type was fixed once, at the declaration
	fmt.Printf("tmp = 7   -> %T %v\n", tmp, tmp)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
