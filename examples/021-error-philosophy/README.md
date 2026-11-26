# Part 021: Error philosophy

Day 11 · Week 2 · Why Go doesn't have try/catch, and why that's a feature.

## What this shows

- A sentinel `ErrNotFound` (made with `errors.New`) and a custom `*QueryError{Query, Err}` with `Error()` and `Unwrap()`, so `errors.Is(err, ErrNotFound)` sees through it and `errors.As` extracts the query.
- Opaque errors: a `retryable` interface with `Retryable() bool`. `IsRetryable` uses `errors.As` with an interface target and works through a `fmt.Errorf("...: %w")` wrapper; the slide's bare type assertion does not.
- Guard clauses: the slide's `RegisterUser` with a fake in-memory `db.Save`, run with an empty email, an empty password, a database failure and a valid user.
- The typed-nil gotcha: returning a nil `*QueryError` as `error` makes `err != nil` true.
- Panic versus error: a missing required setting panics during boot (recovered here only so the demo can continue), while a missing user during request handling is a returned error.

## Run

```sh
go run ./examples/021-error-philosophy
go test ./examples/021-error-philosophy
```

## What to look for

Expected output of `go run`:

```text
== 1. Sentinel errors and custom error types (slides p03-p04) ==
err: query "SELECT * FROM users WHERE id = 42" failed: not found
errors.Is(err, ErrNotFound): true
failed query: SELECT * FROM users WHERE id = 42

== 2. Opaque errors: check behaviour, not type (slide p05) ==
wrapped: fetch quotes: rate limited, retry after 2s
                       direct   wrapped
type assertion:        true     false
errors.As (interface): true     true
IsRetryable(ErrNotFound): false

== 3. Guard clauses keep the happy path left (slide p08) ==
empty email:    invalid email
empty password: invalid password
db failure:     database save: connection refused
success:        <nil>
users saved:    1

== 4. The typed-nil gotcha (slide p03) ==
bad:   err != nil is true (dynamic type *main.QueryError, dynamic value nil)
fixed: err != nil is false

== 5. Panic is not an error (slide p07) ==
boot aborted by panic: boot: DATABASE_URL is required
boot ok, dsn = postgres://localhost/app
request for user 7: returned an error (not found: true), still serving
request for user 8: returned an error (not found: true), still serving
```

- `errors.Is` returns true only because `QueryError` has `Unwrap`. Delete that method and section 1 prints `false`.
- In section 2, the type assertion is right about the bare error and wrong about the wrapped one. Wrapping is what every layer should do (Part 022), so a check that can't see through it is a latent bug.
- `RateLimitError` is exported here, but callers never need to name it. They depend only on the behaviour, which is the point of an opaque error.
- The "bad" line in section 4 is an interface holding the type `*QueryError` and a nil pointer. `staticcheck` flags the comparison (`SA4023: this comparison is always true`), so the example silences that check on purpose.

## Notes and corrections

- **Slide p05:** `err.(temporary)` is a plain type assertion and doesn't see through wrapping. Use `errors.As(err, &te)` with an interface-typed target. `net.Error.Temporary` has been deprecated since Go 1.18 because "temporary" is ill-defined, so prefer `Timeout()` or a domain behaviour such as `Retryable()`.
- **Slide p06:** `uber-go/zap` is a logger, not an error library. It can attach a stack to a log entry, but it doesn't capture one when an error is created. `github.com/pkg/errors` has been archived since 2021. A nil-pointer dereference is a panic, and panics do print a full stack trace; the missing-stack problem applies to `error` values only.
- **Slide p03:** `QueryError` needs `func (e *QueryError) Unwrap() error { return e.Err }`, otherwise `errors.Is` and `errors.As` can't reach the cause.
- **Slide p04:** sentinels are normally created with `errors.New`. `fmt.Errorf` with nothing to format works but is unusual.

## Slides

[021-error-philosophy.pdf](../../slides/021-error-philosophy.pdf)
