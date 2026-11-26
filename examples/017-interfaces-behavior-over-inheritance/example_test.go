package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.

func Example_discoverInterface() {
	discoverInterface()
	// Output:
	// hello, gopher
	// memLogger captured ["hello, gopher"]
}

func Example_composition() {
	composition()
	// Output:
	// ReadWriter round trip: "composed"
	// countingWriter saw 24 bytes: "hello, composition! more"
}

func Example_anyVsGenerics() {
	anyVsGenerics()
	// Output:
	// Sum[int]:     6
	// Sum[float64]: 4.5
	// SumAny:       7.5 <nil>
	// SumAny:       0 element 2: unsupported type string
}
