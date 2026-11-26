# Part 039: Context - The Concurrency Controller

Day 20 · Week 3 · Mastering the "stop" signal and scoped data in Go.

## What this shows

- A cancellation tree: a child's timeout stops only its own grandchildren, while cancelling the root stops everything below it.
- slide p05's worker with the missing `j, ok := <-jobs` check, stopped once by closing `jobs` and once by cancelling `ctx`, with `runtime.NumGoroutine()` before and after.
- `context.WithValue` with an unexported key type, wrapped in `WithUser`/`UserFrom` helpers, and why another package's key cannot collide with it.
- `WithCancelCause` and `WithTimeoutCause`: `ctx.Err()` says *that* it was cancelled, `context.Cause` says *why*.
- `context.AfterFunc` running cleanup when a request is cancelled, and `context.WithoutCancel` letting an audit write finish after the request is gone.
- A simulated `SlowDatabaseQuery(ctx)` that returns as soon as its deadline fires, used with slide p07-p08's `WithTimeout` + `defer cancel()`.

## Run

```sh
go run ./examples/039-context
go test ./examples/039-context
```

## What to look for

Expected output of `go run` (the goroutine counts in section 2 depend on the program; under `go test` they are higher):

```text
== 1. The cancellation tree (slide p04) ==
short's 50ms timeout fires:
  short/g1 stopped: context deadline exceeded
  short/g2 stopped: context deadline exceeded
  root.Err() = <nil>, manual.Err() = <nil> (a child ending does not reach its parent or siblings)
cancelRoot():
  manual/g1 stopped: context canceled
  manual/g2 stopped: context canceled
  manual.Err() = context canceled (cancellation flows downstream)

== 2. A context-aware worker that cannot leak (slide p05) ==
stopped by close(jobs):    processed 10 jobs; goroutines before 1, running 4, after 1
stopped by cancel():       processed 10 jobs; goroutines before 1, running 4, after 1

== 3. Values the right way: an unexported key type (slide p06) ==
UserFrom(ctx)          = "alice", true
TraceIDFrom(ctx)       = "abc-123", true
UserFrom(Background()) = "", false
ctx.Value(otherKey(0)) = <nil> (same number, different type: no collision)

== 4. Why it was cancelled: WithCancelCause, WithTimeoutCause (Go 1.20, 1.21) ==
WithCancelCause:  Err() = context canceled; Cause = payment declined
WithTimeoutCause: Err() = context deadline exceeded; Cause = database took longer than 10ms
WithCancel:       Err() = context canceled; Cause = context canceled (no cause given: same as Err)

== 5. context.AfterFunc and context.WithoutCancel (Go 1.21) ==
stop() before cancel: true (unregistered, will not run)
AfterFunc: request ended, releasing its DB connection
stop() after it ran:  false (already started)
request Err() = context canceled; audit Err() = <nil>; audit log for "alice" written, err = <nil>

== 6. A slow query with a deadline (slides p07, p08) ==
timeout 2s     -> res = "3 rows", err = <nil>
timeout 50ms   -> res = "", err = context deadline exceeded
```

- Section 1: grandchildren report the error of the ancestor that ended them, so `short/g1` says "deadline exceeded" although it was created with `WithCancel`.
- Section 2: both ways of stopping return every worker; nothing is left running.
- Section 5: `stop()` returning `false` means the function has already been started, not that it has finished. `WithoutCancel` keeps the values (the user is still there) but its `Done()` never closes.
- `query_test.go` tests `SlowDatabaseQuery` inside `testing/synctest` with `t.Context()` (Go 1.24): the query returns after exactly 100 ms, 50 ms or 30 ms of virtual time, depending on what ends first.

## Notes and corrections

- **Slide p05:** without the `ok` check, a closed `jobs` channel makes `j := <-jobs` return the zero value immediately, forever: the worker spins calling `process(0)` until `ctx` is cancelled.
- **Slide p08:** the code comment is not accurate. When the deadline fires, the context cancels itself and releases its timer and its link to the parent; not calling `cancel` only holds those resources **until** then (the slide's text says this correctly). Always `defer cancel()` anyway; `go vet`'s `lostcancel` check reports a cancel function that is never called.
- **Slide p04:** newer APIs worth knowing: `WithCancelCause` and `context.Cause` (Go 1.20); `WithTimeoutCause`, `WithDeadlineCause`, `AfterFunc` and `WithoutCancel` (Go 1.21); `signal.NotifyContext` for Ctrl+C (Go 1.16); and `t.Context()` in tests (Go 1.24). `WithoutCancel` is the deliberate exception to "cancelling the parent stops all children".
- **Slide p06:** pair the getter with a setter (`WithUser`), so no caller ever handles the key directly.
- **Slide p03:** the `$1` placeholder is PostgreSQL syntax; MySQL and SQLite drivers use `?`.

## Slides

[039-context.pdf](../../slides/039-context.pdf)
