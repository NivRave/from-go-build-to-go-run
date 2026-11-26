// Command timers-and-tickers demonstrates time.Timer, time.Ticker and
// time.AfterFunc: why a timer in a select beats time.Sleep, how a ticker
// drops ticks for a slow handler, slow-operation warnings with AfterFunc,
// and reusing one timer with Reset under the Go 1.23 timer semantics.
//
// Slides: slides/041-timers-and-tickers.pdf
//
// Run:
//
//	go run ./examples/041-timers-and-tickers
//	go test ./examples/041-timers-and-tickers
//	go test -run='^$' -bench=. -benchmem ./examples/041-timers-and-tickers
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func main() {
	section("1. time.Sleep vs a timer in a select (slides p02-p04)")
	sleepVsTimer()

	section("2. A ticker loop, and a handler slower than the interval (slides p05-p06)")
	tickerDemo()

	section("3. time.AfterFunc and context.AfterFunc (slide p07)")
	afterFuncDemo()

	section("4. Reusing one timer with Reset, no drain (Go 1.23+, slides p04, p08)")
	timerReuse()
}

// waitWithSleep cannot be interrupted: it notices ctx only after the full
// sleep.
func waitWithSleep(ctx context.Context, d time.Duration) error {
	time.Sleep(d)
	return ctx.Err()
}

// waitWithTimer is slide p04 without the drain. Since Go 1.23, Stop
// guarantees that no stale value is received afterwards, and here the timer
// is thrown away anyway, so `if !timer.Stop() { <-timer.C }` is unnecessary.
func waitWithTimer(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sleepVsTimer runs both waits side by side, each meant to last 2s, with a
// context that is cancelled after 300ms.
func sleepVsTimer() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	var wg sync.WaitGroup
	lines := make([]string, 2)
	for i, w := range []struct {
		name string
		wait func(context.Context, time.Duration) error
	}{
		{"time.Sleep(2s)", waitWithSleep},
		{"timer + select", waitWithTimer},
	} {
		wg.Go(func() {
			err := w.wait(ctx, 2*time.Second)
			lines[i] = fmt.Sprintf("%-15s returned after %v: %v",
				w.name+":", time.Since(start).Round(100*time.Millisecond), err)
		})
	}
	wg.Wait()
	for _, l := range lines {
		fmt.Println(l)
	}
}

// tickerLoop is slide p06: a ticker inside for-select, stopped by ctx. It
// returns how many ticks it handled. If both cases are ready at once, select
// picks at random, so after receiving a tick it checks ctx again: shutdown
// wins over one more round of work.
func tickerLoop(ctx context.Context, interval time.Duration, handle func(time.Time)) int {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	handled := 0
	for {
		select {
		case <-ctx.Done():
			return handled // Graceful exit
		case t := <-ticker.C:
			if ctx.Err() != nil {
				return handled
			}
			handled++
			handle(t)
		}
	}
}

// tickerDemo runs the loop for ~550ms with 100ms ticks, once with an
// instant handler and once with a 250ms handler. A ticker holds at most one
// pending tick; the rest are dropped while the handler is busy.
func tickerDemo() {
	run := func(work time.Duration) int {
		ctx, cancel := context.WithTimeout(context.Background(), 550*time.Millisecond)
		defer cancel()
		return tickerLoop(ctx, 100*time.Millisecond, func(time.Time) { time.Sleep(work) })
	}

	var fast, slow int
	var wg sync.WaitGroup
	wg.Go(func() { fast = run(0) })
	wg.Go(func() { slow = run(250 * time.Millisecond) })
	wg.Wait()
	fmt.Printf("instant handler: %d ticks in 550ms\n", fast)
	fmt.Printf("250ms handler:   %d ticks in 550ms (the others were dropped, not queued)\n", slow)
}

// withSlowWarning is slide p07: log a warning if the work takes longer than
// threshold, without blocking the work itself. Stop reports whether it
// prevented the function from running; false means it had already started.
func withSlowWarning(name string, threshold, work time.Duration) {
	timer := time.AfterFunc(threshold, func() {
		fmt.Printf("  warning: %s is exceeding %v\n", name, threshold)
	})

	time.Sleep(work) // performHeavyWork()

	if timer.Stop() {
		fmt.Printf("  %s done in time: Stop() = true, the warning never ran\n", name)
	} else {
		fmt.Printf("  %s done: Stop() = false, the warning had already started\n", name)
	}
}

func afterFuncDemo() {
	fmt.Println("time.AfterFunc, threshold 100ms:")
	withSlowWarning("250ms job", 100*time.Millisecond, 250*time.Millisecond)
	withSlowWarning("20ms job", 100*time.Millisecond, 20*time.Millisecond)

	// context.AfterFunc (Go 1.21) is the same idea, triggered by
	// cancellation instead of a duration.
	fmt.Println("context.AfterFunc:")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		fmt.Println("  ctx cancelled: closing the connection")
		close(done)
	})
	cancel()
	<-done
	fmt.Println("  stop() =", stop(), "(already ran)")
}

// timerReuse keeps one timer for a whole idle-timeout loop. Each iteration
// calls Reset directly: no Stop, no drain. Then it checks the Go 1.23
// guarantee: Reset on a timer that fired but was never received discards
// the old value, so the next receive waits for the new duration.
func timerReuse() {
	timer := time.NewTimer(time.Hour)
	events := make(chan int)
	go func() {
		for i := range 3 {
			time.Sleep(20 * time.Millisecond)
			events <- i + 1
		}
	}()

loop:
	for {
		timer.Reset(50 * time.Millisecond)
		select {
		case e := <-events:
			fmt.Printf("event %d: timer reset\n", e)
		case <-timer.C:
			fmt.Println("no event for 50ms: idle timeout")
			break loop
		}
	}

	timer.Reset(10 * time.Millisecond)
	time.Sleep(30 * time.Millisecond) // the timer fires; nobody receives
	start := time.Now()
	timer.Reset(50 * time.Millisecond)
	<-timer.C
	fmt.Printf("Reset after an unreceived expiry: next value after ~%v (no stale value)\n",
		time.Since(start).Round(10*time.Millisecond))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
