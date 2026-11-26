package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// The CAS retry count, the CAS-flag race window and the lost-update count
// vary from run to run and are not asserted.

func Example_atomicCounter() {
	atomicCounter()
	// Output: 1000 goroutines x Inc() = 1000
}

func Example_onceValueFix() {
	onceValueFix()
	// Output: sync.OnceValue: init ran 1 time; 10 of 10 callers got the finished config
}
