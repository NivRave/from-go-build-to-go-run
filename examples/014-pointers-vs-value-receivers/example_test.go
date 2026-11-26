package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// The -race-demo code is deliberately never called from a test.

func Example_receivers() {
	receivers()
	// Output:
	// Inside method (copy): 11
	// Original caller: 10
	// Inside method (pointer): 11
	// Original caller: 11
}

func Example_methodSets() {
	methodSets()
	// Output:
	// u.UpdateName on a variable: Grace
	// rename(&u, ...) via Namer:  Linus
}

func Example_shallowCopy() {
	shallowCopy()
	// Output:
	// inside Shout:  MIX [INTRO outro]
	// caller after:  mix [INTRO outro]
}

func Example_nilReceivers() {
	nilReceivers()
	// Output:
	// (*User)(nil).GetName() = Guest
	// findUser("") == nil:      false (dynamic type *main.User, GetName "Guest")
	// findUserFixed("") == nil: true
}
