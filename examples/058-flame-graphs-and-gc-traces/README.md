# Part 058: Reading the flames: Flame graphs & GC traces

Day 29 · Week 5 · Decoding the visual language of high-performance Go.

## What this shows

- An allocation-heavy loop (`fmt.Sprintf` per line) and an optimised loop (one reused `[]byte` with `strconv.AppendInt`) that produce byte-for-byte identical output, with `runtime/metrics` GC counters printed after each.
- A heap profile per loop, `heap_before.pprof` and `heap_after.pprof`, and the `-diff_base` command that compares them (slide p08).
- Slide p07's "slow" and "fast" `process` (as `processSlow` and `processFast`), benchmarked instead of assumed: with a save that doesn't keep its argument, both allocate nothing.
- The catch in the "fast" version: with a save that keeps its argument, every stored entry ends up pointing at the last item (a unit test proves it).
- Benchmarks for the other fixes on slides p05 and p07: `sync.Pool` of `*bytes.Buffer`, `make([]T, 0, cap)`, and `strconv`/`strings.Builder` instead of `fmt.Sprintf`.

## Run

```sh
go run ./examples/058-flame-graphs-and-gc-traces
go test ./examples/058-flame-graphs-and-gc-traces
GODEBUG=gctrace=1 go run ./examples/058-flame-graphs-and-gc-traces
go test -run='^$' -bench=. -benchmem -memprofile mem.out ./examples/058-flame-graphs-and-gc-traces
go tool pprof -http=:8080 -sample_index=alloc_space mem.out        # View -> Flame Graph
go build -gcflags=-m -o bin/ ./examples/058-flame-graphs-and-gc-traces      # escape analysis decisions
```

PowerShell form of the gctrace run: `$env:GODEBUG="gctrace=1"; go run ./examples/058-flame-graphs-and-gc-traces` (then `Remove-Item Env:GODEBUG`). With `go run`, the go command and the compiler also print their own `gc` lines first; build the binary and run it to see only the program's.

`-memprofile` writes `mem.out` (and a test binary) to the current directory; delete them afterwards.

## What to look for

