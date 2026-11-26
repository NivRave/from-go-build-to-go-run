# Part 041: Timers and Tickers: The Pulse of Go

Day 21 · Week 3 · Mastering time-based events in concurrent systems.

## What this shows

- `time.Sleep(2s)` and a timer in a `select`, both cancelled at 300 ms: only the timer version reacts, the sleeping one notices 1.7 s late.
- A ticker loop (100 ms ticks) under a ~550 ms context. With a handler slower than the interval, ticks are dropped rather than queued.
- `time.AfterFunc` as a slow-operation warning (threshold 100 ms): a 250 ms job triggers it, a 20 ms job stops it in time. Plus `context.AfterFunc`, the same idea triggered by cancellation.
- One timer reused for a whole idle-timeout loop with `Reset` and no drain, and a check that `Reset` discards a value from an earlier, unreceived expiry (Go 1.23 semantics).
- Benchmarks of `time.After` in a loop versus a reused timer: the remaining difference is allocations, not leaks.

## Run

```sh
go run ./examples/041-timers-and-tickers
go test ./examples/041-timers-and-tickers
go test -run='^$' -bench=. -benchmem ./examples/041-timers-and-tickers
```

## What to look for

Expected output of `go run` (it uses real time, so rounded durations can differ on a very busy machine):

```text
== 1. time.Sleep vs a timer in a select (slides p02-p04) ==
time.Sleep(2s): returned after 2s: context deadline exceeded
timer + select: returned after 300ms: context deadline exceeded

== 2. A ticker loop, and a handler slower than the interval (slides p05-p06) ==
instant handler: 5 ticks in 550ms
250ms handler:   2 ticks in 550ms (the others were dropped, not queued)

== 3. time.AfterFunc and context.AfterFunc (slide p07) ==
time.AfterFunc, threshold 100ms:
  warning: 250ms job is exceeding 100ms
  250ms job done: Stop() = false, the warning had already started
  20ms job done in time: Stop() = true, the warning never ran
context.AfterFunc:
  ctx cancelled: closing the connection
  stop() = false (already ran)

== 4. Reusing one timer with Reset, no drain (Go 1.23+, slides p04, p08) ==
event 1: timer reset
event 2: timer reset
event 3: timer reset
no event for 50ms: idle timeout
Reset after an unreceived expiry: next value after ~50ms (no stale value)
```

- Section 2: a ticker's channel holds at most one pending tick. While the 250 ms handler is busy, one tick waits and the next is dropped, so the loop never builds a backlog. `ticker_test.go` checks the exact counts (5, 4 and 2 ticks for 0, 120 and 250 ms handlers) and the exact return times inside `testing/synctest`, in virtual time.
- Section 4: with the pre-1.23 semantics, the last receive would have returned immediately with the stale value from the earlier expiry. Now it waits the full 50 ms.

Benchmark shape (numbers vary by machine):

```text
BenchmarkTimeAfterInLoop   ~165 ns/op   248 B/op   3 allocs/op
BenchmarkReusedTimer        ~67 ns/op     0 B/op   0 allocs/op
```

- The reused timer saves an allocation per iteration. It is a micro-optimisation for hot loops, not a fix for a leak: the unfired `time.After` timers are collected as soon as the `select` returns.

## Notes and corrections

- **Slides p08, p09:** since Go 1.23, timers that nobody references any more are garbage-collected even if they never fired or were never stopped, so `time.After` in a loop no longer leaves "ghost timers" until they expire (the module must say `go 1.23` or later in `go.mod`). `NewTimer` plus `Reset` still saves allocations.
- **Slide p05:** a `Ticker` that becomes unreachable no longer needs `Stop()` to avoid a leak, though stopping a ticker you are done with is still good practice. A ticker drops ticks for slow receivers instead of queueing them, and `Ticker.Reset` (Go 1.15) changes its period.
- **Slide p04:** the `if !timer.Stop() { <-timer.C }` drain is unnecessary since Go 1.23, because `Stop` and `Reset` guarantee no stale value is received afterwards. Combined with `defer timer.Stop()`, the slide also stops the timer twice.
- **Slide p07:** `Stop()` returning `false` means the function has already started; `Stop` does not wait for it to finish. `context.AfterFunc` (Go 1.21) runs a function when a context is cancelled.

## Slides

[041-timers-and-tickers.pdf](../../slides/041-timers-and-tickers.pdf)
