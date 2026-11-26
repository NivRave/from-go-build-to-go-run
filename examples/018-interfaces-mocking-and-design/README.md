# Part 018: Interfaces - based mocking & design

Day 09 · Week 2 · Decoupling for testability.

## What this shows

- A consumer-defined `DataStore` interface (`SaveUser`, `GetUser`, `DeleteUser`) and a `UserService` that depends only on it. `main` wires it to an in-memory store and registers two users.
- `Register` validates input and wraps store errors with `%w`, so a validation error is a value the caller can inspect with `errors.Is`.
- In `service_test.go`: a hand-written `MockStore` that records calls, a `FailStore` returning the sentinel `ErrConnLost`, and table-driven tests asserting with `errors.Is`.
- A partial mock that embeds `DataStore` and overrides one method, plus a test proving that a non-overridden method panics with a nil pointer dereference.
- Pure validation logic tested directly, without any mock.
- `RetryRegister`, which sleeps 100 ms between attempts, tested inside `synctest.Test` with exact virtual durations and no `Clock` interface.

## Run

```sh
go run ./examples/018-interfaces-mocking-and-design
go test ./examples/018-interfaces-mocking-and-design
go test -v -run 'TestPartialMock|TestRetryRegister' ./examples/018-interfaces-mocking-and-design
```

## What to look for

Expected output of `go run`:

```text
== 1. UserService wired to an in-memory store (slides p02-p03) ==
register u1: err = <nil>
register u2: err = <nil>
users stored: 2
lookup u2: Linus <linus@example.com>, err = <nil>

== 2. Validation errors are values the caller can inspect (slide p05) ==
register u3: invalid user: email "grace.example.com" has no @
errors.Is(err, ErrInvalidUser): true

The interesting part is in service_test.go: go test -v ./examples/018-interfaces-mocking-and-design
```

- `main` is the only code that names `MemStore`. `UserService` sees a `DataStore`, so the tests can pass `MockStore`, `FailStore` or a partial mock instead.
- In `go test -v`, `TestPartialMockPanicsOnMissingMethod` logs `calling a non-overridden method panics: runtime error: invalid memory address or nil pointer dereference`. Embedding an interface satisfies it at compile time, but the embedded value is nil.
- `TestRetryRegister/succeeds_on_third_try` asserts a virtual elapsed time of exactly 200 ms, yet the whole package's tests finish in well under a second of real time. Inside a `synctest` bubble, `time.Sleep` advances a fake clock.
- `TestRetryRegisterStopsWhenContextIsCancelled` uses `context.WithTimeout` inside the bubble. The deadline also runs on the fake clock.

## Notes and corrections

- **Slide p07:** embedding `io.Reader` and overriding `Read` doesn't show anything, because `io.Reader` has only that one method. The example embeds the three-method `DataStore` instead. Any method you don't override belongs to the nil embedded interface and panics when called.
- **Slide p06:** since Go 1.25, `testing/synctest` gives time-dependent code a fake clock, so `time.Sleep`, timers and `context.WithTimeout` can be tested deterministically without a `Clock` interface. For randomness, inject a seeded generator from `math/rand/v2`, for example `rand.New(rand.NewPCG(1, 2))`.
- **Slide p09:** `github.com/golang/mock` has been archived since 2023. The maintained fork is `go.uber.org/mock`.
- **Slide p05:** the slide doesn't show the assertion. Compare against a sentinel with `errors.Is`, not against the error string, so wrapping with context doesn't break the test.
- **Slide p04:** `MockStore` as written is not safe for concurrent use. That's fine while the code under test calls it from one goroutine.

## Slides

[018-interfaces-mocking-and-design.pdf](../../slides/018-interfaces-mocking-and-design.pdf)
