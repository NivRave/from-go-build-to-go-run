package main

// Only demo 3 has deterministic output: the values are sorted before
// printing. The other demos depend on goroutine scheduling and timing.

func Example_loopVariables() {
	loopVariables()
	// Output:
	// captured: [alpha beta delta gamma]
	// all distinct: true
}
