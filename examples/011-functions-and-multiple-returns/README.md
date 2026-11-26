# Part 011: Functions: The power of multiple returns

Day 06 · Week 1 · How Go's multiple return values eliminate the "exception" mess.

## What this shows

- `Divide(a, b float64) (float64, error)` and the caller's `if err != nil`, plus proof that `_, _ = Divide(1, 0)` compiles: Go does not force error checks.
- Named results as documentation (`GetCoordinates() (lat, lng float64)`) and a naked vs explicit return in `ComplexLogic(cond bool)`, which returns `(10, fail)`.
- `CaptureError` turning a panic into an error through its named result, next to an unnamed-result version whose deferred function cannot change what is returned.
- `type Filter func(int) bool` with a working `ApplyFilter`, a generic `FilterSlice[T any]`, and `slices.DeleteFunc`.
- Closures: `AuthMiddleware`, a stateful counter factory, and the Go 1.22 per-iteration loop variable.

## Run

```sh
go run ./examples/011-functions-and-multiple-returns
go test ./examples/011-functions-and-multiple-returns
```

## What to look for

Expected output of `go run`:

```text
== 1. The value-error pair (slide p02) ==
Divide(10, 4) = 2.5
Divide(10, 0): error: cannot divide by zero
_, _ = Divide(1, 0) compiled and ran; the error was silently dropped

== 2. Named results and naked returns (slides p03, p04) ==
GetCoordinates() = (lat 32.0853, lng 34.7818)
ComplexLogic(true)  = (10, fail)
ComplexLogic(false) = (10, <nil>)

== 3. Named results and defer (slide p05) ==
named result:   recovered: boom
  deferred func set its local err to "recovered: boom"
unnamed result: <nil>

== 4. Functions as values (slide p06) ==
ApplyFilter(nums, isEven):  [2 4 6]
FilterSlice(words, isLong): [gopher build]
slices.DeleteFunc(isEven):  [1 3 5]

== 5. Closures (slide p07) ==
check("auth-123"): true
check("auth-999"): false
counter a: 1 2 3
counter b: 1
loop closures: 0 1 2
```

- The naked `return` in `ComplexLogic(true)` returns the current values of the named results, `(10, fail)`, exactly as the slide says.
- Both versions in section 3 recover the panic. Only the named result lets the deferred function change what the caller receives; the unnamed version returns `nil` even though its local `err` was set.
- The two counters do not share state: each call to `NewCounter` creates a new `n` captured by its closure.
- `loop closures: 0 1 2` relies on Go 1.22 semantics (this module declares `go 1.26.0`). With `go 1.21` in `go.mod` the same loop prints `2 2 2`, because all iterations shared one `i`.

## Notes and corrections

- **Slide p02:** the compiler does not force the caller to handle the error. `_, _ = Divide(1, 0)` compiles, and so does calling a function that returns only an `error` without using the result. Only unused variables are compile errors; linters such as `errcheck` enforce error checks.
- **Slide p04:** a naked return with a shadowed named result is already a compile error. Go 1.26 reports `result parameter result not in scope at return` (and the same for `err`), pointing at the inner declaration. The real cost of naked returns in long functions is readability. The snippet's `condition` is undefined; this example passes it in as `cond`.
- **Slide p06:** `ApplyFilter`'s body is elided on the slide, so it does not compile as shown. Generics (Go 1.18) allow one `FilterSlice[T any]` for every element type, and `slices.DeleteFunc` (Go 1.21) covers the common case. The generic function is not called `Filter`, because a package cannot declare both a type and a function with the same name.
- **Slide p07:** the closure example is correct. Since Go 1.22, loop variables are per iteration, so closures created in a loop no longer all see the last value.

## Slides

[011-functions-and-multiple-returns.pdf](../../slides/011-functions-and-multiple-returns.pdf)
