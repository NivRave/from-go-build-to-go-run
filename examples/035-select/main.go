// Command select demonstrates the select statement: random choice between
// ready cases, load shedding with default, the nil-channel trick, a true
// single-select bridge, and a daemon that shuts down cleanly instead of
// parking forever in select{}.
//
// Slides: slides/035-select.pdf
//
// Run:
//
//	go run ./examples/035-select
//	go run ./examples/035-select -wait       # daemon waits for Ctrl+C
//	go run ./examples/035-select -deadlock   # select{} with nothing else to run
//	go test -run='^$' -bench=Select ./examples/035-select
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	wait := flag.Bool("wait", false, "daemon demo: wait for Ctrl+C instead of a 200ms fallback timeout")
	deadlock := flag.Bool("deadlock", false, "run select{} with no other goroutines (ends in a fatal error)")
	flag.Parse()

	if *deadlock {
		emptySelectDeadlock()
		return
	}

	section("1. Ready cases are chosen at random (slide p02)")
	randomChoice()

	section("2. default: load shedding (slide p03)")
	loadShedding()

	section("3. The nil-channel trick: merging two producers (slide p04)")
	mergeNil()

	section("4. A true single-select bridge (slide p06)")
	bridgeDemo()

	section("5. A daemon without select{} (slide p07)")
	daemon(*wait)
}

// randomChoice runs a select over two channels that are always ready. If
// select were "first come, first served" or picked the first listed case,
// one side would win every time; instead the runtime picks uniformly at
// random, which is what prevents one busy channel from starving the other.
func randomChoice() {
	chan1 := make(chan int, 1)
	chan2 := make(chan int, 1)
	chan1 <- 1
	chan2 <- 2

	const rounds = 10_000
	counts := map[int]int{}
	for range rounds {
		select {
		case msg1 := <-chan1:
			counts[msg1]++
			chan1 <- msg1 // refill so both cases stay ready
		case msg2 := <-chan2:
			counts[msg2]++
			chan2 <- msg2
		}
	}
	fmt.Printf("chan1 won %d times, chan2 won %d times out of %d (about 50/50, varies per run)\n",
		counts[1], counts[2], rounds)
}

// loadShedding offers a burst of 20 tasks to a queue of 5. The worker is held
// until the burst is over, so exactly 5 tasks fit and the other 15 hit the
// default case instead of blocking the caller.
func loadShedding() {
	ch := make(chan int, 5)
	release := make(chan struct{})
	processed := make(chan int)

	go func() {
		<-release // a busy worker: it cannot take anything during the burst
		n := 0
		for range ch {
			n++
		}
		processed <- n
	}()

	queued, dropped := 0, 0
	for task := range 20 {
		select {
		case ch <- task:
			queued++
		default:
			// Buffer full! Drop the request to save the system.
			dropped++
		}
	}
	close(release)
	close(ch)
	fmt.Printf("burst of 20: %d queued, %d shed\n", queued, dropped)
	fmt.Printf("worker processed %d tasks after the burst\n", <-processed)
}

// mergeNil merges two producers that finish at different times. A receive
// from a nil channel blocks forever, so a nil case never fires: setting a
// closed channel to nil switches its case off, and the loop ends when both
// are nil.
func mergeNil() {
	ch1 := produce(1, 2, 3)
	ch2 := produce(10, 20, 30, 40, 50)

	var fromCh1, fromCh2 []int
	for ch1 != nil || ch2 != nil {
		select {
		case v, ok := <-ch1:
			if !ok {
				ch1 = nil // This case is now disabled
				continue
			}
			fromCh1 = append(fromCh1, v)
		case v, ok := <-ch2:
			if !ok {
				ch2 = nil
				continue
			}
			fromCh2 = append(fromCh2, v)
		}
	}
	fmt.Println("from ch1:", fromCh1)
	fmt.Println("from ch2:", fromCh2)
	fmt.Println("both channels are nil: loop finished")
}

