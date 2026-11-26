// Command signals-and-cleanup demonstrates the mechanics of a clean exit: the
// run() pattern, waiting for workers with a timeout, idempotent teardown with
// sync.OnceValue, releasing signal handlers, and a two-stage Ctrl+C.
//
// Slides: slides/052-signals-and-cleanup.pdf
//
// Run:
//
//	go run ./examples/052-signals-and-cleanup
//	go run ./examples/052-signals-and-cleanup -wait
package main

import (
	"context"
	"fmt"
	"os"
)

// main is the only place that chooses the exit code (slide p08). Everything
// with a defer lives in run, because os.Exit skips deferred calls.
func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("main: exited cleanly. Goodbye!")
	// No os.Exit(0) here: returning from main already exits with status 0.
}