Expected output of `go run` (the GC counts and the temporary directory vary; the checksums don't):

```text
== 1. Allocation-heavy loop: fmt.Sprintf per line (slides p04, p05) ==
formatted 1000000 lines, checksum 1e270ce1e94dd3bf
GC cycles:       20
heap allocated:  72.4 MB
wrote C:\Users\you\AppData\Local\Temp\flames-demo-140281053\heap_before.pprof

== 2. Optimised loop: one reused []byte buffer (slide p07) ==
formatted 1000000 lines, checksum 1e270ce1e94dd3bf
GC cycles:       0
heap allocated:  0.3 MB
wrote C:\Users\you\AppData\Local\Temp\flames-demo-140281053\heap_after.pprof

== 3. Prove the win with a differential profile (slides p06, p08) ==
# red = more than the base, green = less; View -> Flame Graph:
go tool pprof -http=:8080 -sample_index=alloc_space -diff_base=C:\Users\you\AppData\Local\Temp\flames-demo-140281053\heap_before.pprof C:\Users\you\AppData\Local\Temp\flames-demo-140281053\heap_after.pprof
# the same comparison as text:
go tool pprof -top -sample_index=alloc_space -diff_base=C:\Users\you\AppData\Local\Temp\flames-demo-140281053\heap_before.pprof C:\Users\you\AppData\Local\Temp\flames-demo-140281053\heap_after.pprof
# clean up when done:
rm -r C:\Users\you\AppData\Local\Temp\flames-demo-140281053
```

- Same checksum, about 70 MB of allocation versus almost none, and 20 GCs versus none. Nothing was leaked in either case: this is churn, which only the `alloc_space` view shows (slide p06).
- A heap profile is cumulative since the process started, so two profiles taken in one process can't show "before vs after". The program therefore starts itself once per phase (passing the phase in the `FLAMES_PHASE` environment variable), so each profile covers one loop, like comparing two builds on slide p08. The `-top -diff_base` output shows `main.heavyLoop`, `fmt.Sprintf` and `io.WriteString` as negative: allocations that went away.

### One gctrace line, field by field

A real line from `GODEBUG=gctrace=1` on the heavy loop (Windows, where the timer resolution rounds the clock times to multiples of about 0.5 ms):

```text
gc 1 @0.014s 0%: 0.51+1.1+0.51 ms clock, 8.1+0/0.51/0+8.2 ms cpu, 3->3->0 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 16 P
```

| Field | Meaning |
| --- | --- |
| `gc 1` | The first GC cycle of this process. |
| `@0.014s` | Time since the program started. |
| `0%` | Share of CPU time spent in GC since the program started. |
| `0.51+1.1+0.51 ms clock` | Wall-clock time of the three phases: stop-the-world sweep termination, **concurrent** mark and scan, stop-the-world mark termination. Only the first and last pause the program. |
| `8.1+0/0.51/0+8.2 ms cpu` | CPU time of the same phases; the middle one is split into assist / background / idle GC work. |
| `3->3->0 MB` | Heap size at GC start, at the end of marking, and the live heap that survived. Nearly everything was garbage. |
| `4 MB goal` | The heap size at which the next GC is due, set by `GOGC` (default 100) relative to the live heap. |
| `0 MB stacks, 0 MB globals` | Goroutine stacks and global data the GC had to scan. |
| `16 P` | Number of processors (`GOMAXPROCS`) used. |

A tiny live heap gives a tiny goal, so this loop triggers a GC roughly every 4 MB of allocation, about every 7 ms. That is the "many GCs per second" pattern from slide p05, caused by allocation rate, and it is harmless at this size.

### Benchmark shape

From one run (`-benchmem`, AMD Ryzen 7 5800X; numbers vary by machine):

```text
BenchmarkProcessSlow              1406 ns/op         0 B/op       0 allocs/op
BenchmarkProcessFast              1524 ns/op         0 B/op       0 allocs/op
BenchmarkProcessSlowRetained     21125 ns/op     32000 B/op    1000 allocs/op
BenchmarkBufferFresh               210 ns/op       272 B/op       3 allocs/op
BenchmarkBufferPooled              156 ns/op        80 B/op       1 allocs/op
BenchmarkAppendGrow              49057 ns/op    357627 B/op      19 allocs/op
BenchmarkAppendPrealloc          13156 ns/op     81920 B/op       1 allocs/op
BenchmarkLabelSprintf               76 ns/op        16 B/op       1 allocs/op
BenchmarkLabelConcat                47 ns/op        24 B/op       2 allocs/op
BenchmarkLabelBuilder               40 ns/op        16 B/op       1 allocs/op
```

- `processSlow` and `processFast` both report **0 allocs/op**, and "fast" is no faster. `go build -gcflags=-m` explains why:

  ```text
  process.go:47:21: []string{...} does not escape
  process.go:55:13: make([]string, 2) does not escape
  process.go:68:21: []string{...} escapes to heap
  process.go:77:13: make([]string, 2) escapes to heap
  ```

  The slice literal lives on the stack unless `save` keeps it. Once it does (`processSlowRetained`), each item costs one allocation, and that allocation is what makes the result correct.
- `sync.Pool` removes the buffer's growth allocations but not the returned string; preallocating removes all but one allocation; `strconv` plus a sized `strings.Builder` beats `fmt.Sprintf`. Plain concatenation with `strconv.Itoa` costs two allocations, the digits and the result.

## Notes and corrections

- **Slide p07:** the claim needs measuring. A slice literal that doesn't escape is stack-allocated, so the "slow" version can cost nothing extra, and the "fast" version is an aliasing bug if `save` retains its argument (`TestProcessFastRetainedAliasing`). The two functions also can't share the name `process` in one package.
- **Slide p02:** look at **self** time, the part of a frame's top edge not covered by its children. The base frame is always 100% wide, so a wide base is not a hot spot. pprof's web flame graph is drawn top-down.
- **Slide p04:** Go's GC is concurrent, with two short stop-the-world phases (the first and last clock numbers). `4->4->3 MB` means heap at GC start, at mark end, and live heap, not "memory cleared". Since Go 1.18 the line also ends with stacks, globals and `P` count.
- **Slide p05:** many GCs per second can be normal for a small heap, because the GC goal scales with the live heap. Reduce allocations first, then tune `GOGC` or `GOMEMLIMIT` (Go 1.19). `sync.Pool` must hold pointers such as `*bytes.Buffer`, or `Put` itself allocates (staticcheck SA6002).
- **Since publication:** Go 1.26 makes the Green Tea garbage collector the default, which lowers GC CPU cost on many workloads, so the slides' absolute GC-overhead numbers predate it.

## Slides

[058-flame-graphs-and-gc-traces.pdf](../../slides/058-flame-graphs-and-gc-traces.pdf)
