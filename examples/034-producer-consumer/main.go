// Command producer-consumer demonstrates the producer/consumer pattern: a
// producer that owns and closes its channel, a pool of consumers, backpressure
// from a bounded buffer, the difference between closing and cancelling, and a
// graceful shutdown on Ctrl+C.
//
// Slides: slides/034-producer-consumer.pdf
//
// Run:
//
//	go run ./examples/034-producer-consumer
//	go run ./examples/034-producer-consumer -wait   # stop with Ctrl+C
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	wait := flag.Bool("wait", false, "run demo 5 until Ctrl+C instead of stopping after 300 ms")
	flag.Parse()

	section("1. The producer owns the channel (slides p03, p07)")
	single()

	section("2. A pool of consumers (slides p04-p05)")
	pool()

	section("3. Backpressure from a bounded buffer (slide p06)")
	backpressure()

	section("4. Close drains, cancel stops (slide p08)")
	closeVersusCancel()

	section("5. Graceful shutdown (slide p07)")
	shutdown(*wait)
}

// produce is slides p03 and p07 combined: it creates the channel, fills it
// from its own goroutine, closes it when done, and stops early if ctx is
// cancelled. The buffer gives bursts some room (slide p03).
func produce(ctx context.Context, items []string) <-chan string {
	out := make(chan string, 10) // Buffered for burst room
	go func() {
		defer close(out) // Signal consumers we are done
		for _, item := range items {
			select {
			case out <- item:
			case <-ctx.Done():
				return // Stop producing on cancellation
			}
		}
	}()
	return out
}

// consume processes items until the channel is closed and drained, or until
// ctx is cancelled. An item already being worked on is always finished. It
// returns how many items it processed.
func consume(ctx context.Context, in <-chan string, work time.Duration) int {
	n := 0
	for {
		select {
		case <-ctx.Done():
			return n
		case _, ok := <-in:
			if !ok {
				return n // closed and drained
			}
			// select picks at random when both cases are ready, so check
			// again rather than start new work after a cancel.
			if ctx.Err() != nil {
				return n
			}
			time.Sleep(work) // Simulated work
			n++
		}
	}
}

func items(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("job-%02d", i+1)
	}
	return out
}

func single() {
	var got []string
	for item := range produce(context.Background(), []string{"a", "b", "c", "d", "e"}) {
		got = append(got, item)
	}
	fmt.Println("received in order:", strings.Join(got, " "))
	fmt.Println("range ended because the producer closed the channel")
}

// pool starts numWorkers consumers with wg.Go, so consume needs no
// *sync.WaitGroup parameter (compare slide p05).
func pool() {
	const numWorkers, numItems = 3, 12
	ctx := context.Background()
	jobs := produce(ctx, items(numItems))

	var total atomic.Int32
	var wg sync.WaitGroup
	start := time.Now()
	for range numWorkers {
		wg.Go(func() {
			total.Add(int32(consume(ctx, jobs, 10*time.Millisecond)))
		})
	}
	wg.Wait()
	fmt.Printf("%d workers processed %d items (10 ms each) in about %v\n",
		numWorkers, total.Load(), time.Since(start).Round(10*time.Millisecond))
}

// backpressure pairs a producer that could send instantly with two slow
// consumers and a buffer of 3. The buffer fills, and from then on the
// producer can only send as fast as the consumers take items out.
func backpressure() {
	const numItems = 12
	ch := make(chan string, 3)
	start := time.Now()
	producerDone := make(chan time.Duration, 1)
	go func() {
		defer close(ch)
		for _, item := range items(numItems) {
			ch <- item // blocks while the buffer is full
		}
		producerDone <- time.Since(start)
	}()

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { consume(context.Background(), ch, 20*time.Millisecond) })
	}

	samples := make([]int, 0, 8)
	for range 8 {
		time.Sleep(10 * time.Millisecond)
		samples = append(samples, len(ch))
	}
	wg.Wait()
	fmt.Printf("len(ch) every 10 ms: %v (cap %d)\n", samples, cap(ch))
	fmt.Printf("the producer needed about %v to hand over %d items it could have sent at once\n",
		(<-producerDone).Round(10*time.Millisecond), numItems)
}

// runPipeline runs produce with workers consumers, each spending work per
// item, and returns how many items were processed in total.
func runPipeline(ctx context.Context, all []string, workers int, work time.Duration) int {
	jobs := produce(ctx, all)
	var total atomic.Int32
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() { total.Add(int32(consume(ctx, jobs, work))) })
	}
	wg.Wait()
	return int(total.Load())
}

func closeVersusCancel() {
	all := items(20)

	// Never cancelled: the producer closes the channel after the last item,
	// and the consumers still drain everything that was queued.
	n := runPipeline(context.Background(), all, 2, 10*time.Millisecond)
	fmt.Printf("close:            processed %d of %d\n", n, len(all))

	// Cancelled after 25 ms: the producer stops, and the consumers finish
	// their current item and leave the rest of the buffer untouched.
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	n = runPipeline(ctx, all, 2, 10*time.Millisecond)
	fmt.Printf("cancel at 25 ms:  processed %d of %d\n", n, len(all))
}

// generate produces jobs until ctx is cancelled.
func generate(ctx context.Context) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for i := 1; ; i++ {
			select {
			case out <- fmt.Sprintf("job-%d", i):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// shutdown runs an endless pipeline until Ctrl+C. Unless wait is set, a
// 300 ms timeout stands in for the signal so the demo ends on its own.
func shutdown(wait bool) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if wait {
		fmt.Println("running until you press Ctrl+C...")
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
	}

	jobs := generate(ctx)
	var total atomic.Int32
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() { total.Add(int32(consume(ctx, jobs, 10*time.Millisecond))) })
	}
	wg.Wait() // every consumer has finished its current item

	reason := "Ctrl+C"
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		reason = "300 ms fallback timeout"
	}
	fmt.Printf("stopped by %s: %d jobs processed, all 3 workers exited cleanly\n", reason, total.Load())
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
