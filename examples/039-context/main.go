// Command context demonstrates context.Context as the stop signal and the
// carrier of request-scoped values: a cancellation tree, a leak-free worker,
// typed value keys, cancellation causes, AfterFunc and WithoutCancel, and a
// simulated slow query that honours its deadline.
//
// Slides: slides/039-context.pdf
//
// Run:
//
//	go run ./examples/039-context
//	go test ./examples/039-context
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
)

func main() {
	section("1. The cancellation tree (slide p04)")
	cancellationTree()

	section("2. A context-aware worker that cannot leak (slide p05)")
	workerDemo()

	section("3. Values the right way: an unexported key type (slide p06)")
	values()

	section("4. Why it was cancelled: WithCancelCause, WithTimeoutCause (Go 1.20, 1.21)")
	causes()

	section("5. context.AfterFunc and context.WithoutCancel (Go 1.21)")
	afterFuncAndWithoutCancel()

	section("6. A slow query with a deadline (slides p07, p08)")
	slowQuery()
}

// cancellationTree builds:
//
//	root (WithCancel)
//	├── short  (WithTimeout 50ms) ── short/g1, short/g2
//	└── manual (WithCancel)       ── manual/g1, manual/g2
//
// Each grandchild runs in a goroutine that reports when its Done channel
// closes. Reports are sorted because goroutines finish in any order.
func cancellationTree() {
	root, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	short, cancelShort := context.WithTimeout(root, 50*time.Millisecond)
	defer cancelShort()
	manual, cancelManual := context.WithCancel(root)
	defer cancelManual()

	reports := make(chan string)
	var wg sync.WaitGroup
	for _, child := range []struct {
		name string
		ctx  context.Context
	}{{"short", short}, {"manual", manual}} {
		for i := range 2 {
			grandchild, cancel := context.WithCancel(child.ctx)
			wg.Go(func() {
				defer cancel()
				<-grandchild.Done()
				reports <- fmt.Sprintf("  %s/g%d stopped: %v", child.name, i+1, grandchild.Err())
			})
		}
	}
	collect := func(n int) {
		lines := make([]string, 0, n)
		for range n {
			lines = append(lines, <-reports)
		}
		slices.Sort(lines)
		for _, l := range lines {
			fmt.Println(l)
		}
	}

	fmt.Println("short's 50ms timeout fires:")
	collect(2)
	fmt.Printf("  root.Err() = %v, manual.Err() = %v (a child ending does not reach its parent or siblings)\n",
		root.Err(), manual.Err())

	fmt.Println("cancelRoot():")
	cancelRoot()
	collect(2)
	fmt.Printf("  manual.Err() = %v (cancellation flows downstream)\n", manual.Err())
	wg.Wait()
}

// worker is slide p05 with the missing ok check. Without it, a closed jobs
// channel makes `j := <-jobs` return 0 immediately, forever, and the worker
// spins calling process(0) until someone cancels ctx.
func worker(ctx context.Context, jobs <-chan int, process func(int)) {
	for {
		select {
		case <-ctx.Done():
			// The signal to stop has arrived
			return
		case j, ok := <-jobs:
			if !ok {
				return // no more work will ever come
			}
			process(j)
		}
	}
}

// workerDemo stops three workers twice: once by closing jobs, once by
// cancelling ctx while jobs stays open. Either way every goroutine exits.
func workerDemo() {
	run := func(label string, stop func(jobs chan int, cancel context.CancelFunc)) {
		before := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		jobs := make(chan int)
		var processed atomic.Int64
		var wg sync.WaitGroup
		for range 3 {
			wg.Go(func() { worker(ctx, jobs, func(int) { processed.Add(1) }) })
		}
		running := runtime.NumGoroutine()
		for j := range 10 {
			jobs <- j
		}
		stop(jobs, cancel)
		wg.Wait()
		fmt.Printf("%-26s processed %d jobs; goroutines before %d, running %d, after %d\n",
			label+":", processed.Load(), before, running, settle(before))
	}
	run("stopped by close(jobs)", func(jobs chan int, _ context.CancelFunc) { close(jobs) })
	run("stopped by cancel()", func(_ chan int, cancel context.CancelFunc) { cancel() })
}

// settle waits briefly for goroutines that have called wg.Done but not yet
// returned, then reports runtime.NumGoroutine.
func settle(want int) int {
	for range 100 {
		if n := runtime.NumGoroutine(); n <= want {
			return n
		}
		time.Sleep(time.Millisecond)
	}
	return runtime.NumGoroutine()
}

// Use an unexported type for keys to prevent collisions with other packages.
type ctxKey int

