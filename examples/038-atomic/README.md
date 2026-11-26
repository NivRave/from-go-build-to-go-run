# Part 038: Atomic Operations & Lock-Free Concepts

Day 19 · Week 3 · Speeding past the mutex.

## What this shows

- An `atomic.Uint64` request counter (slide p03) incremented by 1,000 goroutines gives the exact total. `-race-demo` does the same with a plain `uint64`.
- slide p04's CompareAndSwap retry loop on `atomic.Int32` (with the variable renamed from `new` to `next`), counting how often the swap had to be retried under contention.
- Why slide p05's CAS flag is not once-initialisation: the callers that lose the CAS return while the 50 ms init is still running. `sync.OnceValue` makes them wait.
- Copy-on-write configuration with `atomic.Pointer[Config]`: 8 readers and 2 writers. Slide p06's Load-copy-Store loses updates; a `CompareAndSwap` loop loses none.
- Benchmarks of `atomic.Int64.Add` vs a mutex increment, without and with contention.

## Run

```sh
go run ./examples/038-atomic
go test ./examples/038-atomic
go run -race ./examples/038-atomic -race-demo
go test -run='^$' -bench=. -cpu=1,8 ./examples/038-atomic
```

## What to look for

Expected output of `go run` (the retry count and the Load-copy-Store version vary per run):

```text
== 1. atomic.Uint64 request counter, 1,000 goroutines (slide p03) ==
1000 goroutines x Inc() = 1000

== 2. A CompareAndSwap retry loop (slide p04) ==
8 workers x 1000 updates of +5: value = 40000 (want 40000)
CAS retries under contention: 5549 (varies; 0 when nothing overlaps)

== 3. Once-only initialisation (slide p05) ==
CAS flag:       init ran 1 time; 9 of 10 callers returned before it finished
sync.OnceValue: init ran 1 time; 10 of 10 callers got the finished config

== 4. Copy-on-write config with atomic.Pointer (slide p06) ==
Load-copy-Store:     final version  5585 of 10000 writes; inconsistent snapshots seen: 0
CompareAndSwap loop: final version 10000 of 10000 writes; inconsistent snapshots seen: 0
(Load-copy-Store loses a varying number of updates; the CAS loop never does)
```

- Section 2: every retry is a moment where another goroutine changed the value between `Load` and `CompareAndSwap`. The loop never blocks, and the result is still exact.
- Section 3: both versions run the init body once. The difference is what the other nine callers see: with the CAS flag they carry on before the config exists.
- Section 4: Load-copy-Store has no data race (every access is atomic), so the race detector stays quiet, but two writers that copy the same version overwrite each other. The CAS loop retries instead. In both cases readers never see a half-updated config, because published snapshots are never modified. `config_test.go` checks this under `go test -race`.

Benchmark shape from one 8-core machine (numbers vary by machine):

```text
BenchmarkIncrement/AtomicInt64      ~1.7 ns/op    (-cpu=8: ~12 ns/op)
BenchmarkIncrement/Mutex            ~3.8 ns/op    (-cpu=8: ~32 ns/op)
```

- Without contention, the mutex costs about twice the atomic: an uncontended `Lock` is itself one atomic CAS, and `Unlock` is another. Nothing is parked and the scheduler is not involved.
- With 8 goroutines both get much slower, because every operation has to move the cache line from another core. `RunParallel` reports wall time divided by the total number of operations, so each individual increment takes several times the printed figure.

## Notes and corrections

- **PDF:** the original PDF had nine stray pages (p09-p17) from other posts. They have been removed, so page numbers here refer to the cleaned-up PDF.
- **Slide p02:** an uncontended `sync.Mutex` is itself an atomic CAS in user space and costs a few nanoseconds, with no scheduler or OS involvement; goroutines are parked only under contention. Contended atomics are not single-digit nanoseconds either: moving the cache line between cores costs tens to hundreds of nanoseconds. Modern x86 locks a cache line (`LOCK` prefix plus MESI), not the bus.
- **Slide p04:** don't name a variable `new`. It shadows the builtin, which since Go 1.26 also accepts an expression (`new(expr)`). "A single CPU instruction" holds on x86; ARM64 uses either an LL/SC loop or the ARMv8.1 LSE `CASAL` instruction.
- **Slide p05:** a CAS flag is not once-initialisation (and despite the text there is no loop). Callers that lose the CAS return immediately, before initialisation has finished. Use `sync.Once`, or `sync.OnceFunc`/`OnceValue`/`OnceValues` (Go 1.21), which block until the first call returns.
- **Slide p06:** Load, copy, Store loses updates when writers run concurrently. Use a `CompareAndSwap` loop (as `ConfigStore.Update` does) or serialise writers with a mutex. Prefer `atomic.Pointer[T]` (Go 1.19) to `atomic.Value`, which panics on a nil `Store` or on a value of a different concrete type.
- **Slide p08:** `atomic.Int64` and `atomic.Uint64` are guaranteed to be 8-byte aligned, even inside structs on 32-bit platforms. "Put it first in the struct" is only needed for plain `int64` fields used with the older `atomic.AddInt64`-style functions.

## Slides

[038-atomic.pdf](../../slides/038-atomic.pdf)
