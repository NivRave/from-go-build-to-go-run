package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// These use real (short) timeouts with wide margins; sections 3 and 4 print
// memory sizes and durations and are not asserted.

func Example_timeoutNoLeak() {
	timeoutNoLeak()
	// Output:
	// buffered (cap 1): API call timed out; goroutines leaked: 0
	// unbuffered:       API call timed out; goroutines leaked: 1
}

func Example_contextTimeout() {
	contextTimeout()
	// Output:
	// local limit fires:
	//   fetchUser:          context deadline exceeded
	//   ctx.Err():          context deadline exceeded
	//   context.Cause(ctx): local operation exceeded its 50ms limit
	// caller cancels first:
	//   fetchUser:          context canceled
	//   ctx.Err():          context canceled
	//   context.Cause(ctx): user cancelled the request
}

func Example_gracefulShutdownDemo() {
	gracefulShutdownDemo()
	// Output:
	// -- healthy service --
	// Shutting down... draining tasks
	// Tasks finished cleanly
	// Resources closed cleanly
	// result: <nil>
	// -- stuck service --
	// Shutting down... draining tasks
	// Task drain timed out. Forcing resource cleanup...
	// Resource cleanup timed out.
	// result: resource cleanup timed out (a real main would now call os.Exit(1))
}
