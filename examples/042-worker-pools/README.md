# Part 042: Worker Pools: Orchestrating Scale

Day 21 · Week 3 · Managing high-throughput concurrency without crashing your system.

## What this shows

- A complete pool: `-workers` (default `runtime.GOMAXPROCS(0)`) and `-jobs` (default 1000) flags, workers started with `wg.Go`, a producer that sends and then `close(jobs)`, a collector that runs `wg.Wait(); close(results)`, and `main` ranging over the results and checking the sum.
- Pool sizing: 200 CPU-bound jobs (a short hash loop) and 200 I/O-bound jobs (10 ms sleeps) at 1, GOMAXPROCS and 100 workers. The right size depends on the kind of work.
- Classic fan-in: each worker writes to its own channel, and `merge(cs ...<-chan int) <-chan int` combines them into one.
- An early stop with `context.WithTimeout`: workers check `ctx.Done()` while waiting for jobs and while sending results, so the pool stops within one job's time and leaves no goroutine behind.

## Run

```sh
go run ./examples/042-worker-pools
go test ./examples/042-worker-pools
go run ./examples/042-worker-pools -workers=8 -jobs=1000
go run -race ./examples/042-worker-pools
go test -run='^$' -bench=Pool ./examples/042-worker-pools
```

## What to look for

Expected output of `go run` on a machine with 16 logical CPUs (worker counts, per-worker splits, durations and the number of early results vary):

```text
== 1. A complete pool (slides p03-p07) ==
16 workers, 1000 jobs: 1000 results, sum 1001000 (want 1001000, ok: true)
jobs per worker: min 21, max 108 (varies per run)

== 2. Sizing: CPU-bound vs I/O-bound work (slide p08) ==
200 jobs, GOMAXPROCS = 16 (durations vary by machine)
workers       CPU-bound    I/O-bound
1                  51ms       2.081s
16                  5ms        135ms
100                 4ms         22ms

== 3. Classic fan-in: merge one channel per worker (slide p05) ==
4 worker channels merged into one: sum 10100 (want 10100)

== 4. Early stop with a context ==
4 workers, 1000 jobs of 10ms, 50ms budget: context deadline exceeded after 18 results (varies, about 20)
returned after ~50ms; goroutines leaked: 0
```

- Section 1: the per-worker split is uneven and changes every run. Workers pull jobs when they are free; nobody assigns them.
- Section 2: CPU-bound work stops improving at GOMAXPROCS, because there are no more cores to run on. I/O-bound work keeps improving well past it, because the workers are mostly waiting.
- Section 4: `pool_test.go` runs the same early stop inside `testing/synctest` and checks, in virtual time, that the pool returns exactly when the in-flight jobs finish and that no goroutine is left blocked.

Benchmark shape (`BenchmarkPool`, 200 CPU-bound jobs; numbers vary by machine):

```text
workers=1     ~55 ms/op
workers=2     ~27 ms/op
workers=4     ~14 ms/op
workers=16     ~4 ms/op    (GOMAXPROCS)
workers=64     ~4 ms/op
```

## Notes and corrections

- **Slide p06:** `worker(jobs, results)` is missing the `id` argument from the p04 signature, so it does not compile; p07 passes it.
- **Slide p07:** jobs are never sent or closed and results are never read. As written the workers block forever on `range jobs`, and with more jobs than the buffers hold, the producer and the workers deadlock. `Run` adds the producer, the `close(jobs)` and the results loop.
- **Throughout:** `wg.Go(func() { ... })` (Go 1.25) replaces `wg.Add(1)` plus `defer wg.Done()`, and passing `w` as an argument has been unnecessary since Go 1.22, when each loop iteration got its own variable.
- **Slide p08:** size CPU-bound pools with `runtime.GOMAXPROCS(0)`, not `runtime.NumCPU()`. Since Go 1.25, GOMAXPROCS respects container CPU limits, while `NumCPU` does not. For simple cases, `errgroup.Group.SetLimit` or a semaphore channel is lighter than a hand-written pool.
- **Slide p05:** the pool in the slides has all workers writing to one shared results channel. Classic fan-in merges N separate channels into one, as `merge` does here. Both are valid.
- **Slide p09:** backpressure comes from *bounded* channels (an unbuffered channel gives it too). A large buffer only delays it.

## Slides

[042-worker-pools.pdf](../../slides/042-worker-pools.pdf)
