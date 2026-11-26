# Part 032: Launching Tasks Safely

Day 16 · Week 3 · Orchestrating goroutines with sync.WaitGroup.

## What this shows

- Lost work: a function starts 5 goroutines and returns without waiting. An atomic counter shows how many had finished by then, usually none.
- `sync.WaitGroup` the classic way (slide p03's `worker(id, &wg)` with `Add`/`Done`/`Wait`) and Go 1.25's `wg.Go(func() {...})`, which does the `Add(1)`, the `go` and the `defer Done()` for you.
- Loop variables captured directly in `wg.Go` closures: since Go 1.22 each iteration has its own variable, so every goroutine sees a different value.
- `errgroup.WithContext` with `SetLimit(2)`: five fake fetches, one fails, the ones still running or not yet started observe `ctx.Done()`, and `Wait` returns the first error.
- A context-aware ticker worker (slide p07's "RIGHT" version): `runtime.NumGoroutine()` before, during and after shows it exits when the context is cancelled.

## Run

```sh
go run ./examples/032-launching-tasks-safely
go test ./examples/032-launching-tasks-safely
go vet -tags vetdemo ./examples/032-launching-tasks-safely   # the bug from slide p04; exits 1
go run -race ./examples/032-launching-tasks-safely             # needs cgo (a C compiler)
```

## What to look for

Output of one run. Lines in section 2 come out in a different order on every run, and the counts in sections 1 and 5 can vary:

```text
== 1. The race to the exit (slide p02) ==
Exiting...
goroutines finished by then: 0 of 5

== 2. sync.WaitGroup: Add/Done/Wait vs. wg.Go (slides p03-p04, Go 1.25) ==
Worker 3 starting
Worker 1 starting
Worker 2 starting
Worker 2 done
Worker 3 done
Worker 1 done
classic: all workers done
task 3 done
task 2 done
task 1 done
wg.Go: all tasks done

== 3. Loop variables are per iteration (slide p05, Go 1.22) ==
captured: [alpha beta delta gamma]
all distinct: true

== 4. errgroup with WithContext and SetLimit (slides p06, p08) ==
a.example    ok
b.example    ok
bad.example  failed
c.example    cancelled
d.example    cancelled
peak concurrency: 2
Wait returned: fetch bad.example: connection refused

== 5. A worker with an exit path (slide p07) ==
goroutines: 1 before, 2 while the worker runs
cleanups run before cancel: 3
back to 1 or fewer after cancel: true
```

- `0 of 5`: returning (or exiting `main`) doesn't wait for goroutines. `time.Sleep` would make it work most of the time, which is worse than never.
- In section 4, `SetLimit(2)` keeps `bad.example` from starting until a slot frees up, so `peak concurrency` is 2. When it fails, the group cancels `ctx`: `c.example`, which was mid-fetch, and `d.example`, which started afterwards, both return `context.Canceled`, and `Wait` reports only the first error.
- Section 5's goroutine count goes up by one while the worker runs and back down after `cancel()`. Without the `ctx.Done()` case, that goroutine would tick forever.

`go vet -tags vetdemo` compiles `vetdemo.go`, which calls `wg.Add(1)` inside the goroutine. Real output from Go 1.26 on Windows:

```text
examples\032-launching-tasks-safely\vetdemo.go:22:10: WaitGroup.Add called from inside new goroutine
```

## Notes and corrections

- **Slides p03, p04, p09:** Go 1.25 added `wg.Go(func() {...})`, which removes the "Add before go" footgun, and its `waitgroup` vet analyzer reports `Add` called inside the new goroutine (see above).
- **Slide p06:** `url := url` is redundant since Go 1.22, when each loop iteration got its own variable (for modules whose `go.mod` says `go 1.22` or later). `g.SetLimit(n)` bounds concurrency, which is slide p08's throttling advice built in. The `ctx` from `WithContext` is cancelled on the first error, which is why `fetch` takes it.
- **Slide p05:** passing the value as an argument isn't "shadowing"; it gives the goroutine its own copy. Since Go 1.22 plain capture is just as correct, as section 3 shows, and the `modernize` analyzer removes now-redundant `v := v` copies.
- **Slides p03/p09:** `defer wg.Done()` doesn't save you from a panic: an unrecovered panic in any goroutine crashes the whole program. What `defer` does cover is early returns.
- **Slide p07:** since Go 1.23 an unreferenced Ticker is garbage-collected even if it was never stopped, so `defer ticker.Stop()` is hygiene rather than a leak fix. The "WRONG" goroutine still leaks, because the goroutine itself never exits.
- **Slide p08:** each goroutine costs at least a 2 KB stack plus a few hundred bytes of bookkeeping, and since Go 1.19 stacks can start larger, so 200 MB for 100k goroutines is a fair lower estimate.

## Slides

[032-launching-tasks-safely.pdf](../../slides/032-launching-tasks-safely.pdf)
