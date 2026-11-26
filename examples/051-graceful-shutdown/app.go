package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errDBClosed    = errors.New("db is closed")
	errQueueClosed = errors.New("job queue is closed")
)

// FakeDB stands in for a real connection pool. Saving after Close fails,
// which is exactly what happens if you tear things down in the wrong order.
type FakeDB struct {
	mu     sync.Mutex
	closed bool
	saved  int
}

func (db *FakeDB) Save(int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return errDBClosed
	}
	db.saved++
	return nil
}

func (db *FakeDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return errDBClosed
	}
	db.closed = true
	return nil
}

// Stats reports how many rows were saved and whether Close has run.
func (db *FakeDB) Stats() (saved int, closed bool) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.saved, db.closed
}

// jobQueue is the hand-off from HTTP handlers to the worker pool. Close is
// guarded by the same mutex as Push, so a handler still running after a
// forced server.Close cannot send on a closed channel (which would panic).
type jobQueue struct {
	mu     sync.Mutex
	ch     chan int64
	closed bool
}

func newJobQueue(size int) *jobQueue { return &jobQueue{ch: make(chan int64, size)} }

func (q *jobQueue) Push(id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errQueueClosed
	}
	select {
	case q.ch <- id:
		return nil
	default:
		return errors.New("job queue is full")
	}
}

func (q *jobQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}

// App is a small service with the three layers from slide p06: an HTTP entry
// point, background workers, and a database. Run tears them down in the
// reverse order of start-up.
type App struct {
	Listener net.Listener
	DB       *FakeDB
	Logger   *log.Logger

	Workers         int
	SlowDelay       time.Duration // how long POST /orders takes
	ShutdownTimeout time.Duration // limit for server.Shutdown
	WorkerTimeout   time.Duration // limit for waiting on the workers

	nextID atomic.Int64
	queue  *jobQueue
}

// NewApp returns an App with the defaults used by the demo. The listener is
// passed in so tests can use an in-memory one.
func NewApp(ln net.Listener, out io.Writer) *App {
	return &App{
		Listener: ln,
		DB:       &FakeDB{},
		Logger:   log.New(out, "app: ", 0),
		Workers:  3,

		SlowDelay: time.Second,
		// Keep this below the orchestrator's grace period. Kubernetes waits
		// terminationGracePeriodSeconds (30 s by default) before SIGKILL, so a
		// real service would use 20-25 s, not 30 s. The demo uses 5 s.
		ShutdownTimeout: 5 * time.Second,
		WorkerTimeout:   2 * time.Second,
	}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		id := a.nextID.Add(1)
		select {
		case <-time.After(a.SlowDelay):
		case <-r.Context().Done():
			// Only reached when Shutdown gave up and server.Close() cut the
			// connection: the request is lost, which is what draining avoids.
			a.Logger.Printf("handler: order %d abandoned: %v", id, context.Cause(r.Context()))
			return
		}
		if err := a.queue.Push(id); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		a.Logger.Printf("handler: order %d processed, replying 200", id)
		fmt.Fprintf(w, "order %d accepted", id)
	})
	return mux
}

// Run serves until ctx is cancelled, then shuts down in order:
//
//  1. log why (context.Cause),
//  2. stop the HTTP server and drain in-flight requests (server.Shutdown),
//     forcing server.Close() if the drain exceeds ShutdownTimeout,
//  3. close the job queue and wait for the workers, with a timeout,
//  4. close the DB,
//  5. say goodbye.
//
// Each step runs synchronously. Slide p07 runs Shutdown in a goroutine that
// nobody waits for, so main can return (and the process exit) mid-drain.
func (a *App) Run(ctx context.Context) error {
	a.queue = newJobQueue(64)

	// Workers drain the queue until it is closed, rather than stopping on
	// ctx: requests that are still in flight when the signal arrives will
	// enqueue jobs, and those must be saved before the DB closes.
	var wg sync.WaitGroup
	for range a.Workers {
		wg.Go(func() {
			for id := range a.queue.ch {
				if err := a.DB.Save(id); err != nil {
					a.Logger.Printf("worker: order %d: %v", id, err)
				}
			}
		})
	}

	srv := &http.Server{
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(a.Listener) }()
	a.Logger.Printf("serving, %d workers started", a.Workers)

	var errs []error
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		// Serve failed on its own (for example, the listener broke). Still
		// run the rest of the teardown.
		errs = append(errs, fmt.Errorf("serve: %w", err))
		serveErr <- err
	}

	// 1. Why are we stopping? With signal.NotifyContext, Go 1.26 sets a cause
	// that names the signal; the demo sets its own with WithCancelCause.
	a.Logger.Printf("shutdown started: %v", context.Cause(ctx))

	// 2. Stop the entry point. Shutdown closes the listener at once, so
	// ListenAndServe/Serve returns ErrServerClosed immediately; the program
	// must wait for Shutdown itself, not for Serve, to know the drain is over.
	// The shutdown context is derived with WithoutCancel because ctx is
	// already cancelled.
	a.Logger.Printf("[1/4] http: no new connections, draining in-flight requests (limit %v)", a.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.ShutdownTimeout)
	defer cancel()
	switch err := srv.Shutdown(shutdownCtx); {
	case err == nil:
		a.Logger.Print("[1/4] http: drained")
	case errors.Is(err, context.DeadlineExceeded):
		a.Logger.Print("[1/4] http: drain timed out, forcing server.Close()")
		errs = append(errs, fmt.Errorf("http shutdown: %w", err), srv.Close())
	default:
		errs = append(errs, fmt.Errorf("http shutdown: %w", err))
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) && err != nil {
		a.Logger.Printf("[1/4] http: serve: %v", err)
	}

	// 3. No more jobs can arrive; let the workers finish what is queued.
	a.queue.Close()
	if waitTimeout(&wg, a.WorkerTimeout) {
		a.Logger.Printf("[2/4] workers: all %d stopped", a.Workers)
	} else {
		a.Logger.Printf("[2/4] workers: still busy after %v, continuing without them", a.WorkerTimeout)
		errs = append(errs, errors.New("workers did not stop in time"))
	}

	// 4. Close the DB last: workers and handlers needed it until now. Log
	// and continue on error instead of log.Fatalf, which would skip deferred
	// calls and any remaining cleanup.
	saved, _ := a.DB.Stats()
	if err := a.DB.Close(); err != nil {
		a.Logger.Printf("[3/4] db: close: %v", err)
		errs = append(errs, err)
	} else {
		a.Logger.Printf("[3/4] db: closed (orders saved: %d)", saved)
	}

	a.Logger.Print("[4/4] goodbye")
	return errors.Join(errs...)
}

// waitTimeout waits for wg, but gives up after d so a stuck worker cannot
// hang the shutdown forever (more on this in Part 052).
func waitTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