// produce sends vals on a new channel, then closes it.
func produce(vals ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, v := range vals {
			out <- v
		}
	}()
	return out
}

// nestedBridge is the slide p06 version. It is correct, but it is not a
// "single select that sends and receives": while the inner select waits on
// out <- v, nothing is receiving from in.
func nestedBridge(in <-chan int, out chan<- int, done <-chan struct{}) {
	for {
		select {
		case v, ok := <-in:
			if !ok {
				return // Input closed
			}
			select {
			case out <- v:
			case <-done:
				return
			}
		case <-done:
			return
		}
	}
}

// bridge keeps receiving while out is blocked. Values wait in pending, and
// the send case is only enabled (sendCh = out) when something is pending.
// When in closes, its case is switched off with the nil-channel trick and the
// bridge drains pending before returning.
func bridge(in <-chan int, out chan<- int, done <-chan struct{}) {
	var pending []int
	for in != nil || len(pending) > 0 {
		var sendCh chan<- int // nil: the send case is disabled
		var next int
		if len(pending) > 0 {
			sendCh = out
			next = pending[0]
		}
		select {
		case v, ok := <-in:
			if !ok {
				in = nil
				continue
			}
			pending = append(pending, v)
		case sendCh <- next:
			pending = pending[1:]
		case <-done:
			return
		}
	}
}

// bridgeDemo runs both bridges against the same scenario: a producer sends
// five values while the consumer is not reading yet. Only the single-select
// bridge lets the producer finish.
func bridgeDemo() {
	type bridgeFunc func(in <-chan int, out chan<- int, done <-chan struct{})
	for _, b := range []struct {
		name string
		run  bridgeFunc
	}{
		{"nested selects (slide)", nestedBridge},
		{"single select + nil send case", bridge},
	} {
		in := make(chan int)
		out := make(chan int)
		done := make(chan struct{})
		go func() {
			b.run(in, out, done)
			close(out)
		}()

		producerDone := make(chan struct{})
		go func() {
			for v := range 5 {
				in <- v + 1
			}
			close(in)
			close(producerDone)
		}()

		finished := false
		select {
		case <-producerDone:
			finished = true
		case <-time.After(200 * time.Millisecond): // long enough for 5 hand-offs
		}

		// Now start consuming, so both variants run to completion.
		var got []int
		for v := range out {
			got = append(got, v)
		}
		<-producerDone
		close(done)
		fmt.Printf("%-30s producer finished before the consumer started: %-5t got %v\n",
			b.name+":", finished, got)
	}
}

// daemon replaces the slide's select{} with a context that is cancelled by
// Ctrl+C (or SIGTERM), so main can tell the worker to stop and wait for it.
// Without -wait, a 200ms timeout stands in for the signal so the demo exits.
func daemon(wait bool) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !wait {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
	} else {
		fmt.Println("daemon running; press Ctrl+C to stop")
	}

	workerDone := make(chan struct{})
	go backgroundWorker(ctx, workerDone)

	<-ctx.Done() // sleeps at 0% CPU, like select{}, but can be woken
	<-workerDone // let the worker finish its cleanup before main returns
	fmt.Printf("main: shutdown requested (%v), worker stopped, exiting cleanly\n", context.Cause(ctx))
}

// backgroundWorker separates data from control (slide p05): the ticker is
// the data stream, ctx.Done() the control signal.
func backgroundWorker(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// periodic work would go here
		case <-ctx.Done():
			fmt.Println("worker: cleaning up")
			return
		}
	}
}

// emptySelectDeadlock is slide p07 with nothing else left to run: select{}
// blocks forever, and because no other goroutine can ever wake main, the
// runtime aborts with "all goroutines are asleep - deadlock!".
func emptySelectDeadlock() {
	fmt.Println("calling select{} with no other goroutines...")
	select {}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
