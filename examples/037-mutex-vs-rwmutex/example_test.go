package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// The racy and deadlocking demos are never called from tests.

func Example_mutexCounter() {
	mutexCounter()
	// Output: 1000 goroutines x Inc() = 1000
}

func Example_noReentrancy() {
	noReentrancy()
	// Output: Add(5) then Increment() = 6
}

func Example_sessionCache() {
	sessionCache()
	// Output: 8 readers x 1000 Get = 8000 hits; after 10 Sets, session-11 = "bob"
}
