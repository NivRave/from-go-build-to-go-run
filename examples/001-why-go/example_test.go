package main

// Only the concurrency demo is asserted: the platform and build-info demos
// print values that differ per machine and per build.

func Example_concurrentSum() {
	concurrentSum()
	// Output: 4 workers squared [1 2 3 4 5 6 7 8]; sum of squares = 204
}
