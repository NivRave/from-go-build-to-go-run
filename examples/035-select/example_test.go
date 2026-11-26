package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// Section 1 (random split) and section 5 (timing) are not asserted.

func Example_loadShedding() {
	loadShedding()
	// Output:
	// burst of 20: 5 queued, 15 shed
	// worker processed 5 tasks after the burst
}

func Example_mergeNil() {
	mergeNil()
	// Output:
	// from ch1: [1 2 3]
	// from ch2: [10 20 30 40 50]
	// both channels are nil: loop finished
}

func Example_bridgeDemo() {
	bridgeDemo()
	// Output:
	// nested selects (slide):        producer finished before the consumer started: false got [1 2 3 4 5]
	// single select + nil send case: producer finished before the consumer started: true  got [1 2 3 4 5]
}
