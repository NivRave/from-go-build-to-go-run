package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.

func Example_valueErrorPair() {
	valueErrorPair()
	// Output:
	// Divide(10, 4) = 2.5
	// Divide(10, 0): error: cannot divide by zero
	// _, _ = Divide(1, 0) compiled and ran; the error was silently dropped
}

func Example_namedResults() {
	namedResults()
	// Output:
	// GetCoordinates() = (lat 32.0853, lng 34.7818)
	// ComplexLogic(true)  = (10, fail)
	// ComplexLogic(false) = (10, <nil>)
}

func Example_deferAndNamedResults() {
	deferAndNamedResults()
	// Output:
	// named result:   recovered: boom
	//   deferred func set its local err to "recovered: boom"
	// unnamed result: <nil>
}

func Example_functionValues() {
	functionValues()
	// Output:
	// ApplyFilter(nums, isEven):  [2 4 6]
	// FilterSlice(words, isLong): [gopher build]
	// slices.DeleteFunc(isEven):  [1 3 5]
}

func Example_closures() {
	closures()
	// Output:
	// check("auth-123"): true
	// check("auth-999"): false
	// counter a: 1 2 3
	// counter b: 1
	// loop closures: 0 1 2
}
