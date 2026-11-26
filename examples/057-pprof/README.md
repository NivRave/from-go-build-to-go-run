# Part 057: Performance profiling with pprof

Day 29 · Week 5 · Peering into the "engine room" of your Go application.

## What this shows

- A CPU profile recorded with `runtime/pprof` while three deliberately wasteful workloads run: SHA-256 hashing (CPU), naive string building plus JSON (allocations) and eight goroutines fighting over one mutex (contention).
- Heap, mutex and block profiles written next to it, with moderate sampling rates (`SetBlockProfileRate(10_000)`, `SetMutexProfileFraction(100)`) instead of the slide's record-everything `1`.
- The exact `go tool pprof` commands to open each profile, including the web UI and its flame graph.
- With `-serve`: the app on its own mux at `localhost:8080` (`GET /cpu`, `/alloc`, `/leak`, `/lock`) and pprof registered **explicitly** on a second mux at `localhost:6060`, so the profiler is never reachable through the app's port. `-load` adds a built-in load generator.

## Run

```sh
go run ./examples/057-pprof
go test ./examples/057-pprof
go run ./examples/057-pprof -serve -load          # Ctrl+C to stop
```

The default run writes four profiles to a fresh temporary directory and prints its path. Open them with the commands it prints, for example:

```sh
go tool pprof -top <dir>/cpu.pprof
go tool pprof -http=:8081 -sample_index=alloc_space <dir>/heap.pprof   # View -> Flame Graph
```

With `-serve -load` running, in a second terminal:

```sh
curl 'localhost:6060/debug/pprof/goroutine?debug=1'
go tool pprof localhost:6060/debug/pprof/mutex
go tool pprof -top 'localhost:6060/debug/pprof/profile?seconds=10'
go tool pprof -http=:8081 -sample_index=alloc_space localhost:6060/debug/pprof/heap
```

## What to look for

Expected output of `go run` (the counts and the temporary directory change on every run and machine):

```text
== 1. CPU profile while three workloads run (slides p02, p04) ==
hashWork:  385536 SHA-256 rounds (CPU-bound)
allocWork: 86473132 bytes of JSON (allocation-bound)
lockWork:  1836446000 increments by 8 goroutines (lock-contended)
wrote C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\cpu.pprof (1.9s of samples)

== 2. Heap, mutex and block profiles (slides p05, p08) ==
wrote C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\heap.pprof
wrote C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\mutex.pprof
wrote C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\block.pprof

== 3. Open them with go tool pprof (slide p06) ==
# hottest functions, flat and cumulative:
go tool pprof -top C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\cpu.pprof
# interactive graph and flame graph (View -> Flame Graph):
go tool pprof -http=:8081 C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\cpu.pprof
# every byte ever allocated (churn) instead of what is still live:
go tool pprof -http=:8081 -sample_index=alloc_space C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\heap.pprof
# who held contended locks / who waited:
go tool pprof -top C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\mutex.pprof
go tool pprof -top C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298\block.pprof
# clean up when done:
rm -r C:\Users\you\AppData\Local\Temp\pprof-demo-1776716298
```

The top of a real `go tool pprof -top cpu.pprof` (trimmed; your numbers will differ). Note that `sum%` only ever grows, unlike the screenshot on slide p04:

```text
Duration: 1.90s, Total samples = 2.46s (129.49%)
      flat  flat%   sum%        cum   cum%
     0.52s 21.14% 21.14%      0.52s 21.14%  crypto/internal/fips140/sha256.blockSHANI
     0.42s 17.07% 38.21%      0.55s 22.36%  main.lockWork.func1
     0.18s  7.32% 45.53%      0.18s  7.32%  runtime.semawakeup
     0.09s  3.66% 49.19%      0.09s  3.66%  runtime.memmove
     0.08s  3.25% 52.44%      0.37s 15.04%  runtime.concatstrings
```

- Each profile points at a different workload: the CPU profile at the SHA-256 block function, the `alloc_space` heap view at `main.buildReport` (about 95% of all bytes, from the `+=` string building), and the mutex and block profiles at `main.lockWork.func1`.
- `Total samples` can exceed `Duration` (129% above): the profiler samples every running thread, so with eight goroutines on several cores there is more than one second of CPU per second of wall time.
- `flat` is time in the function itself; `cum` includes its callees. `runtime.concatstrings` has small `flat` but large `cum` because the copying happens in `memmove` below it.
- In `-serve` mode, `GET localhost:8080/debug/pprof/` returns 404: the profiler exists only on the localhost-only port 6060. The goroutine dump at `?debug=1` groups identical stacks, so the leak shows as one entry, `main.leakHandler.func1`, with a count that grows with every `/leak` request.

PGO: a CPU profile is also an optimisation input. Copy a representative profile to `default.pgo` in the main package's directory and `go build` uses it automatically (Go 1.21+); the Go team reports gains of around 2-7%, mostly from better inlining.

## Notes and corrections

- **Slide p03:** `http.ListenAndServe(":6060", nil)` listens on every interface, not only an internal one. The blank import registers the handlers on `http.DefaultServeMux`, so an app that also serves the default mux exposes pprof publicly. Bind to `localhost:6060` and register the handlers on a separate mux, as `server.go` does. (Importing `net/http/pprof` by name still registers on the default mux; the safety comes from never serving it.)
- **Slide p02:** the CPU profiler doesn't pause the program. It samples about 100 times per second using a `SIGPROF` timer on Unix and records the stack of whatever was running; it doesn't stop the world.
- **Slides p04 and p06:** the screenshots are illustrative, not real output. In real `top` output `sum%` is cumulative, and some of the function names shown don't exist. See the real excerpt above.
- **Slide p08:** a rate or fraction of `1` records every blocking or contention event, which costs noticeably in production. Use a sampling value (this example uses `10_000` ns and `100`) or enable profiling temporarily.
- **Since publication:** Go 1.25 added `trace.FlightRecorder` for capturing the last few seconds of an execution trace on demand, which suits off-CPU problems (I/O, scheduler latency) better than these profiles. Go 1.26 adds an experimental goroutine-leak profile (`GOEXPERIMENT=goroutineleakprofile`, served at `/debug/pprof/goroutineleak`), which finds goroutines like the one `/leak` creates using the garbage collector.

## Slides

[057-pprof.pdf](../../slides/057-pprof.pdf)