const (
	userKey ctxKey = iota
	traceKey
)

// WithUser is the setter that belongs with UserFrom: callers never touch
// the key, so only this package can store or read it.
func WithUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// UserFrom is slide p06's FromContext, named after what it returns.
func UserFrom(ctx context.Context) (string, bool) {
	u, ok := ctx.Value(userKey).(string)
	return u, ok
}

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey, id)
}

func TraceIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(traceKey).(string)
	return id, ok
}

// otherKey stands in for another package's key type. Its value 0 is the same
// number as userKey, but keys compare by type and value, so it can't collide.
type otherKey int

func values() {
	ctx := WithTraceID(WithUser(context.Background(), "alice"), "abc-123")

	u, ok := UserFrom(ctx)
	fmt.Printf("UserFrom(ctx)          = %q, %t\n", u, ok)
	id, ok := TraceIDFrom(ctx)
	fmt.Printf("TraceIDFrom(ctx)       = %q, %t\n", id, ok)
	u, ok = UserFrom(context.Background())
	fmt.Printf("UserFrom(Background()) = %q, %t\n", u, ok)
	fmt.Printf("ctx.Value(otherKey(0)) = %v (same number, different type: no collision)\n", ctx.Value(otherKey(0)))
}

var (
	errPaymentDeclined = errors.New("payment declined")
	errDBTooSlow       = errors.New("database took longer than 10ms")
)

// causes shows that ctx.Err() only ever says "canceled" or "deadline
// exceeded", while context.Cause carries the reason the canceller gave.
func causes() {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errPaymentDeclined)
	fmt.Printf("WithCancelCause:  Err() = %v; Cause = %v\n", ctx.Err(), context.Cause(ctx))

	ctx, cancelTimeout := context.WithTimeoutCause(context.Background(), 10*time.Millisecond, errDBTooSlow)
	defer cancelTimeout()
	<-ctx.Done()
	fmt.Printf("WithTimeoutCause: Err() = %v; Cause = %v\n", ctx.Err(), context.Cause(ctx))

	ctx, cancelPlain := context.WithCancel(context.Background())
	cancelPlain()
	fmt.Printf("WithCancel:       Err() = %v; Cause = %v (no cause given: same as Err)\n", ctx.Err(), context.Cause(ctx))
}

// afterFuncAndWithoutCancel: AfterFunc runs cleanup in its own goroutine
// when a request is cancelled. WithoutCancel detaches follow-up work, such
// as an audit log, from the request's cancellation but keeps its values.
// It is the explicit exception to "cancelling the parent stops everything".
func afterFuncAndWithoutCancel() {
	reqCtx, cancelReq := context.WithCancel(WithUser(context.Background(), "alice"))

	cleaned := make(chan struct{})
	stopCleanup := context.AfterFunc(reqCtx, func() {
		fmt.Println("AfterFunc: request ended, releasing its DB connection")
		close(cleaned)
	})
	stopUnused := context.AfterFunc(reqCtx, func() { fmt.Println("never printed") })
	fmt.Println("stop() before cancel:", stopUnused(), "(unregistered, will not run)")

	cancelReq()
	<-cleaned
	fmt.Println("stop() after it ran: ", stopCleanup(), "(already started)")

	audit := context.WithoutCancel(reqCtx)
	user, _ := UserFrom(audit)
	_, err := SlowDatabaseQuery(audit) // the request is gone; the audit write still completes
	fmt.Printf("request Err() = %v; audit Err() = %v; audit log for %q written, err = %v\n",
		reqCtx.Err(), audit.Err(), user, err)
}

// queryLatency is how long the simulated database takes to answer.
const queryLatency = 100 * time.Millisecond

// SlowDatabaseQuery stands in for a context-aware driver call such as
// db.QueryRowContext. It returns as soon as ctx is done, without waiting for
// the result.
func SlowDatabaseQuery(ctx context.Context) (string, error) {
	select {
	case <-time.After(queryLatency):
		return "3 rows", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// processRequest is slides p07-p08. defer cancel() releases the context's
// timer as soon as the function returns. Forgetting it does not leak forever:
// when the deadline fires, the context cancels itself and frees everything.
// It only holds those resources until then. `go vet`'s lostcancel check
// reports a cancel func that is never called.
func processRequest(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // Always release resources!

	res, err := SlowDatabaseQuery(ctx)
	fmt.Printf("timeout %-6v -> res = %q, err = %v\n", timeout, res, err)
}

func slowQuery() {
	processRequest(2 * time.Second) // the slide's budget: the 100ms query fits
	processRequest(50 * time.Millisecond)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
