package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// The other sections print timings or per-worker counts that vary per run;
// pool_test.go checks their logic instead.

func Example_fanInDemo() {
	fanInDemo()
	// Output: 4 worker channels merged into one: sum 10100 (want 10100)
}
