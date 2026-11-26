# Part 031: Goroutines: The M:P:G Model

Day 16 · Week 3 · The magic of the Go scheduler.

## What this shows

- The numbers behind the model: `runtime.NumCPU()`, `runtime.GOMAXPROCS(0)` (the number of Ps, container-aware on Linux since Go 1.25) and `runtime.NumGoroutine()`.
- How cheap goroutines are: 10,000 of them, started with `wg.Go` and parked on one channel, start in milliseconds and cost a few KiB of stack each (`MemStats.StackInuse`).
- Asynchronous preemption: with `GOMAXPROCS(1)`, a goroutine spinning in `for !stop.Load() {}` with no function calls would hold the only P forever. `main` still wakes up and stops it, because the scheduler preempts the loop with a signal (Go 1.14+).
- Syscall handoff: with `GOMAXPROCS(1)`, a goroutine blocked in a real read syscall on an `os.Pipe` doesn't stop the others, because the runtime moves the P to another thread. The `threadcreate` profile shows how many OS threads exist.

## Run

```sh
go run ./examples/031-goroutines

# Print scheduler state every second (Ps, threads, run queues):
GODEBUG=schedtrace=1000 go run ./examples/031-goroutines

# Turn asynchronous preemption off: demo 3 then hangs forever. Stop it with Ctrl+C.
GODEBUG=asyncpreemptoff=1 go run ./examples/031-goroutines
```

PowerShell forms:

```powershell
$env:GODEBUG="schedtrace=1000"; go run ./examples/031-goroutines; Remove-Item Env:GODEBUG
$env:GODEBUG="asyncpreemptoff=1"; go run ./examples/031-goroutines   # Ctrl+C to stop
Remove-Item Env:GODEBUG
```

There are no tests: every number here depends on the machine and the scheduler. CI runs the example to check that it terminates.

## What to look for

Output of one run on Windows with 16 logical CPUs. Every number varies between machines and runs:

```text
== 1. Ps, CPUs and goroutines (slides p03-p04, p08) ==
runtime.NumCPU():       16
runtime.GOMAXPROCS(0):  16
runtime.NumGoroutine(): 1

== 2. Goroutines are cheap (slide p02) ==
started 10000 goroutines in 24ms
goroutines alive while parked: 10001
StackInuse grew by 80128 KiB, about 8205 bytes per goroutine
all 10000 finished after close(release): total 25ms

== 3. Asynchronous preemption of a tight loop (slide p07) ==
GOMAXPROCS=1: main woke up and stopped the spinner after 120ms
the spinner looped about 451 million times meanwhile

== 4. Syscall handoff (slide p06) ==
other goroutines finished 15 ticks while the reader was blocked (blocked=true)
reader got "wake up"
threadcreate profile: 18 threads before, 18 after
```

- About 8 KiB per goroutine is the Windows figure. The runtime adds 4 KiB to every stack on Windows, which has no separate signal stack, and rounds 2 KiB + 4 KiB up to a power of two (`runtime/stack.go`). On Linux and macOS the minimum is 2 KiB, so expect a figure near that. An OS thread's stack is measured in megabytes.
- Demo 3 finishes slightly after the 100 ms sleep. That delay is how long the scheduler's monitor thread (`sysmon`) let the spinner run before preempting it. Under `GODEBUG=asyncpreemptoff=1` the program prints the demo 3 heading and then hangs, which proves the loop has no other preemption point.
- `15 ticks ... (blocked=true)`: three goroutines finished all their work on the only P while another goroutine's thread sat in a blocking read. The threadcreate count often doesn't change because the runtime reuses an idle thread for the handoff; it grows only when no idle thread is available.
- A `schedtrace` line looks like this (`[ 47 0 ... ]` are the per-P local run queues):

  ```text
  SCHED 0ms: gomaxprocs=16 idleprocs=14 threads=7 spinningthreads=1 needspinning=0 idlethreads=3 runqueue=0 [ 47 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 ] schedticks=[ 3 0 0 0 0 0 0 0 0 0 0 0 0 23 12 10 ]
  ```

## Notes and corrections

- **Slides p08/p09:** since Go 1.25, when `go.mod` says `go 1.25` or later, the default GOMAXPROCS on Linux respects the container's CPU limit and is updated if the limit changes, so `uber-go/automaxprocs` is no longer needed. `GODEBUG=containermaxprocs=0` and `updatemaxprocs=0` opt out.
- **Slide p02:** 2 KB is the minimum stack on Linux (8 KiB on Windows). Since Go 1.19 the starting size adapts to the average stack usage the runtime has observed, so new goroutines can start larger. Default OS thread stacks are typically 8 MB on Linux and 1 MB on Windows, so "~2 MB" is a ballpark.
- **Slide p05:** besides stealing half of another P's local run queue, the scheduler also checks the global run queue (every 61st schedule, for fairness) and the network poller.
- **Slide p06:** the handoff isn't immediate. A thread entering a syscall keeps its P; `sysmon` takes the P away if the syscall lasts more than about 20 µs. Network I/O doesn't block a thread at all: the goroutine parks on the netpoller (epoll, kqueue or IOCP). File I/O, the slide's example, does block a thread. On Unix, `os.Pipe` reads also go through the netpoller; this example calls `Fd()` to switch the pipe to blocking mode so that the read really occupies a thread.
- **Slide p07:** tight loops without function calls are preemptible since Go 1.14, as demo 3 shows. What remains non-preemptible is mostly runtime-internal code and platforms without signal-based preemption.

## Slides

[031-goroutines.pdf](../../slides/031-goroutines.pdf)
