# Part 037: Mutex vs. RWMutex

Day 19 · Week 3 · Protecting state without killing throughput.

## What this shows

- A `Counter` guarded by `sync.Mutex`, incremented by 1,000 goroutines started with `wg.Go` (Go 1.25): the total is exact. `-race-demo` runs the same loop without the mutex.
- Go mutexes are not re-entrant. `-deadlock` runs slide p06 as written and ends in the runtime's fatal error; the fix is an unexported `increment()` that assumes the lock is held.
- slide p04's `SessionCache` with the constructor it needs, read by eight goroutines under `RLock` while one writer uses `Lock`.
- A sharded map (32 shards picked with `hash/maphash`) used by 16 goroutines at once.
- Benchmarks: Mutex vs RWMutex for a tiny read and for a slow read at 99% reads, and a single-lock map vs the sharded map vs `sync.Map`.
- `go vet` catching slide p07's `BadValue`, which copies the mutex with a value receiver.

## Run

```sh
go run ./examples/037-mutex-vs-rwmutex
go test ./examples/037-mutex-vs-rwmutex
go run -race ./examples/037-mutex-vs-rwmutex -race-demo
go run ./examples/037-mutex-vs-rwmutex -deadlock          # ends in a fatal error on purpose
go vet -tags vetdemo ./examples/037-mutex-vs-rwmutex     # fails on purpose
go test -run='^$' -bench=. -cpu=1,4,8 ./examples/037-mutex-vs-rwmutex
```

## What to look for

Expected output of `go run` (the shard sizes on the last line vary per run):

```text
== 1. sync.Mutex counter, 1,000 goroutines (slide p02) ==
1000 goroutines x Inc() = 1000

== 2. No re-entrancy: lock once, call an unexported helper (slide p06) ==
Add(5) then Increment() = 6

== 3. RWMutex for read-heavy state (slides p03-p04) ==
8 readers x 1000 Get = 8000 hits; after 10 Sets, session-11 = "bob"

== 4. A sharded map: 32 locks instead of one (slide p08) ==
8000 keys across 32 shards; every shard used: true
smallest shard 226 keys, largest 267 (varies: the hash seed is random)
```

`-race-demo` without `-race` usually loses a few updates (`1000 goroutines x value++ = 993`); with `go run -race` the race detector also reports the data race and points at `value++`.

`-deadlock` locks `mu` twice from one goroutine. Nothing else can ever unlock it, so the runtime aborts with exit status 2:

```text
calling Add(5), which calls Increment() while holding the lock...
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [sync.Mutex.Lock]:
```

In a real server other goroutines are still running, so the runtime cannot detect it: that goroutine just hangs.

`go vet -tags vetdemo ./examples/037-mutex-vs-rwmutex` reports the lock copy in `vetdemo.go`:

```text
examples\037-mutex-vs-rwmutex\vetdemo.go:12:9: BadValue passes lock by value: github.com/NivRave/from-go-build-to-go-run/examples/037-mutex-vs-rwmutex.Counter contains sync.Mutex
```

Benchmark shape from one 8-core machine (`-cpu=1,4,8`; numbers vary by machine and run):

```text
BenchmarkTinyRead/Mutex         ~5 ns/op    ~11 ns/op    ~33 ns/op
BenchmarkTinyRead/RWMutex       ~4 ns/op    ~20 ns/op    ~21 ns/op
BenchmarkSlowRead99/Mutex     ~250 ns/op   ~280 ns/op   ~255 ns/op
BenchmarkSlowRead99/RWMutex   ~220 ns/op   ~115 ns/op   ~130 ns/op
BenchmarkMap/SingleLock        ~17 ns/op    ~47 ns/op    ~54 ns/op
BenchmarkMap/Sharded32         ~21 ns/op    ~21 ns/op    ~19 ns/op
BenchmarkMap/SyncMap           ~27 ns/op    ~11 ns/op    ~8 ns/op
```

- Tiny reads: neither lock wins consistently. Readers of an RWMutex still update a shared reader count, so the cache line bounces between cores just like a Mutex's. That is slide p05's trap: switch only when a profile shows reader contention.
- Slow reads: once readers hold the lock long enough to overlap, RWMutex is about twice as fast under contention.
- Maps: the single lock gets slower as cores are added; the sharded map stays flat; `sync.Map` scales best here.

## Notes and corrections

- **Slide p04:** `data` is never initialised, so the first `Set` panics with `assignment to entry in nil map`; `NewSessionCache` fixes that. A recursive `RLock` is also a deadlock risk: if a writer is waiting between the two `RLock` calls, the second one waits for the writer, which waits for the first.
- **Slide p07:** a mutex's state is more than a waiter count (a lock bit, woken and starving flags, the waiter count, plus a semaphore). `go vet`'s `copylocks` check catches the copy automatically, as shown above.
- **Slide p08:** Go 1.24 rebuilt `sync.Map` on a concurrent hash-trie, which makes it competitive with hand-rolled sharding for many workloads (see `BenchmarkMap`). For the plain counter case, `atomic.Int64` (Go 1.19) needs no lock at all (Part 038).
- **Slide p02:** `sync.Mutex` has a starvation mode. If a waiter has been blocked for more than 1 ms, the mutex hands ownership directly to waiters in FIFO order until the queue drains, trading some throughput for fairness.

## Slides

[037-mutex-vs-rwmutex.pdf](../../slides/037-mutex-vs-rwmutex.pdf)
