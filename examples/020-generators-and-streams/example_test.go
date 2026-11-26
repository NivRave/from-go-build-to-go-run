package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.

func Example_closurePipeline() {
	closurePipeline()
	// Output: 20 40 60 80 100
}

func Example_iterPipeline() {
	iterPipeline()
	// Output: 20 40 60 80 100
}

func Example_earlyExit() {
	earlyExit()
	// Output: consumed up to 3, producer generated 3 values
}

func Example_streamErrors() {
	streamErrors()
	// Output:
	// default buffer: 1 lines, err = bufio.Scanner: token too long
	// 1 MiB buffer:   3 lines, err = <nil>
}
