// Command functions-and-multiple-returns demonstrates Go's (value, error)
// returns, named results, deferred functions that change a result, function
// types, and closures.
//
// Slides: slides/011-functions-and-multiple-returns.pdf
//
// Run:
//
//	go run ./examples/011-functions-and-multiple-returns
package main

import (
	"errors"
	"fmt"
	"slices"
)

func main() {
	section("1. The value-error pair (slide p02)")
	valueErrorPair()

	section("2. Named results and naked returns (slides p03, p04)")
	namedResults()

	section("3. Named results and defer (slide p05)")
	deferAndNamedResults()

	section("4. Functions as values (slide p06)")
	functionValues()

	section("5. Closures (slide p07)")
	closures()
}

// Divide is the slide p02 function: the error is the last result.
func Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("cannot divide by zero")
	}
	return a / b, nil
}

func valueErrorPair() {
	for _, b := range []float64{4, 0} {
		q, err := Divide(10, b)
		if err != nil {
			fmt.Printf("Divide(10, %g): error: %v\n", b, err)
			continue
		}
		fmt.Printf("Divide(10, %g) = %g\n", b, q)
	}

	// This compiles. The language never forces you to look at an error; it
	// only rejects variables you declare and never use. Linters such as
	// errcheck are what catch ignored errors.
	_, _ = Divide(1, 0)
	fmt.Println("_, _ = Divide(1, 0) compiled and ran; the error was silently dropped")
}

// GetCoordinates names its results so the signature documents which float
// is which.
func GetCoordinates() (lat, lng float64) {
	return 32.0853, 34.7818
}

// ComplexLogic is the slide p04 function, with the undefined `condition`
// turned into a parameter. Both paths return result = 10.
//
// The "shadowing bug" a naked return could cause is already a compile error.
// Declaring `result, err := 20, errors.New("fail")` inside the if and then
// writing a bare `return` gives:
//
//	result parameter result not in scope at return
//	result parameter err not in scope at return
//
// The remaining reason to avoid naked returns is readability.
func ComplexLogic(cond bool) (result int, err error) {
	result = 10
	if cond {
		err = errors.New("fail")
		return // naked: returns the current (10, fail)
	}
	return result, nil // explicit: the reader sees what comes back
}

func namedResults() {
	lat, lng := GetCoordinates()
	fmt.Printf("GetCoordinates() = (lat %.4f, lng %.4f)\n", lat, lng)

	r, err := ComplexLogic(true)
	fmt.Printf("ComplexLogic(true)  = (%d, %v)\n", r, err)
	r, err = ComplexLogic(false)
	fmt.Printf("ComplexLogic(false) = (%d, %v)\n", r, err)
}

// CaptureError turns a panic into an error. The deferred function runs after
// the panic and can assign to err because err is a named result.
func CaptureError() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	panic("boom")
}

// captureErrorUnnamed tries the same thing without naming the result. The
// deferred function recovers, but it can only change a local variable; the
// function returns the zero value of its result, nil.
func captureErrorUnnamed() error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
			fmt.Printf("  deferred func set its local err to %q\n", err)
		}
	}()
	panic("boom")
}

func deferAndNamedResults() {
	fmt.Println("named result:  ", CaptureError())
	fmt.Println("unnamed result:", captureErrorUnnamed())
}

// Filter is the slide p06 function type.
type Filter func(int) bool

// ApplyFilter returns the numbers f keeps, in order. The slide elides the body.
func ApplyFilter(nums []int, f Filter) []int {
	var out []int
	for _, n := range nums {
		if f(n) {
			out = append(out, n)
		}
	}
	return out
}

// FilterSlice is the generic version. It is called FilterSlice rather than
// Filter because one package cannot declare both a type and a function
// named Filter.
func FilterSlice[T any](s []T, keep func(T) bool) []T {
	var out []T
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func functionValues() {
	nums := []int{1, 2, 3, 4, 5, 6}
	isEven := func(n int) bool { return n%2 == 0 }

	words := []string{"go", "gopher", "run", "build"}
	isLong := func(s string) bool { return len(s) > 3 }

	fmt.Println("ApplyFilter(nums, isEven): ", ApplyFilter(nums, isEven))
	fmt.Println("FilterSlice(words, isLong):", FilterSlice(words, isLong))

	// The standard library alternative (Go 1.21). DeleteFunc works in place,
	// so it gets a copy here.
	odd := slices.DeleteFunc(slices.Clone(nums), isEven)
	fmt.Println("slices.DeleteFunc(isEven): ", odd)
}

// AuthMiddleware is the slide p07 closure: the returned function keeps
// secretToken alive for as long as the function itself is reachable.
func AuthMiddleware(token string) func(string) bool {
	secretToken := "auth-" + token
	return func(input string) bool {
		return input == secretToken
	}
}

// NewCounter is a stateful factory: each counter has its own n.
func NewCounter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

func closures() {
	check := AuthMiddleware("123")
	fmt.Println(`check("auth-123"):`, check("auth-123"))
	fmt.Println(`check("auth-999"):`, check("auth-999"))

	a, b := NewCounter(), NewCounter()
	fmt.Println("counter a:", a(), a(), a())
	fmt.Println("counter b:", b())

	// Since Go 1.22 every loop iteration has its own i, so each closure sees
	// a different value. Before Go 1.22 all three shared one variable and this
	// printed 2 2 2.
	var funcs []func() int
	for i := range 3 {
		funcs = append(funcs, func() int { return i })
	}
	fmt.Print("loop closures:")
	for _, f := range funcs {
		fmt.Print(" ", f())
	}
	fmt.Println()
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
