// Command goroutines makes the Go scheduler's M:P:G model observable: the
// processor count, how cheap goroutines are, asynchronous preemption of a
// tight loop, and the handoff of a P away from a thread blocked in a syscall.
//
// Slides: slides/031-goroutines.pdf
//
// Run:
//
//	go run ./examples/031-goroutines
//	GODEBUG=schedtrace=1000 go run ./examples/031-goroutines
//
// With GODEBUG=asyncpreemptoff=1, demo 3 never finishes (by design).
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	section("1. Ps, CPUs and goroutines (slides p03-p04, p08)")
	counts()

	section("2. Goroutines are cheap (slide p02)")
	cheapGoroutines(10_000)

	section("3. Asynchronous preemption of a tight loop (slide p07)")
	preemption()

	section("4. Syscall handoff (slide p06)")
	syscallHandoff()
}

func counts() {
	fmt.Println("runtime.NumCPU():      ", runtime.NumCPU())
	// Since Go 1.25 the default respects a container's CPU limit on Linux,
	// so this can be lower than NumCPU inside Docker or Kubernetes.
	fmt.Println("runtime.GOMAXPROCS(0): ", runtime.GOMAXPROCS(0))
	fmt.Println("runtime.NumGoroutine():", runtime.NumGoroutine())
}

// cheapGoroutines starts n goroutines that all park on one channel, measures
// the stack memory they hold while parked, then releases them.
func cheapGoroutines(n int) {
	var before, during runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()
	release := make(chan struct{})
	var ready, wg sync.WaitGroup
	ready.Add(n)
	for range n {
		wg.Go(func() {
			ready.Done()
			<-release // park until main closes the channel
		})
	}
	ready.Wait()
	started := time.Since(start)

	runtime.ReadMemStats(&during)
	alive := runtime.NumGoroutine()
	close(release)
	wg.Wait()

	stack := int64(during.StackInuse) - int64(before.StackInuse)
	fmt.Printf("started %d goroutines in %v\n", n, started.Round(time.Millisecond))
	fmt.Printf("goroutines alive while parked: %d\n", alive)
	fmt.Printf("StackInuse grew by %d KiB, about %d bytes per goroutine\n", stack/1024, stack/int64(n))
	fmt.Printf("all %d finished after close(release): total %v\n", n, time.Since(start).Round(time.Millisecond))
}

// preemption runs a loop with no function calls on the only P. main can set
// stop only if the scheduler takes the P away from the spinning goroutine,
// which since Go 1.14 it does with a signal (asynchronous preemption). With
// GODEBUG=asyncpreemptoff=1 this function hangs.
func preemption() {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	var stop atomic.Bool
	var spins int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() { // no calls, so no cooperative preemption point
			spins++
		}
	}()

	start := time.Now()
	time.Sleep(100 * time.Millisecond) // the spinner now owns the only P
	stop.Store(true)                   // reached only because the spinner was preempted
	<-done
	fmt.Printf("GOMAXPROCS=1: main woke up and stopped the spinner after %v\n", time.Since(start).Round(10*time.Millisecond))
	fmt.Printf("the spinner looped %s times meanwhile\n", roughly(spins))
}

// syscallHandoff blocks one goroutine in a read(2)-style syscall on a pipe
// while GOMAXPROCS is 1, and checks that other goroutines still run. They can
// only run if the runtime detaches the P from the blocked thread and gives it
// to another thread.
func syscallHandoff() {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	r, w, err := os.Pipe()
	if err != nil {
		fmt.Println("pipe:", err)
		return
	}
	defer r.Close()
	defer w.Close()
	// On Unix, os.Pipe is non-blocking and reads park on the netpoller, which
	// would not block a thread at all. Fd() switches the descriptor to
	// blocking mode, so the Read below really sits in a syscall. (On Windows
	// pipe reads block a thread anyway.)
	_ = r.Fd()

	threadsBefore := pprof.Lookup("threadcreate").Count()

	var blocked atomic.Bool
	got := make(chan string)
	go func() {
		buf := make([]byte, 16)
		blocked.Store(true)
		n, err := r.Read(buf) // the thread (M) running this blocks in the OS
		blocked.Store(false)
		if err != nil {
			got <- "error: " + err.Error()
			return
		}
		got <- string(buf[:n])
	}()
	time.Sleep(20 * time.Millisecond) // let the reader enter the syscall

	var ticks atomic.Int32
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for range 5 {
				time.Sleep(5 * time.Millisecond)
				ticks.Add(1)
			}
		})
	}
	wg.Wait()
	fmt.Printf("other goroutines finished %d ticks while the reader was blocked (blocked=%v)\n", ticks.Load(), blocked.Load())

	if _, err := w.Write([]byte("wake up")); err != nil {
		fmt.Println("write:", err)
		return
	}
	fmt.Printf("reader got %q\n", <-got)
	fmt.Printf("threadcreate profile: %d threads before, %d after\n",
		threadsBefore, pprof.Lookup("threadcreate").Count())
}

// roughly rounds a large count for display, e.g. "about 300 million".
func roughly(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("about %.1f billion", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("about %d million", n/1_000_000)
	default:
		return fmt.Sprint(n)
	}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
