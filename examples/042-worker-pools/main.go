// Command worker-pools demonstrates a bounded worker pool: fan-out over a
// jobs channel, a shared results channel closed by a collector, sizing the
// pool for CPU-bound and I/O-bound work, classic fan-in with merge, and an
// early stop with a context.
//
// Slides: slides/042-worker-pools.pdf
//
// Run:
//
//	go run ./examples/042-worker-pools
//	go run ./examples/042-worker-pools -workers=8 -jobs=1000
//	go run -race ./examples/042-worker-pools
//	go test -run='^$' -bench=. ./examples/042-worker-pools
package main

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"slices"
	"time"
)

func main() {
	workers := flag.Int("workers", runtime.GOMAXPROCS(0), "number of workers in the pool")
	jobs := flag.Int("jobs", 1000, "number of jobs to run")
	flag.Parse()
	if *workers < 1 || *jobs < 0 {
		fmt.Println("need -workers >= 1 and -jobs >= 0")
		return
	}

	section("1. A complete pool (slides p03-p07)")
	completePool(*workers, *jobs)

	section("2. Sizing: CPU-bound vs I/O-bound work (slide p08)")
	sizing()

	section("3. Classic fan-in: merge one channel per worker (slide p05)")
	fanInDemo()

	section("4. Early stop with a context")
	earlyStop()
}

func completePool(workers, jobs int) {
	stats := Run(workers, jobs, process)
	want := jobs * (jobs + 1) // sum of 2*j for j = 1..jobs
	fmt.Printf("%d workers, %d jobs: %d results, sum %d (want %d, ok: %t)\n",
		workers, jobs, stats.Results, stats.Sum, want, stats.Sum == want)
	fmt.Printf("jobs per worker: min %d, max %d (varies per run)\n",
		slices.Min(stats.PerWorker), slices.Max(stats.PerWorker))
}

// sizing runs 200 jobs of each kind at three pool sizes and prints the wall
// time. CPU-bound work stops improving at GOMAXPROCS (there are no more
// cores to run on); I/O-bound work keeps improving, because the workers
// spend their time waiting, not computing.
func sizing() {
	const n = 200
	procs := runtime.GOMAXPROCS(0)
	fmt.Printf("%d jobs, GOMAXPROCS = %d (durations vary by machine)\n", n, procs)
	fmt.Printf("%-10s %12s %12s\n", "workers", "CPU-bound", "I/O-bound")
	for _, size := range []int{1, procs, 100} {
		cpu := timeIt(func() { Run(size, n, cpuWork) })
		io := timeIt(func() { Run(size, n, ioWork) })
		fmt.Printf("%-10d %12v %12v\n", size, cpu, io)
	}
}

func timeIt(f func()) time.Duration {
	start := time.Now()
	f()
	return time.Since(start).Round(time.Millisecond)
}

func fanInDemo() {
	const workers, n = 4, 100
	sum := FanIn(workers, n)
	fmt.Printf("%d worker channels merged into one: sum %d (want %d)\n", workers, sum, n*(n+1))
}

// earlyStop gives 1,000 slow jobs a 50ms budget. Workers check ctx.Done()
// while waiting for jobs and while sending results, so they all exit
// promptly instead of finishing the queue.
func earlyStop() {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	stats, err := RunContext(ctx, 4, 1000, ioWork)
	fmt.Printf("4 workers, 1000 jobs of 10ms, 50ms budget: %v after %d results (varies, about 20)\n",
		err, stats.Results)
	fmt.Printf("returned after ~%v; goroutines leaked: %d\n",
		time.Since(start).Round(10*time.Millisecond), settle(before)-before)
}

// settle waits briefly for goroutines that are on their way out, then
// reports runtime.NumGoroutine.
func settle(want int) int {
	for range 100 {
		if n := runtime.NumGoroutine(); n <= want {
			return n
		}
		time.Sleep(time.Millisecond)
	}
	return runtime.NumGoroutine()
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
