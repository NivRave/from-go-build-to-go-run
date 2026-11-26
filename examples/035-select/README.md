# Part 035: The Select Statement

Day 18 · Week 3 · Orchestrating multiple channels.

## What this shows

- When several cases are ready, `select` picks one at random: two always-ready channels split 10,000 rounds roughly 50/50.
- `default` makes a send non-blocking, which is how you shed load: a burst of 20 tasks into a queue of 5 gives exactly 5 queued and 15 dropped.
- The nil-channel trick: setting a closed channel to `nil` switches its case off, so a merge loop ends when both producers are done.
- A true single-select bridge that receives and sends in the same `select`, by enabling the send case only when a value is pending. The slide's nested version is shown next to it.
- A daemon that waits on `signal.NotifyContext` instead of `select{}`, so it can clean up. A 200 ms fallback timeout stands in for Ctrl+C unless you pass `-wait`.

## Run

```sh
go run ./examples/035-select
go test ./examples/035-select
go run ./examples/035-select -wait        # stop it with Ctrl+C
go run ./examples/035-select -deadlock    # ends in a fatal error on purpose
go test -run='^$' -bench=Select ./examples/035-select
```

## What to look for

Expected output of `go run` (the split in section 1 varies per run):

```text
== 1. Ready cases are chosen at random (slide p02) ==
chan1 won 5074 times, chan2 won 4926 times out of 10000 (about 50/50, varies per run)

== 2. default: load shedding (slide p03) ==
burst of 20: 5 queued, 15 shed
worker processed 5 tasks after the burst

== 3. The nil-channel trick: merging two producers (slide p04) ==
from ch1: [1 2 3]
from ch2: [10 20 30 40 50]
both channels are nil: loop finished

== 4. A true single-select bridge (slide p06) ==
nested selects (slide):        producer finished before the consumer started: false got [1 2 3 4 5]
single select + nil send case: producer finished before the consumer started: true  got [1 2 3 4 5]

== 5. A daemon without select{} (slide p07) ==
worker: cleaning up
main: shutdown requested (context deadline exceeded), worker stopped, exiting cleanly
```

- Section 4: with nested selects, the bridge stops receiving while it waits on `out <- v`, so the producer is stuck until someone reads `out`. The single-select bridge keeps accepting input into `pending` and delivers it later, in order.
- `-deadlock` runs a bare `select{}` with no other goroutine to wake it. The runtime detects that and aborts with exit status 2:

```text
calling select{} with no other goroutines...
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [select (no cases)]:
```

Benchmark shape (numbers vary by machine):

```text
BenchmarkSelect2Cases            ~60 ns/op    0 allocs/op
BenchmarkSelect16Cases          ~580 ns/op    0 allocs/op
BenchmarkReflectSelect16Cases  ~1060 ns/op   20 allocs/op
```

- The cost of a `select` grows with the number of cases, even when only one is ready: every listed channel is locked and polled on every operation. `reflect.Select` handles a case count known only at run time, at a further cost.

## Notes and corrections

- **Slide p02:** the title says "first come, first served", but when several cases are ready `select` chooses one uniformly at random. That randomness is what prevents starvation.
- **Slide p06:** the code uses two nested selects. While it is blocked on `out <- v` it is not receiving from `in`, so the loop does block when `out` is full. A single `select` that sends and receives needs a pending value and a send case that is `nil` while nothing is pending (section 4).
- **Slide p07:** `select{}` with no other runnable goroutines ends in `fatal error: all goroutines are asleep - deadlock!`. For a daemon, `signal.NotifyContext` plus `<-ctx.Done()` sleeps just as cheaply and allows a clean shutdown.
- **Slide p08:** the "rule of 10" is a rule of thumb, not a Go rule. The cost is per case on every `select` (lock all channels in address order, poll in random order), which is per-operation overhead rather than lock contention. `reflect.Select` exists for a dynamic number of cases.

## Slides

[035-select.pdf](../../slides/035-select.pdf)
