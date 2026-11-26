package main

import (
	"os"
	"path/filepath"
)

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.

func Example_wrapAndIs() {
	dir, err := os.MkdirTemp("", "022-example-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	wrapAndIs(filepath.Join(dir, "config.json"))
	// Output:
	// err starts with "load config: open ": true
	// err == fs.ErrNotExist:          false
	// errors.Is(err, fs.ErrNotExist): true
	// using defaults: {Port:8080 LogLevel:info}
}

func Example_joinErrors() {
	joinErrors()
	// Output:
	// errors.Join message (one line per error):
	// redis: close: broken pipe
	// file: close: disk full
	// errors.Is(errs, ErrRedisClose): true
	// errors.Is(errs, ErrFileClose):  true
	// multiple %w: shutdown: redis: close: broken pipe; file: close: disk full
	// errors.Is finds both: true
}

func Example_sanitize() {
	sanitize()
	// Output:
	// internal (logs only): db update failed for user 7: dial tcp 10.0.0.5:5432: connect: connection refused
	// public (to client):   {"error":"Update failed. Please try again.","request_id":"req-0001"}
	// internal (logs only): find order 99: order not found
	// public (to client):   {"error":"Order not found.","request_id":"req-0002"}
}
