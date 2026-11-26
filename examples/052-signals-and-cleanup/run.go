package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var errAlreadyClosed = errors.New("fake db: already closed")

// config holds the timings that differ between the simulated demo and -wait.
type config struct {
	workers       int
	signalDelay   time.Duration // default mode: when the simulated signal fires
	zombieTimeout time.Duration // how long to wait for the workers
}

// run is the whole program. It owns every defer, so they all run before main
// decides the exit code; os.Exit in main would skip them otherwise (slide
// p08). It takes its context, arguments and output as parameters, so tests
// call it with a cancelled context instead of sending real signals.
func run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("signals-and-cleanup", flag.ContinueOnError)
	fs.SetOutput(out)
	wait := fs.Bool("wait", false, "wait for a real Ctrl+C instead of simulating one")
	if err := fs.Parse(args); err != nil {
		return err
	}
	defer fmt.Fprintln(out, "run: deferred calls ran before main chose the exit code")

	cfg := config{workers: 3, signalDelay: 200 * time.Millisecond, zombieTimeout: 500 * time.Millisecond}

	section(out, "1. Start workers; one will hang (slide p03)")
	var stop context.CancelFunc
	if *wait {
		// The real thing. NotifyContext registers its own buffered channel,
		// so signals are never dropped (slide p06). Slide p06 itself does
		// not compile, because it declares sigs twice:
		//
		//	sigs := make(chan os.Signal)
		//	sigs := make(chan os.Signal, 1) // compile error: no new variables on left side of :=
		//
		// and passing the unbuffered channel to signal.Notify fails go vet:
		// "misuse of unbuffered os.Signal channel as argument to signal.Notify".
		// signal.Notify never blocks, so it drops a signal nobody is ready
		// to receive.
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		cfg.zombieTimeout = 5 * time.Second // time to try the second Ctrl+C
		fmt.Fprintln(out, "press Ctrl+C to start a graceful shutdown")
	} else {
		// Simulate the signal by cancelling the context, exactly what
		// NotifyContext does when one arrives. This works on every OS and in
		// tests; Windows cannot send os.Interrupt to its own process.
		var cancel context.CancelCauseFunc
		ctx, cancel = context.WithCancelCause(ctx)
		timer := time.AfterFunc(cfg.signalDelay, func() { cancel(errors.New("simulated SIGINT")) })
		defer timer.Stop()
		stop = func() { cancel(nil) }
		fmt.Fprintf(out, "simulating a signal in %v (use -wait for a real Ctrl+C)\n", cfg.signalDelay)
	}
	defer stop()

	// In production a zombie worker stays stuck until the process exits. The
	// demo releases it when run returns so tests do not leak goroutines.
	release := make(chan struct{})
	defer close(release)
	pool := startWorkers(ctx, cfg.workers, release)
	fmt.Fprintf(out, "workers: %d started (worker %d ignores ctx)\n", cfg.workers, cfg.workers)

	res := newResources(out)

	<-ctx.Done()
	section(out, "2. Signal received: stop the workers, with a safety net (slides p02, p03)")
	fmt.Fprintf(out, "signal: %v\n", context.Cause(ctx))
	if *wait {
		// Two-stage Ctrl+C (slide p02): stop() unregisters NotifyContext's
		// channel. Nothing else is registered for SIGINT, so a second Ctrl+C
		// now gets Go's default handler and kills the process at once.
		stop()
		fmt.Fprintf(out, "graceful shutdown started; press Ctrl+C again within %v to force-quit\n", cfg.zombieTimeout)
	}

	stopped, ok := pool.wait(cfg.zombieTimeout)
	if ok {
		fmt.Fprintf(out, "workers: all %d stopped\n", stopped)
	} else {
		fmt.Fprintf(out, "workers: %d of %d stopped\n", stopped, cfg.workers)
		fmt.Fprintf(out, "zombie timeout: gave up after %v, proceeding with cleanup\n", cfg.zombieTimeout)
	}

	section(out, "3. Idempotent teardown with sync.OnceValue (slide p05)")
	// Two shutdown paths race to clean up, as happens when a signal handler
	// and a failing request both decide to stop the service.
	var callers sync.WaitGroup
	var errSignal, errMain error
	callers.Go(func() { errSignal = res.teardown() })
	callers.Go(func() { errMain = res.teardown() })
	callers.Wait()
	fmt.Fprintf(out, "signal path got: %v\n", errSignal)
	fmt.Fprintf(out, "main path got:   %v\n", errMain)
	fmt.Fprintf(out, "teardown body ran %d time(s)\n", res.runs.Load())
	fmt.Fprintf(out, "without Once, a second close(events) panics: %v\n", closeTwice())

	section(out, "4. Release the signal handler (slide p04)")
	if *wait {
		// stop() already called signal.Stop on NotifyContext's channel.
		// signal.Reset restores the default for these signals even if some
		// other package also registered a channel for them.
		signal.Reset(os.Interrupt, syscall.SIGTERM)
		fmt.Fprintln(out, "signal.Reset(os.Interrupt, syscall.SIGTERM): default handling restored")
	} else {
		fmt.Fprintln(out, "default mode registered no handler; with -wait, stop() calls signal.Stop and run then calls signal.Reset here")
	}

	section(out, "5. Return to main (slide p08)")
	return nil
}

// workerPool tracks the workers and how many have stopped.
type workerPool struct {
	wg      sync.WaitGroup
	n       int
	stopped atomic.Int32
}

// startWorkers starts n workers. The last one ignores ctx and blocks on
// release, like a goroutine stuck on a resource that cleanup already closed.
func startWorkers(ctx context.Context, n int, release <-chan struct{}) *workerPool {
	p := &workerPool{n: n}
	for i := range n {
		p.wg.Go(func() {
			if i == n-1 {
				<-release // the zombie: never looks at ctx
			} else {
				<-ctx.Done()
			}
			p.stopped.Add(1)
		})
	}
	return p
}

// wait is wg.Wait with a safety net (slide p03: never call wg.Wait without
// one). It reports how many workers stopped and whether all of them did.
func (p *workerPool) wait(timeout time.Duration) (stopped int, ok bool) {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return p.n, true
	case <-timer.C:
		return int(p.stopped.Load()), false
	}
}

// fakeDB returns an error on a second Close, as many resources do.
// (*sql.DB is an exception: its Close is already safe to call twice.)
type fakeDB struct{ closed atomic.Bool }

func (db *fakeDB) Close() error {
	if db.closed.Swap(true) {
		return errAlreadyClosed
	}
	return nil
}

// resources are passed explicitly rather than kept in package globals
// (Part 047). teardown is built once with sync.OnceValue: the body runs
// exactly once, and every caller gets the same error value back.
type resources struct {
	db     *fakeDB
	events chan string
	runs   atomic.Int32

	teardown func() error
}

func newResources(out io.Writer) *resources {
	r := &resources{db: &fakeDB{}, events: make(chan string, 8)}
	r.teardown = sync.OnceValue(func() error {
		r.runs.Add(1)
		fmt.Fprintln(out, "teardown: closing events channel")
		close(r.events) // a second close would panic
		fmt.Fprintln(out, "teardown: closing db")
		return r.db.Close()
	})
	return r
}

// closeTwice shows what sync.OnceValue prevents. The panic is recovered so
// the demo can continue.
func closeTwice() (msg any) {
	defer func() { msg = recover() }()
	ch := make(chan struct{})
	close(ch)
	close(ch)
	return nil
}

func section(out io.Writer, title string) {
	fmt.Fprintf(out, "\n== %s ==\n", title)
}
