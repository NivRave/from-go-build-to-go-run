# Part 025: Mastering defer

Day 13 · Week 2 · Defer: LIFO and execution timing.

## What this shows

- LIFO: three deferred calls run in reverse order, and so do the defers pushed by `for i := range 3 { defer ... }`.
- The evaluation trap: `defer fmt.Println("arg:", x)` captures `x` immediately, while a deferred closure reads it at exit. The timing version shows `time.Since` evaluated at the `defer` statement (about zero) versus inside a closure (about 50 ms).
- Decorating a named result: `Save` wraps its error in a deferred function without also logging it, and `writeReport` folds a deferred `Close` error into the result with `errors.Join`.
- Defer in a loop: five real temporary files opened with `defer` inside the loop (peak open = 5) versus a helper function per file (peak open = 1).
- Entry/exit tracing with one line: `defer un(trace("ComplexLogic"))`.
- With `-exit`: `os.Exit` ends the process while a deferred call is pending, and that call never runs.

## Run

```sh
go run ./examples/025-mastering-defer
go run ./examples/025-mastering-defer -exit
go test ./examples/025-mastering-defer
go test -run='^$' -bench=. -benchmem ./examples/025-mastering-defer
go vet -tags vetdemo ./examples/025-mastering-defer
```

`-exit` is safe to run: it prints one line and exits with status 0. It is opt-in only because it ends the program before anything after it could run.

## What to look for

Expected output of `go run` (the two durations in section 2 vary slightly):

```text
== 1. LIFO: last deferred, first run (slide p02) ==
Third
Second
First
loop defer 2
loop defer 1
loop defer 0

== 2. Arguments are evaluated at the defer statement (slide p04) ==
closure: 2
arg:     1
argument, evaluated at defer: 0s
closure, evaluated at exit:  50ms

== 3. Decorating named results (slide p05) ==
Save ok:       <nil>
Save failing:  save: disk full | errors.Is(err, errDiskFull): true
write ok, close ok:      <nil>
write ok, close fails:   close: device busy
both fail (joined):      "disk full\nclose: device busy"

== 4. Defer in a loop (slide p07) ==
defer in loop:   peak open files = 5 (err = <nil>)
helper per file: peak open files = 1 (err = <nil>)

== 5. Entry/exit tracing with one defer (slide p08) ==
entering: ComplexLogic
  ... actual work ...
leaving:  ComplexLogic
```

With `-exit`, the output ends with:

```text
== 6. os.Exit skips deferred calls (slide p03) ==
calling os.Exit(0) with one deferred call pending
```

The deferred line `deferred: this line is never printed` is indeed never printed.

The slide's timing code as written fails `go vet` (it lives in `vetdemo.go`, built only with `-tags vetdemo`):

```text
$ go vet -tags vetdemo ./examples/025-mastering-defer
examples\025-mastering-defer\vetdemo.go:22:37: call to time.Since is not deferred
```

Benchmark shape (numbers vary by machine):

```text
BenchmarkDirect         ~2.4 ns/op   0 B/op   0 allocs/op
BenchmarkDefer          ~2.8 ns/op   0 B/op   0 allocs/op
BenchmarkDeferInLoop   ~17.8 ns/op   0 B/op   0 allocs/op
```

- In section 2, `closure: 2` prints before `arg: 1`: the closure was deferred last, so it runs first, and it sees the final value of `x`.
- The argument version shows `0s` because `time.Since` ran right after `time.Now`, and the demo rounds to the millisecond. Unrounded, the value is tiny and its exact form depends on the platform's clock.
- `writeReport` keeps both errors. With a plain `defer r.Close()`, the close error would be dropped silently.
- An open-coded defer costs a fraction of a nanosecond more than a direct call. The same defer inside a one-iteration loop is several times slower, because it needs a runtime defer record. That cost only matters in hot paths.

## Notes and corrections

- **Slide p04:** the slide's output `Duration: 0s` depends on the platform's clock resolution. With a fine-grained clock it prints a small non-zero value such as `120ns`. `go vet` (Go 1.23 and later) reports the pattern as `call to time.Since is not deferred`. The fix is `defer func() { fmt.Printf("Duration: %v\n", time.Since(start)) }()`.
- **Slide p03:** deferred calls also run when a function reaches the end of its body, when it panics (which is how `recover` works) and on `runtime.Goexit`. They don't run on `os.Exit` or `log.Fatal`.
- **Slide p05:** the deferred block both logs the error and returns it, which breaks Part 022's "handle or return" rule: the caller will log it again. The example only decorates.
- **Slide p06:** open-coded defers (Go 1.14) don't apply to defers inside loops, and a function may have at most 8 of them. Otherwise the runtime falls back to a defer record, which is still on the stack in most cases (Go 1.13).
- **Slide p07:** this is a file-descriptor leak more than a memory leak. The first snippet also ignores the `os.Open` error, and the fixed version silently skips files that fail to open. The example's `readOne` returns the error, and the loop collects all of them with `errors.Join`.

## Slides

[025-mastering-defer.pdf](../../slides/025-mastering-defer.pdf)
