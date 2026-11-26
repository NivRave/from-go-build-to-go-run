package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// Section 5 (map order) is random by design, so only its sorted half is
// asserted.

func Example_forForms() {
	forForms()
	// Output:
	// C-style:          [0 1 2]
	// while-style:      [1 2 4 8]
	// infinite + break: stopped after 3 tries
	// range over int:   [0 1 2]
	// range over func:  [3 2 1]
}

func Example_rangeString() {
	rangeString()
	// Output:
	// 0:'h' 1:'é' 3:'l' 4:'l' 5:'o' 6:',' 7:' ' 8:'世' 11:'界'
	// len (bytes):             14
	// utf8.RuneCountInString:  9
}

func Example_copyTrap() {
	copyTrap()
	// Output:
	// value loop: [{ID:1 Active:false} {ID:2 Active:false}]
	// index loop: [{ID:1 Active:true} {ID:2 Active:true}]
}

func Example_loopVariables() {
	loopVariables()
	// Output:
	// values: 1 2 3
	// all distinct: true
}

func Example_sortedKeys() {
	sortedKeys()
	// Output: sorted:   Alpha:1 Beta:2 Delta:4 Epsilon:5 Gamma:3
}

func Example_filterInPlace() {
	filterInPlace()
	// Output:
	// manual filter:      [2 4 6] backing array: [2 4 6 4 5 6]
	// slices.DeleteFunc:  [2 4 6] backing array: [2 4 6 0 0 0]
}

func Example_rangeChannel() {
	rangeChannel()
	// Output:
	// Processing: build
	// Processing: test
	// Processing: deploy
	// after close: v="" ok=false
}
