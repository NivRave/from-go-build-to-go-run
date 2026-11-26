package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// Section 2 prints goroutine counts, which depend on the test binary, and is
// not asserted.

func Example_cancellationTree() {
	cancellationTree()
	// Output:
	// short's 50ms timeout fires:
	//   short/g1 stopped: context deadline exceeded
	//   short/g2 stopped: context deadline exceeded
	//   root.Err() = <nil>, manual.Err() = <nil> (a child ending does not reach its parent or siblings)
	// cancelRoot():
	//   manual/g1 stopped: context canceled
	//   manual/g2 stopped: context canceled
	//   manual.Err() = context canceled (cancellation flows downstream)
}

func Example_values() {
	values()
	// Output:
	// UserFrom(ctx)          = "alice", true
	// TraceIDFrom(ctx)       = "abc-123", true
	// UserFrom(Background()) = "", false
	// ctx.Value(otherKey(0)) = <nil> (same number, different type: no collision)
}

func Example_causes() {
	causes()
	// Output:
	// WithCancelCause:  Err() = context canceled; Cause = payment declined
	// WithTimeoutCause: Err() = context deadline exceeded; Cause = database took longer than 10ms
	// WithCancel:       Err() = context canceled; Cause = context canceled (no cause given: same as Err)
}

func Example_afterFuncAndWithoutCancel() {
	afterFuncAndWithoutCancel()
	// Output:
	// stop() before cancel: true (unregistered, will not run)
	// AfterFunc: request ended, releasing its DB connection
	// stop() after it ran:  false (already started)
	// request Err() = context canceled; audit Err() = <nil>; audit log for "alice" written, err = <nil>
}
