// Command anatomy-of-channels demonstrates unbuffered and buffered channels,
// directional channel types, closing and the comma-ok idiom, broadcasting by
// closing a channel, and how nil channels behave.
//
// Slides: slides/033-anatomy-of-channels.pdf
//
// Run:
//
//	go run ./examples/033-anatomy-of-channels
//	go test -run='^$' -bench=. -benchmem ./examples/033-anatomy-of-channels
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	section("1. Unbuffered: the send waits for the receiver (slide p02)")
	unbuffered()

	section("2. Buffered: sends succeed until the buffer is full (slides p03-p04)")
	buffered()

	section("3. Directional channels, close and comma-ok (slides p05-p06)")
	directional()

	section("4. close(done) as a broadcast (slide p07)")
	broadcast(100)

	section("5. Nil channels (slide p06)")
	nilChannels()
}

// unbuffered is slide p02's program with the receiver arriving 50 ms late.
// It records when the receiver became ready and when the send completed. The
// goroutine doesn't print "Sent!" itself: main waits for it and prints, so
// the line can't be lost when main returns.
func unbuffered() {
	ch := make(chan string)
	var sentAt time.Time
	var wg sync.WaitGroup
	start := time.Now()
	wg.Go(func() {
		ch <- "data" // Blocks until main() is ready
		sentAt = time.Now()
	})

	time.Sleep(50 * time.Millisecond) // the receiver is late
	readyAt := time.Now()
	fmt.Println(<-ch) // Blocks until goroutine sends
	wg.Wait()         // without this, "Sent!" might never be reported

	fmt.Println("Sent!")
	// A receive on an unbuffered channel is synchronized before the send
	// completes, so sentAt can't be earlier than readyAt.
	fmt.Println("send completed after the receiver arrived:", !sentAt.Before(readyAt))
	fmt.Printf("sender was blocked for about %v\n", sentAt.Sub(start).Round(10*time.Millisecond))
}

func buffered() {
	// Create a channel with a buffer size of 2
	ch := make(chan string, 2)

	// These happen immediately; no receiver needed yet
	ch <- "message 1"
	ch <- "message 2"
	fmt.Printf("len=%d cap=%d: buffer is full, but the sender was not blocked\n", len(ch), cap(ch))

	// A third plain send would block forever here (no receiver), so ask
	// without blocking instead.
	select {
	case ch <- "message 3":
		fmt.Println("third send succeeded")
	default:
		fmt.Println("third send would block")
	}

	fmt.Println(<-ch) // Takes "message 1", buffer now has 1 slot free
	fmt.Printf("len=%d cap=%d\n", len(ch), cap(ch))
}

// producer may only send; closing is its job because only the sender knows
// when there is no more data.
func producer(ch chan<- int) {
	defer close(ch) // Always ensure closure to unblock receivers
	for i := range 5 {
		ch <- i
	}
}

// consumer may only receive, so it can't close the channel by mistake:
//
//	close(ch) // compile error: invalid operation: cannot close receive-only channel ch (variable of type <-chan int)
func consumer(ch <-chan int) {
	// The 'range' loop automatically exits when the channel is closed
	// AND all existing values have been consumed.
	for val := range ch {
		fmt.Printf("Received: %d\n", val)
	}
	fmt.Println("Channel closed, cleaning up...")
}

func directional() {
	ch := make(chan int)
	go producer(ch)
	consumer(ch)

	// Receiving from a closed, drained channel never blocks: it yields the
	// zero value at once, and ok tells you that it isn't real data.
	val, ok := <-ch
	fmt.Printf("after close: val=%d ok=%v\n", val, ok)
}

// broadcast parks n goroutines on one channel and wakes them all with a single
// close. Each listener blocks in a receive; slide p07's select with a default
// case would instead busy-poll and burn CPU while waiting.
func broadcast(n int) {
	done := make(chan struct{})
	var exited atomic.Int32
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			<-done // wakes up when done is closed
			exited.Add(1)
		})
	}

	close(done) // To shut everything down
	wg.Wait()
	fmt.Printf("%d of %d goroutines exited after one close(done)\n", exited.Load(), n)
}

func nilChannels() {
	var ch chan int // nil: declared but never made

	// Sending to or receiving from a nil channel blocks forever. The default
	// case shows it without hanging the program.
	select {
	case ch <- 1:
		fmt.Println("sent")
	default:
		fmt.Println("send to nil channel: would block forever")
	}
	select {
	case v := <-ch:
		fmt.Println("received", v)
	default:
		fmt.Println("receive from nil channel: would block forever")
	}

	// Closing a nil channel panics; recover it so the demo continues.
	func() {
		defer func() {
			fmt.Println("close(nil channel) panicked:", recover())
		}()
		close(ch)
	}()
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
