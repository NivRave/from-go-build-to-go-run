# Part 023: Testing philosophy - the "Go" way

Day 12 · Week 2 · High coverage != high quality.

## What this shows

- `main.go` runs the code under test: `split.Split` on the slide's inputs, and a `split.Service` with an injected `Storer`.
- `split/split_test.go` (`package split_test`, black-box): a table-driven `TestSplit` with subtests, `t.Parallel()` without `tt := tt`, `slices.Equal`, an `assertEqual` helper that calls `t.Helper()`, and an `ExampleSplit`.
- `split/internal_test.go` (`package split`, white-box): tests the unexported `cut` helper directly.
- `split/service_test.go`: a stub `Storer` injected through `NewService`, per-test teardown with `t.Cleanup`, and `t.Context()`.
- `split/main_test.go`: `TestMain` with setup and teardown and no `os.Exit`.

## Run

```sh
go run ./examples/023-testing-the-go-way
go test ./examples/023-testing-the-go-way/...
go test -v ./examples/023-testing-the-go-way/...
go test -run 'TestSplit/no_sep' -v ./examples/023-testing-the-go-way/split
go test -cover ./examples/023-testing-the-go-way/...
```

## What to look for

Expected output of `go run`:

```text
== 1. split.Split, the function under test (slide p03) ==
Split("a/b/c", "/") = ["a" "b" "c"]
Split("abc", "/") = ["abc"]
Split("a/", "/") = ["a" ""]
Split("a::b::c", "::") = ["a" "b" "c"]

== 2. split.Service with an injected Storer (slide p07) ==
Fields(user:1) = ["Ada" "Lovelace" "1815"], err = <nil>
Fields(user:2) = [], err = fields "user:2": no such record

Now run the tests: go test -v ./examples/023-testing-the-go-way/...
```

Running one subtest by name (the case is called `"no sep"`; `go test` turns spaces in names into underscores):

```text
$ go test -run 'TestSplit/no_sep' -v ./examples/023-testing-the-go-way/split
TestMain: setup done, 2 fixture records
=== RUN   TestSplit
=== PAUSE TestSplit
=== CONT  TestSplit
=== RUN   TestSplit/no_sep
=== PAUSE TestSplit/no_sep
=== CONT  TestSplit/no_sep
--- PASS: TestSplit (0.00s)
    --- PASS: TestSplit/no_sep (0.00s)
PASS
TestMain: teardown done
ok  	github.com/NivRave/from-go-build-to-go-run/examples/023-testing-the-go-way/split	0.166s
```

Coverage (timings vary):

```text
$ go test -cover ./examples/023-testing-the-go-way/...
	github.com/NivRave/from-go-build-to-go-run/examples/023-testing-the-go-way		coverage: 0.0% of statements
ok  	github.com/NivRave/from-go-build-to-go-run/examples/023-testing-the-go-way/split	0.177s	coverage: 100.0% of statements
```

- `TestMain` prints its setup and teardown lines only with `-v` (it checks `testing.Verbose()`). Teardown runs from a `defer` after `m.Run()` returns, which would not happen if it called `os.Exit`.
- With `-run 'TestSplit/simple_case'`, the slide's pattern, nothing matches: `go test` prints `testing: warning: no tests to run` and still reports `ok`. A typo in `-run` doesn't fail the build, so check that the test you meant actually ran.
- 100% coverage for `split` says every statement ran, not that the cases are the right ones. The `"leading"` and `"multi-char sep"` cases add no coverage, but they pin down behaviour a caller depends on.
- In `-v` output, the `cleanup: stub served N loads` lines come from `t.Cleanup` and appear after each service test body finishes.

## Notes and corrections

- **Slide p04:** `-run TestSplit/simple_case` doesn't match the case named `"simple"`. Use `-run 'TestSplit/simple'` or `'TestSplit/no_sep'`; spaces in subtest names become underscores. Subtests isolate `t.Fatal` and `t.FailNow` (only that subtest stops) and give per-case reporting. They don't isolate panics: a panic in a subtest still ends the whole test binary.
- **Slide p04:** `t.Parallel()` inside a `range` loop needed a `tt := tt` copy before Go 1.22. With per-iteration loop variables it is safe as written, and the `copyloopvar` linter flags the old copy as unnecessary.
- **Slide p05:** since Go 1.15, `TestMain` doesn't need `os.Exit(m.Run())`: when it returns, the binary exits with `m.Run()`'s code. Calling `os.Exit` also skips deferred calls, so a deferred teardown would never run. `t.Context()` (Go 1.24) gives each test a context that is cancelled just before its cleanup functions run.
- **Slides p03 and p06:** for slices, prefer `slices.Equal` (Go 1.21) to `reflect.DeepEqual`: it is type-checked and doesn't use reflection. `github.com/google/go-cmp` is a third-party module.
- **Slide p08:** `-failfast` is the flag that stops after the first failure; `-v` only adds verbosity. `-failfast` applies within one package's test binary, so other packages still run.
- **Slide p02:** external `_test` packages are linked into the same test binary. Their main benefit is that they can import packages that import the package under test, which breaks import cycles.

## Slides

[023-testing-the-go-way.pdf](../../slides/023-testing-the-go-way.pdf)
