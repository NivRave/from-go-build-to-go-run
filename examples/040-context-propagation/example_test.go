package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// Sections 3 and 4 depend on real timing across HTTP hops and are not
// asserted; propagation_test.go covers the same logic deterministically.

func Example_transportDemo() {
	transportDemo()
	// Output:
	// sent:     X-Trace-ID=abc-123, X-Timeout-Ms=~500ms
	// original: 0 headers (the caller's request was not modified)
}

func Example_middlewareDemo() {
	middlewareDemo()
	// Output:
	// handler sees trace_id=abc-123, budget ~700ms
	// handler sees trace_id=<new, 26 chars>, budget none
}

func Example_slogDemo() {
	slogDemo()
	// Output:
	// level=INFO msg="payment captured" service=payments amount=42 trace_id=abc-123
	// level=INFO msg="cache warmed" service=payments
}
