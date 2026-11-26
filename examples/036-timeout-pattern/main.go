// Command timeout-pattern demonstrates timeouts with select and time.After:
// a leak-free call with a deadline, the context equivalent with causes, the
// Go 1.23 timer garbage collection, a heartbeat watchdog, and a two-stage
// graceful shutdown.
//
// Slides: slides/036-timeout-pattern.pdf
//
// Run:
//
//	go run ./examples/036-timeout-pattern
//	go test ./examples/036-timeout-pattern
package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

func main() {
	section("1. select + time.After, and the buffered result channel (slide p02)")
	timeoutNoLeak()

	section("2. The context version: one derived deadline with a cause (slides p04, p07)")
	contextTimeout()

	section("3. time.After in a loop no longer leaks (Go 1.23+, slides p03, p09)")
	timerGC()

	section("4. Heartbeat watchdog with Reset and no drain (slide p05)")
	watchdogDemo()

	section("5. Two-stage shutdown (slide p06)")
	gracefulShutdownDemo()
}

var errAPITimeout = errors.New("API call timed out")

// callExternalAPI simulates a slow API. The goroutine sends its result after
// latency. If nobody is receiving any more and the channel is unbuffered,
// that send blocks forever: the goroutine leaks. With a buffer of 1 the send
// always succeeds and the goroutine exits, whether or not anyone reads it.
func callExternalAPI(latency time.Duration, buffered bool) <-chan string {
	res := make(chan string)
	if buffered {
		res = make(chan string, 1)
	}
	go func() {
		time.Sleep(latency) // the work itself keeps running after a timeout
		res <- "200 OK"
	}()
	return res
}

// callWithTimeout races the API against the clock (slide p02).
func callWithTimeout(timeout time.Duration, buffered bool) (string, error) {
	select {
	case res := <-callExternalAPI(50*time.Millisecond, buffered):
		return res, nil
	case <-time.After(timeout):
		return "", errAPITimeout
	}
}

func timeoutNoLeak() {
	for _, c := range []struct {
		name     string
		buffered bool
	}{
		{"buffered (cap 1)", true},
		{"unbuffered", false},
	} {
		before := runtime.NumGoroutine()
		_, err := callWithTimeout(10*time.Millisecond, c.buffered)
		// Give the API goroutine time to finish its 50ms of work and exit.
		after := settle(before, 150*time.Millisecond)
		fmt.Printf("%-17s %v; goroutines leaked: %d\n", c.name+":", err, after-before)
	}
}

// settle polls runtime.NumGoroutine until it drops back to want or max
// elapses, and returns the last count.
func settle(want int, maxWait time.Duration) int {
	deadline := time.Now().Add(maxWait)
	for {
		n := runtime.NumGoroutine()
		if n <= want || time.Now().After(deadline) {
			return n
		}
		time.Sleep(5 * time.Millisecond)
	}
}

var (
	errLocalLimit    = errors.New("local operation exceeded its 50ms limit")
	errUserCancelled = errors.New("user cancelled the request")
)

// fetchUser is the nested function: it does not know about any timeout, it
// just passes ctx down. That is what time.After cannot do (slide p04).
func fetchUser(ctx context.Context) error {
	return queryDB(ctx)
}

