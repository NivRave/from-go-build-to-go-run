package main

import (
	"context"
	"sync"
	"time"
)

// Result records which worker handled which job.
type Result struct {
	Worker, Job, Value int
}

// Stats summarises a pool run.
type Stats struct {
	Results   int   // number of results received
	Sum       int   // sum of all result values
	PerWorker []int // jobs handled by each worker (index = id - 1)
}

func (s *Stats) add(r Result) {
	s.Results++
	s.Sum += r.Value
	s.PerWorker[r.Worker-1]++
}

// process is the slide's "work": double the job.
func process(j int) int { return j * 2 }

// worker is slide p04: a loop over jobs that ends when jobs is closed. It
// takes the work function as a parameter so the same pool can run CPU-bound
// and I/O-bound jobs.
func worker(id int, jobs <-chan int, results chan<- Result, work func(int) int) {
	for j := range jobs {
		results <- Result{Worker: id, Job: j, Value: work(j)}
	}
}

// Run is the complete orchestrator that slide p07 leaves unfinished:
//
//   - workers are started with sync.WaitGroup.Go (Go 1.25), and w is
//     captured directly: since Go 1.22 each iteration has its own w;
//   - a producer goroutine sends jobs 1..n and then closes jobs, which is
//     what ends every worker's range loop;
//   - a collector goroutine waits for all workers, then closes results;
//   - the caller ranges over results until it is closed.
//
// Both channels are bounded (cap = workers). That is the backpressure: the
// producer can never get more than a few jobs ahead of the workers.
func Run(workers, n int, work func(int) int) Stats {
	jobs := make(chan int, workers)
	results := make(chan Result, workers)

	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Go(func() { worker(w, jobs, results, work) })
	}

	go func() { // producer
		defer close(jobs)
		for j := 1; j <= n; j++ {
			jobs <- j
		}
	}()

	go func() { // collector
		wg.Wait()      // Block until all workers finish
		close(results) // Safe to close; no more senders
	}()

	stats := Stats{PerWorker: make([]int, workers)}
	for r := range results {
		stats.add(r)
	}
	return stats
}

// ctxWorker is worker with an early exit: it stops when ctx is done, both
// while waiting for a job and while waiting to deliver a result, so a
// cancelled pool never leaves a worker blocked on a full results channel.
func ctxWorker(ctx context.Context, id int, jobs <-chan int, results chan<- Result, work func(int) int) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}
			// select picks at random among ready cases, so a job can win
			// against ctx.Done(). Check again before starting the work.
			if ctx.Err() != nil {
				return
			}
			select {
			case results <- Result{Worker: id, Job: j, Value: work(j)}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// RunContext is Run with cancellation. When ctx is done, the producer stops
// sending, the workers stop, and RunContext returns what was finished so far
// together with ctx.Err().
func RunContext(ctx context.Context, workers, n int, work func(int) int) (Stats, error) {
	jobs := make(chan int, workers)
	results := make(chan Result, workers)

	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Go(func() { ctxWorker(ctx, w, jobs, results, work) })
	}

	go func() {
		defer close(jobs)
		for j := 1; j <= n; j++ {
			select {
			case jobs <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	stats := Stats{PerWorker: make([]int, workers)}
	for r := range results {
		stats.add(r)
	}
	return stats, ctx.Err()
}

// merge is classic fan-in: N input channels become one output channel,
// which is closed once every input is closed and drained.
func merge(cs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	for _, c := range cs {
		wg.Go(func() {
			for v := range c {
				out <- v
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// FanIn runs n jobs on workers that each have their own output channel, and
// merges those channels into one. It returns the sum of the results.
func FanIn(workers, n int) int {
	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for j := 1; j <= n; j++ {
			jobs <- j
		}
	}()

	outs := make([]<-chan int, workers)
	for w := range workers {
		out := make(chan int)
		outs[w] = out
		go func() {
			defer close(out) // each worker owns, and closes, its own channel
			for j := range jobs {
				out <- process(j)
			}
		}()
	}

	sum := 0
	for v := range merge(outs...) {
		sum += v
	}
	return sum
}

// cpuWork is a short CPU-bound job: a xorshift hash loop with nothing to
// wait for.
func cpuWork(j int) int {
	h := uint64(j) | 1
	for range 200_000 {
		h ^= h << 13
		h ^= h >> 7
		h ^= h << 17
	}
	return int(h & 0xffff)
}

// ioWork stands in for an I/O-bound job, such as a network call: almost all
// of its time is spent waiting.
func ioWork(j int) int {
	time.Sleep(10 * time.Millisecond)
	return j
}
