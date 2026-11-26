package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// Demo 5 prints memory sizes, which vary, so it has no Example.

func Example_commaOK() {
	commaOK()
	// Output:
	// m["zero"]    = 0
	// m["missing"] = 0
	// zero    -> present, value 0
	// missing -> not present
}

func Example_sharedMaps() {
	sharedMaps()
	// Output:
	// after update:  map[done:1 start:0]
	// inside replace, before: len = 2
	// inside replace, after:  map[replaced:1]
	// after replace: map[done:1 start:0]
}

func Example_safeMapDemo() {
	safeMapDemo()
	// Output:
	// keys: 101
	// hits: 10000
}

func Example_keyRules() {
	keyRules()
	// Output:
	// struct key: treasure
	// array key:  listUsers
	// any key:    recovered: runtime error: hash of unhashable type []int
	// NaN keys:   len = 3, lookup found = false
	// after clear: len = 0
}
