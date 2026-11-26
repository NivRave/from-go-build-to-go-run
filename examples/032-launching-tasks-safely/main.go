// Command launching-tasks-safely demonstrates how to start goroutines and know
// when they are done: the lost-work problem, sync.WaitGroup (classic and
// Go 1.25's wg.Go), per-iteration loop variables, errgroup with a limit, and
// a context-aware worker that doesn't leak.
//
// Slides: slides/032-launching-tasks-safely.pdf
//
// Run:
//
//	go run ./examples/032-launching-tasks-safely
//	go vet -tags vetdemo ./examples/032-launching-tasks-safely
package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	section("1. The race to the exit (slide p02)")
	lostWork()

	section("2. sync.WaitGroup: Add/Done/Wait vs. wg.Go (slides p03-p04, Go 1.25)")
	classicWaitGroup()
	modernWaitGroup()

	section("3. Loop variables are per iteration (slide p05, Go 1.22)")
	loopVariables()

	section("4. errgroup with WithContext and SetLimit (slides p06, p08)")
	errgroupDemo()

	section("5. A worker with an exit path (slide p07)")
	contextWorker()
}

// fireAndForget starts five goroutines and returns without waiting, like
// slide p02's main. Nothing tells the caller when they finish.
func fireAndForget(finished *atomic.Int32) {
	for range 5 {
		go func() {
			time.Sleep(10 * time.Millisecond)
			finished.Add(1)
		}()
	}
}

func lostWork() {
	var finished atomic.Int32
	fireAndForget(&finished)
	fmt.Println("Exiting...")
	// Had this been main, the program would end here and the work would be
	// lost. The count is usually 0, but nothing guarantees any value.
	fmt.Printf("goroutines finished by then: %d of 5\n", finished.Load())
}

// worker is slide p03's worker, with a shorter sleep. defer wg.Done() covers
// early returns; it does not rescue a panic, which crashes the whole program
// unless something recovers it.
func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Worker %d starting\n", id)
	time.Sleep(10 * time.Millisecond)
	fmt.Printf("Worker %d done\n", id)
}

func classicWaitGroup() {
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1) // before the go statement (slide p04)
		go worker(i, &wg)
	}
	wg.Wait() // Block until counter is 0
	fmt.Println("classic: all workers done")
}

// work no longer needs a *sync.WaitGroup: wg.Go does Add(1), go and
// defer Done() for it, so the "Add before go" rule can't be broken.
func work(id int) {
	time.Sleep(10 * time.Millisecond)
	fmt.Printf("task %d done\n", id)
}

func modernWaitGroup() {
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Go(func() { work(i) })
	}
	wg.Wait()
	fmt.Println("wg.Go: all tasks done")
}

// loopVariables captures the loop variable directly. Since Go 1.22 (for
// modules whose go.mod says go 1.22 or later) each iteration has its own
// val, so every goroutine sees a different value. Before 1.22 they could all
// print the last one.
func loopVariables() {
	data := []string{"alpha", "beta", "gamma", "delta"}

	var (
		mu  sync.Mutex
		got []string
		wg  sync.WaitGroup
	)
	for _, val := range data {
		wg.Go(func() {
			mu.Lock()
			defer mu.Unlock()
			got = append(got, val)
		})
	}
	wg.Wait()

	slices.Sort(got) // goroutines finish in any order
	fmt.Println("captured:", got)
	fmt.Println("all distinct:", len(slices.Compact(slices.Clone(got))) == len(data))
}

var errRefused = errors.New("connection refused")

// fetch pretends to download url. "bad.example" fails quickly; the others
// take 50 ms unless ctx is cancelled first.
func fetch(ctx context.Context, url string) error {
	if url == "bad.example" {
		time.Sleep(10 * time.Millisecond)
		return fmt.Errorf("fetch %s: %w", url, errRefused)
	}
	select {
	case <-time.After(50 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func errgroupDemo() {
	urls := []string{"a.example", "b.example", "bad.example", "c.example", "d.example"}
	results := make([]error, len(urls)) // each goroutine writes only its own index

	var running, peak atomic.Int32
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(2) // at most 2 fetches at once; g.Go blocks until a slot frees
	for i, url := range urls {
		// No `url := url` needed since Go 1.22.
		g.Go(func() error {
			storeMax(&peak, running.Add(1))
			defer running.Add(-1)

			results[i] = fetch(ctx, url)
			return results[i]
		})
	}
	err := g.Wait() // the first non-nil error; ctx is cancelled when it happens

	for i, url := range urls {
		status := "ok"
		switch {
		case errors.Is(results[i], context.Canceled):
			status = "cancelled"
		case results[i] != nil:
			status = "failed"
		}
		fmt.Printf("%-12s %s\n", url, status)
	}
	fmt.Println("peak concurrency:", peak.Load())
	fmt.Println("Wait returned:", err)
}

// storeMax raises *v to n if n is larger, safely under concurrency.
func storeMax(v *atomic.Int32, n int32) {
	for {
		cur := v.Load()
		if n <= cur || v.CompareAndSwap(cur, n) {
			return
		}
	}
}

// contextWorker is slide p07's "RIGHT" version: a ticker loop that also
// listens on ctx.Done(), so cancelling the context ends the goroutine.
func contextWorker() {
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	var cleanups atomic.Int32
	done := make(chan struct{})
	go func(ctx context.Context) {
		defer close(done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop() // good hygiene; since Go 1.23 not needed to free the ticker

		for {
			select {
			case <-ticker.C:
				cleanups.Add(1) // doBackgroundCleanup()
			case <-ctx.Done():
				return // Exit cleanly when context is cancelled
			}
		}
	}(ctx)

	time.Sleep(35 * time.Millisecond)
	during := runtime.NumGoroutine()
	cancel()
	<-done

	fmt.Printf("goroutines: %d before, %d while the worker runs\n", before, during)
	fmt.Printf("cleanups run before cancel: %d\n", cleanups.Load())
	fmt.Printf("back to %d or fewer after cancel: %v\n", before, settlesAt(before))
}

// settlesAt reports whether runtime.NumGoroutine drops to at most n within a
// second. A goroutine that has closed done may still be a moment away from
// exiting, so one immediate check could be flaky.
func settlesAt(n int) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
