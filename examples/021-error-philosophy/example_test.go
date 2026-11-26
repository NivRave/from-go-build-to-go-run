package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's "expected output" can never go stale.

func Example_sentinelsAndTypes() {
	sentinelsAndTypes()
	// Output:
	// err: query "SELECT * FROM users WHERE id = 42" failed: not found
	// errors.Is(err, ErrNotFound): true
	// failed query: SELECT * FROM users WHERE id = 42
}

func Example_behaviourChecks() {
	behaviourChecks()
	// Output:
	// wrapped: fetch quotes: rate limited, retry after 2s
	//                        direct   wrapped
	// type assertion:        true     false
	// errors.As (interface): true     true
	// IsRetryable(ErrNotFound): false
}

func Example_guardClauses() {
	guardClauses()
	// Output:
	// empty email:    invalid email
	// empty password: invalid password
	// db failure:     database save: connection refused
	// success:        <nil>
	// users saved:    1
}

func Example_typedNil() {
	typedNil()
	// Output:
	// bad:   err != nil is true (dynamic type *main.QueryError, dynamic value nil)
	// fixed: err != nil is false
}

func Example_panicVsError() {
	panicVsError()
	// Output:
	// boot aborted by panic: boot: DATABASE_URL is required
	// boot ok, dsn = postgres://localhost/app
	// request for user 7: returned an error (not found: true), still serving
	// request for user 8: returned an error (not found: true), still serving
}
