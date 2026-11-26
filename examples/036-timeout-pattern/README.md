# Part 036: Timeout Pattern: Select + time.After

Day 18 · Week 3 · Protecting your service boundaries.

## What this shows

- `select` racing a slow API against `time.After`. With a buffered (cap 1) result channel nothing leaks; with an unbuffered one the API goroutine is stuck forever, and `runtime.NumGoroutine()` shows it.
- The context version: one `context.WithTimeoutCause` deadline passed into a nested function, with `ctx.Err()` and `context.Cause(ctx)` telling "my limit fired" apart from "my caller cancelled".
- Since Go 1.23, 100,000 unfired `time.After(time.Hour)` timers in a loop are garbage-collected straight away; the heap only grows if you keep references to them.
- A heartbeat watchdog that calls `t.Reset(timeout)` with no drain, and fires exactly `timeout` after the last beat.
- A two-stage shutdown (drain tasks, then close resources), each stage with its own timeout, that returns an error instead of pretending to exit.

## Run

```sh
go run ./examples/036-timeout-pattern
go test ./examples/036-timeout-pattern
```

## What to look for

Expected output of `go run` (the heap sizes in section 3 and the duration in section 4 vary slightly per run):

```text
== 1. select + time.After, and the buffered result channel (slide p02) ==
buffered (cap 1): API call timed out; goroutines leaked: 0
unbuffered:       API call timed out; goroutines leaked: 1

== 2. The context version: one derived deadline with a cause (slides p04, p07) ==
local limit fires:
  fetchUser:          context deadline exceeded
  ctx.Err():          context deadline exceeded
  context.Cause(ctx): local operation exceeded its 50ms limit
caller cancels first:
  fetchUser:          context canceled
  ctx.Err():          context canceled
  context.Cause(ctx): user cancelled the request

== 3. time.After in a loop no longer leaks (Go 1.23+, slides p03, p09) ==
100000 time.After(1h) in a loop, after GC: +0.0 MiB heap in use
100000 timers still referenced:            +25.4 MiB heap in use

== 4. Heartbeat watchdog with Reset and no drain (slide p05) ==
5 heartbeats, then silence: worker watchdog triggered after 150ms (expected ~150ms)

== 5. Two-stage shutdown (slide p06) ==
-- healthy service --
Shutting down... draining tasks
Tasks finished cleanly
Resources closed cleanly
result: <nil>
-- stuck service --
Shutting down... draining tasks
Task drain timed out. Forcing resource cleanup...
Resource cleanup timed out.
result: resource cleanup timed out (a real main would now call os.Exit(1))
```

- Section 1: in both cases the caller got its timeout on time. The difference is what happens afterwards. Timing out does not stop the work either; only a `ctx` that the work honours can do that.
- Section 2: `ctx.Err()` is always one of the two sentinels, which is what callers compare against. `context.Cause` carries the reason, so a single `ctx.Done()` case replaces slide p07's separate `time.After(localLimit)` case.
- Section 3: the one-hour timers that nobody references cost nothing after a GC. The referenced ones show what the pre-1.23 behaviour looked like.
- The watchdog test runs inside `testing/synctest`, so it checks that the watchdog fires *exactly* `timeout` after the last heartbeat, in virtual time and in no real time at all.

## Notes and corrections

- **Slides p03, p09:** since Go 1.23, an unreferenced timer from `time.After` is garbage-collected immediately, even if it has not fired, so "it stays in memory until it expires" no longer applies (the module must say `go 1.23` or later in `go.mod`). `time.NewTimer` plus `Reset` is now only an allocation optimisation, and `defer t.Stop()` is not needed to free memory.
- **Slide p05:** the `if !t.Stop() { <-t.C }` drain is unnecessary since Go 1.23: `Reset` and `Stop` guarantee that no stale value is received afterwards. In general the idiom can block forever if the value was already received. The slide's comment says the worker is restarted, but the code only returns an error.
- **Slide p02:** the result channel must be buffered (cap 1), or the API goroutine blocks forever on its send after a timeout. The API work keeps running in either case unless it takes a `ctx`.
- **Slide p06:** "Hard exit." only prints a message. Here `GracefulShutdown` returns an error so the caller can exit for real.
- **Slide p07:** one derived context, `context.WithTimeoutCause(ctx, localLimit, cause)`, is simpler than a separate `time.After` case, and `context.Cause` still tells the two deadlines apart.

## Slides

[036-timeout-pattern.pdf](../../slides/036-timeout-pattern.pdf)
