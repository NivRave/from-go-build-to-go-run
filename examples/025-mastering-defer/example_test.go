package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// timingTrap is left out because its durations depend on the clock.

func Example_lifo() {
	lifo()
	loopLIFO()
	// Output:
	// Third
	// Second
	// First
	// loop defer 2
	// loop defer 1
	// loop defer 0
}

func Example_evalTrap() {
	evalTrap()
	// Output:
	// closure: 2
	// arg:     1
}

func Example_namedResults() {
	namedResults()
	// Output:
	// Save ok:       <nil>
	// Save failing:  save: disk full | errors.Is(err, errDiskFull): true
	// write ok, close ok:      <nil>
	// write ok, close fails:   close: device busy
	// both fail (joined):      "disk full\nclose: device busy"
}

func Example_deferInLoop() {
	deferInLoop()
	// Output:
	// defer in loop:   peak open files = 5 (err = <nil>)
	// helper per file: peak open files = 1 (err = <nil>)
}

func ExampleComplexLogic() {
	ComplexLogic()
	// Output:
	// entering: ComplexLogic
	//   ... actual work ...
	// leaving:  ComplexLogic
}
