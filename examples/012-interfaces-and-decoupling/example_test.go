package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.

func Example_implicitSatisfaction() {
	implicitSatisfaction()
	// Output:
	// email to ops@example.com: disk 90% full
	// sms to +1-555-0100: disk 90% full
}

func Example_acceptInterfaces() {
	acceptInterfaces()
	// Output:
	// os.Stdout:     user id=1 name=Ada
	// bytes.Buffer:  holds "user id=1 name=Ada\n"
	// fake writer:   1 write, 19 bytes
}

func Example_returnStructs() {
	returnStructs()
	// Output:
	// Name() = notes.txt, Size() = 33
	// countBytes(f) = 33, err = <nil>
	// from io.Reader, after an assertion: other.txt
}

func Example_typeAssertions() {
	typeAssertions()
	// Output:
	// Integer: 42
	// String: hello
	// Other: float64
	// i.(int) with comma-ok: n = 0, ok = false
	// i.(int) without comma-ok: recovered: interface conversion: interface {} is string, not int
}

func Example_methodSetTrap() {
	methodSetTrap()
	// Output:
	// (&Square{5}).Area() via Shaper = 25
	// sq.Area() on an addressable value = 9
}
