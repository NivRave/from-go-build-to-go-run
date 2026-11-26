package main

// The test binary initialises package main exactly as `go run` does, so by
// the time this Example runs, trace already holds the startup steps. Only
// main.main is missing: tests never call it.

func Example_printTrace() {
	printTrace()
	// Output:
	// 1. var name     = loadName()     (declared second, initialised first)
	// 2. var greeting = makeGreeting() (depends on name)
	// 3. init()                        (greeting = Hello, Gopher!)
}
