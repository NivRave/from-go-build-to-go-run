//go:build vetdemo

package main

// This file is excluded from normal builds. It holds the slide p04 code as
// written, which go vet rejects (the "defers" analyzer, Go 1.23+):
//
//	go vet -tags vetdemo ./examples/025-mastering-defer
//
// time.Since(start) is an argument of the deferred call, so it is evaluated
// at the defer statement, not when main returns. The printed duration is
// the near-zero time between the two lines, not the second of sleep.

import (
	"fmt"
	"time"
)

func slideDurationTrap() {
	start := time.Now()
	// This captures the value of time.Since(start) NOW.
	defer fmt.Printf("Duration: %v\n", time.Since(start))

	time.Sleep(1 * time.Second)
}