// queryDB stands in for a call that honours ctx, like database/sql does.
func queryDB(ctx context.Context) error {
	select {
	case <-time.After(200 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// contextTimeout replaces slide p07's three-way select (result, ctx.Done(),
// time.After) with one derived context. WithTimeoutCause records why the
// local limit fired, so context.Cause can tell "my limit" from "my caller
// gave up", while ctx.Err() stays the standard sentinel.
func contextTimeout() {
	run := func(label string, parent context.Context) {
		ctx, cancel := context.WithTimeoutCause(parent, 50*time.Millisecond, errLocalLimit)
		defer cancel()
		err := fetchUser(ctx)
		fmt.Printf("%s:\n  fetchUser:          %v\n  ctx.Err():          %v\n  context.Cause(ctx): %v\n",
			label, err, ctx.Err(), context.Cause(ctx))
	}

	run("local limit fires", context.Background())

	parent, cancelParent := context.WithCancelCause(context.Background())
	time.AfterFunc(10*time.Millisecond, func() { cancelParent(errUserCancelled) })
	run("caller cancels first", parent)
}

// timerGC runs 100,000 selects that each create an unfired one-hour timer.
// Before Go 1.23 every one of them stayed in memory for an hour. Since 1.23
// an unreferenced timer is garbage, fired or not, so the heap stays flat.
// For contrast, the same number of timers that are still referenced.
func timerGC() {
	const n = 100_000
	ready := make(chan struct{})
	close(ready)

	base := heapInuse()
	for range n {
		select {
		case <-ready:
		case <-time.After(time.Hour):
		}
	}
	unreferenced := heapInuse()

	kept := make([]<-chan time.Time, 0, n)
	for range n {
		kept = append(kept, time.After(time.Hour))
	}
	referenced := heapInuse()
	runtime.KeepAlive(kept)

	fmt.Printf("%d time.After(1h) in a loop, after GC: %+.1f MiB heap in use\n", n, mib(unreferenced, base))
	fmt.Printf("%d timers still referenced:            %+.1f MiB heap in use\n", n, mib(referenced, base))
}

func heapInuse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

func mib(now, base uint64) float64 {
	return max(0, float64(now)-float64(base)) / (1 << 20) // GC noise can make it slightly negative
}

// watchdogDemo starts a worker that sends five heartbeats 20ms apart and
// then hangs. The watchdog fires 50ms after the last beat.
func watchdogDemo() {
	const beats = 5
	heartbeat := make(chan struct{})
	go func() {
		for range beats {
			time.Sleep(20 * time.Millisecond)
			heartbeat <- struct{}{}
		}
		// The worker is stuck from here on: no more pulses.
	}()

	start := time.Now()
	err := Watchdog(heartbeat, 50*time.Millisecond)
	fmt.Printf("%d heartbeats, then silence: %v after %v (expected ~150ms)\n",
		beats, err, time.Since(start).Round(10*time.Millisecond))
}

var errCleanupTimeout = errors.New("resource cleanup timed out")

// GracefulShutdown is slide p06's two stages. The slide ends with "Hard
// exit." but only prints; here the function returns an error so the caller
// can actually exit (for example with os.Exit(1)). If closeResources hangs
// forever, its goroutine is abandoned, which is acceptable only because the
// process is about to exit.
func GracefulShutdown(done <-chan struct{}, closeResources func(), drain, cleanup time.Duration) error {
	// Stage 1: wait for ongoing tasks (e.g. an HTTP server drain).
	fmt.Println("Shutting down... draining tasks")
	select {
	case <-done:
		fmt.Println("Tasks finished cleanly")
	case <-time.After(drain):
		fmt.Println("Task drain timed out. Forcing resource cleanup...")
	}

	// Stage 2: clean up resources (e.g. database, cache).
	cleanupDone := make(chan struct{})
	go func() {
		closeResources()
		close(cleanupDone)
	}()

	select {
	case <-cleanupDone:
		fmt.Println("Resources closed cleanly")
		return nil
	case <-time.After(cleanup):
		fmt.Println("Resource cleanup timed out.")
		return errCleanupTimeout
	}
}

func gracefulShutdownDemo() {
	pause := func(d time.Duration) func() { return func() { time.Sleep(d) } }

	fmt.Println("-- healthy service --")
	done := make(chan struct{})
	time.AfterFunc(10*time.Millisecond, func() { close(done) })
	err := GracefulShutdown(done, pause(10*time.Millisecond), 50*time.Millisecond, 30*time.Millisecond)
	fmt.Println("result:", err)

	fmt.Println("-- stuck service --")
	never := make(chan struct{}) // tasks that never finish
	err = GracefulShutdown(never, pause(100*time.Millisecond), 50*time.Millisecond, 30*time.Millisecond)
	fmt.Printf("result: %v (a real main would now call os.Exit(1))\n", err)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
