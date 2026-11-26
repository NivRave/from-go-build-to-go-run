# Part 003: Variables, constants, the := controversy

Day 02 · Week 1 · When to use short declarations.

## What this shows

- The three declaration forms from slides p02-p03 (`const`, `var` with a zero value, `:=`), each printed with the type the compiler gave it.
- Untyped constants adapt to their use: the same `const big = 1 << 40` becomes an `int64` and a `float64`, and a constant too large for any integer type still compiles while it stays a constant.
- The slide p04 grouped package-level `var ( databaseURL; timeout; retries )` block, printed with its zero values.
- Shadowing exactly as on slide p07, then the realistic version of the bug: `err` (and the result) shadowed inside an `if` block, so a valid value is ignored and an error is silently lost. Plus the fix.
- Static typing: `tmp := 6` is an `int` forever, and the slide's `tmp = 3.14` is left in a comment with the real compiler message.

## Run

```sh
go run ./examples/003-variables-constants
go test ./examples/003-variables-constants
```

## What to look for

Expected output of `go run`:

```text
== 1. The Go trinity: const, var, := (slides p02-p03) ==
const maxRetries = 10         int      10
var name string               string   ""
pi := 3.14                    float64  3.14
user := "niv_rave"            string   "niv_rave"
count, active := 10, true     int      10
  (and active)                bool     true

== 2. Untyped constants adapt to their use (slide p05) ==
int64:   int64 1099511627776
float64: float64 1.099511627776e+12
huge >> 98 = 4

== 3. Grouped package-level var block (slide p04) ==
databaseURL = ""
timeout     = 0s
retries     = 0

== 4. Shadowing: the silent bug (slide p07) ==
6  Inside block
10 Outside block
  (inside the if, timeout = 5s)
buggy("5s") = 30s, <nil>
buggy("five") = 30s, <nil>
fixed("5s") = 5s, <nil>
fixed("five") = 0s, timeout "five": time: invalid duration "five"

== 5. Inference is not dynamic (slide p06) ==
tmp := 6  -> int 6
tmp = 7   -> int 7
```

- An untyped integer constant like `maxRetries` takes its default type, `int`, only when it has to (here, when passed as `any`).
- `huge` is `1 << 100`, which no Go integer type can hold, yet `huge >> 98` compiles and prints 4: constant arithmetic is exact, and only the final value has to fit.
- `parseTimeoutBuggy("5s")` parses 5s inside the `if` and then returns the default 30s, and `parseTimeoutBuggy("five")` returns no error at all. Both outer variables were shadowed by `:=`. The fix declares nothing new inside the block and assigns with `=`.
- Every block of output is asserted by an `Example` in `example_test.go`.

## Notes and corrections

- **Slide p06:** the comment quotes `3.5`, but the code assigns `3.14`. The real message is `cannot use 3.14 (untyped float constant) as int value in assignment (truncated)`.
- **Slide p04:** an unused local `var` is a compile error, so the slide's `Connect()` fails once the `...` is removed: `declared and not used: currentAttemp`. (`currentAttemp` is also missing a `t`.)
- **Slide p07:** `go vet` doesn't report shadowing by default. The optional `shadow` analyzer does: `go run golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest ./...`.
- **Slide p05:** only boolean, numeric and string kinds can be constants. Untyped constants have arbitrary precision until they are used, and then take the type of their context (`const pi = 3.14` works as a `float32` or a `float64`; the standard library also provides `math.Pi`).

## Slides

[003-variables-constants.pdf](../../slides/003-variables-constants.pdf)
