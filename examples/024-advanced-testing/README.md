# Part 024: Advanced testing

Day 12 · Week 2 · Benchmarking, fuzzing, and the O(1) mindset.

## What this shows

- `JoinNaive` (`+=`) versus `JoinBuilder` (`strings.Builder` with `Grow`), benchmarked in both the old style (`b.N` and `b.ResetTimer`) and the Go 1.24 style (`b.Loop()`).
- `Parse` for `key=value`, attacked by `FuzzParse` with a seed corpus and three properties: no panic, every error wraps `ErrSyntax`, and parse-render-parse round-trips.
- `Greeting`, a consumer of a `Client` interface, tested with a function-field `MockClient`.
- `RenderReport`, whose output is compared against `testdata/report.golden`, with a custom `-update` flag to rewrite it.
- An integration-style test that reads a real file and skips itself under `go test -short`.

## Run

```sh
go run ./examples/024-advanced-testing
go test ./examples/024-advanced-testing
go test -run='^$' -bench=. -benchmem ./examples/024-advanced-testing
go test -fuzz=FuzzParse -fuzztime=10s ./examples/024-advanced-testing
go test ./examples/024-advanced-testing -update
go test -short -v ./examples/024-advanced-testing
go test -run='^$' -bench=. -cpuprofile cpu.out ./examples/024-advanced-testing && go tool pprof -http=:8080 cpu.out
```

The profiling command writes `cpu.out` and the test binary (`024-advanced-testing.test`, or `.test.exe` on Windows) to the current directory. Both are git-ignored; delete them when you're done.

## What to look for

Expected output of `go run`:

```text
== 1. JoinNaive vs JoinBuilder: same result, different cost (slides p02-p03) ==
JoinNaive:   "go test -bench ."
JoinBuilder: "go test -bench ."
equal: true

== 2. Parse: the function FuzzParse attacks (slide p04) ==
"port=8080"    -> Key="port" Value="8080"
" key = a=b "  -> Key="key" Value="a=b"
"novalue"      -> error: syntax error: missing '=' in "novalue"
"=x"           -> error: syntax error: empty key in "=x"
"bad key=1"    -> error: syntax error: whitespace in key "bad key"

== 3. Greeting depends on a Client interface (slide p06) ==
Hello, gopher!
Hello, stranger!

== 4. RenderReport: output checked by a golden file (slide p07) ==
Service configuration
=====================
KEY        VALUE                                     LENGTH
name       checkout                                  8
port       8080                                      4
dsn        postgres://app@db/orders?sslmode=disable  40
log_level  info                                      4
4 settings
```

Benchmark shape for 200 parts of 8 bytes (time varies by machine; bytes and allocations don't):

```text
BenchmarkJoinNaiveOld      ~381,000 B/op   398 allocs/op
BenchmarkJoinBuilderOld       2,048 B/op     1 allocs/op
BenchmarkJoinNaive         ~381,000 B/op   398 allocs/op
BenchmarkJoinBuilder          2,048 B/op     1 allocs/op
```

- `JoinNaive` makes two allocations per part (one for each `+=`), and copies everything built so far each time, so its cost grows with the square of the input. `JoinBuilder` makes exactly one allocation of exactly the final size (1,799 bytes, rounded up to a 2,048-byte size class).
- Both benchmark styles measure the same thing here. The difference is what you have to remember: the old style needs `b.ResetTimer` and a `sink` variable, and reruns its setup on every `b.N` ramp-up. `b.Loop` needs neither.
- A 10-second `-fuzz` run executes a few million inputs (about 3.5 million on the machine used to write this) and finds no failure. If it did, the failing input would be saved under `testdata/fuzz/FuzzParse/`, and every later plain `go test` would replay it.
- With `-short`, `TestReportFromFile` reports `--- SKIP: ... skipping file-system test in -short mode`.
- Change `RenderReport` (for example the padding) and `TestRenderReportGolden` fails with both versions printed. `go test ./examples/024-advanced-testing -update` rewrites the golden file, and `git diff` shows what changed.

## Notes and corrections

- **Slide p02:** Go 1.24's `b.Loop()` replaces the `b.N` loop. Setup before the loop is excluded automatically, the benchmark function runs only once, and calls in the loop can't be optimised away. The runner doesn't analyse statistical significance; it grows `b.N` until the run fills `-benchtime` (1 s by default). For significance, run with `-count=10` and compare with `benchstat` (`golang.org/x/perf/cmd/benchstat`).
- **Slide p04:** fuzzing is coverage-guided mutation, not purely random input. Plain `go test` runs only the seed corpus; real fuzzing needs `-fuzz=FuzzParse`, for one fuzz target and one package at a time. Failing inputs are saved to `testdata/fuzz/FuzzParse/` and become regression tests.
- **Slide p05:** add `-run '^$'` when profiling benchmarks, so tests don't show up in the profile. `-memprofile mem.out` works the same way. In the pprof web UI, the flame graph is under View, Flame Graph.
- **Slide p07:** `-update` is not a built-in `go test` flag. The test file declares it (`var update = flag.Bool("update", false, "update golden files")`), and `go test` passes it through to the test binary. Golden files conventionally live in `testdata/`, which the Go tool ignores when building.
- **Slide p08:** `testing.Short()` reports whether `-short` was passed. `testcontainers-go` is a third-party module and needs Docker.
- **Slide p06:** calling a nil function field panics. The example's `MockClient.Get` falls back to an empty response instead.

## Slides

[024-advanced-testing.pdf](../../slides/024-advanced-testing.pdf)
