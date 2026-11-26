package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.
// The layout demo's sizes depend on the architecture, so its Example lives
// in example_layout_test.go, which only builds on 64-bit platforms.

func Example_zeroValues() {
	zeroValues()
	// Output:
	// zero Config: Paths = [], nil = true
	// after AddPath: [/etc/app /usr/local/etc/app]
	// bytes.Buffer works from its zero value too
}

func Example_embedding() {
	embedding()
	// Output:
	// promoted field:  a.Name = Ada
	// promoted method: a.Greet() = Hello, Ada
	// as a Greeter:    Hello, Ada
	// a.User:          user #1 Ada
}

func Example_structTags() {
	structTags()
	// Output:
	// {"id":1,"created":"0001-01-01T00:00:00Z"}
	// field ID: json="id" db="user_id"
}

func Example_encapsulation() {
	encapsulation()
	// Output:
	// rejected: deposit must be positive, got -500
	// ada's balance: 1050 cents ($10.50)
	// float64: 0.1 + 0.2 = 0.30000000000000004; cents: 10 + 20 = 30
	// NewDatabase(""): url required
	// NewDatabase(url): url = postgres://localhost/app, err = <nil>
}
