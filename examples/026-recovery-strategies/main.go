// Command recovery-strategies demonstrates panic and recover: recovering at a
// boundary, the rule that recover must be called directly by the deferred
// function, recovering in goroutines, recovery middleware for HTTP handlers,
// panic(nil), and the failures recover can't catch.
//
// Slides: slides/026-recovery-strategies.pdf
//
// Run:
//
//	go run ./examples/026-recovery-strategies
//	go run ./examples/026-recovery-strategies -repanic   # ends in a panic, exit status 2
//	go run ./examples/026-recovery-strategies -crash     # ends in a fatal error, exit status 2
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

func main() {
	repanic := flag.Bool("repanic", false, "recover, log and re-panic (shows Go 1.25's [recovered, repanicked] output)")
	crash := flag.Bool("crash", false, "end in a fatal error that recover cannot catch (concurrent map writes)")
	flag.Parse()

	switch {
	case *repanic:
		rePanic()
		return
	case *crash:
		fatalConcurrentMapWrites()
		return
	}

	logger := newDemoLogger()

	section("1. recover in a deferred function (slide p03)")
	SafeExecution()
	boundary()

	section("2. recover must be called directly by the deferred function (slide p03)")
	recoverPitfall()

	section("3. Every goroutine needs its own recover (slide p04)")
	goroutines(logger)

	section("4. HTTP: net/http's own recovery vs RecoveryMiddleware (slides p05-p06)")
	httpDemo(logger)

	section("5. panic(nil) is recoverable since Go 1.21 (slide p03)")
	panicNil()

	fmt.Println("\nAlso try -repanic and -crash (both end the process on purpose).")
}

// newDemoLogger writes slog's text format to stdout without timestamps, and
// shortens the "stack" attribute to its size so the demo stays readable.
// A production logger would keep the whole stack.
func newDemoLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch {
			case a.Key == slog.TimeKey && len(groups) == 0:
				return slog.Attr{}
			case a.Key == "stack":
				return slog.String("stack", fmt.Sprintf("<%d bytes, trimmed for the demo>", len(a.Value.String())))
			}
			return a
		},
	}))
}

// ---- 1. SafeExecution and a panic-to-error boundary ---------------------------

// SafeExecution is the slide p03 function. Execution never resumes after the
// panic: the deferred function runs, and SafeExecution returns normally to
// its caller. (The slide's fmt.Println after panic is unreachable, and
// go vet reports it, so it's left out.)
func SafeExecution() {
	defer func() {
		// recover returns the value passed to panic (an any).
		if r := recover(); r != nil {
			fmt.Printf("Rescued from: %v\n", r)
		}
	}()

	fmt.Println("Starting...")
	panic("unrecoverable state") // control jumps to the deferred function
}

// runTask is a boundary: whatever the task does, the caller gets an error
// value, never a panic. The named result is what lets the deferred function
// change what runTask returns.
func runTask(name string, task func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task %s panicked: %v", name, r)
		}
	}()
	task()
	return nil
}

func boundary() {
	fmt.Println("runTask(ok):    ", runTask("ok", func() {}))
	fmt.Println("runTask(divide):", runTask("divide", func() {
		var zero int
		fmt.Println(10 / zero) // runtime error: integer divide by zero
	}))
}

// ---- 2. the helper pitfall -----------------------------------------------------------

// tryRecover calls recover. It works only when it IS the deferred function.
func tryRecover(label string) {
	if r := recover(); r != nil {
		fmt.Printf("%-36s recovered %q\n", label, r)
		return
	}
	fmt.Printf("%-36s recover() returned nil\n", label)
}

func deferHelperDirectly() {
	defer tryRecover("defer tryRecover(...):")
	panic("boom 1")
}

func deferHelperInClosure() {
	// The deferred function is the closure; tryRecover is one call deeper,
	// so its recover returns nil and the panic keeps going.
	defer func() { tryRecover("defer func() { tryRecover(...) }():") }()
	panic("boom 2")
}

// protect is the outer safety net that lets the demo continue.
func protect(f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%-36s caught %q\n", "  outer protective recover:", r)
		}
	}()
	f()
}

func recoverPitfall() {
	protect(deferHelperDirectly)
	protect(deferHelperInClosure)
}

// ---- 3. goroutines ---------------------------------------------------------------------

// safeGo runs fn in a goroutine with its own recover. A recover in main can't
// catch a panic in another goroutine, and an unrecovered goroutine panic ends
// the whole process.
func safeGo(wg *sync.WaitGroup, logger *slog.Logger, name string, fn func()) {
	wg.Go(func() { // Go 1.25: Add(1), go, defer Done() in one call
		defer func() {
			if r := recover(); r != nil {
				logger.Error("goroutine panicked", "worker", name, "panic", r, "stack", string(debug.Stack()))
			}
		}()
		fn()
	})
}

func goroutines(logger *slog.Logger) {
	var wg sync.WaitGroup
	var done atomic.Int32
	for _, name := range []string{"resize", "thumbnail", "upload"} {
		safeGo(&wg, logger, name, func() {
			if name == "thumbnail" {
				panic("decoder: corrupt header")
			}
			done.Add(1)
		})
	}
	wg.Wait()
	fmt.Printf("%d workers finished, 1 panic was logged, and main is still running\n", done.Load())
}

// ---- 4. HTTP ------------------------------------------------------------------------------

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) {
		panic("nil user record")
	})
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	return mux
}

// newServer starts an httptest server on 127.0.0.1 and silences its error
// log: net/http logs every handler panic with a full stack there.
func newServer(h http.Handler) *httptest.Server {
	srv := httptest.NewUnstartedServer(h)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	return srv
}

func get(c *http.Client, url string) string {
	resp, err := c.Get(url)
	if err != nil {
		// The message includes the random port, so print what matters.
		return fmt.Sprintf("request failed, errors.Is(err, io.EOF) = %v", errors.Is(err, io.EOF))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("%d %q", resp.StatusCode, body)
}

func httpDemo(logger *slog.Logger) {
	bare := newServer(newMux())
	defer bare.Close()
	fmt.Println("without middleware:")
	fmt.Println("  GET /panic ->", get(bare.Client(), bare.URL+"/panic"))
	fmt.Println("  GET /ok    ->", get(bare.Client(), bare.URL+"/ok"), "(the server survived)")

	wrapped := newServer(RecoveryMiddleware(logger, newMux()))
	defer wrapped.Close()
	fmt.Println("with RecoveryMiddleware:")
	fmt.Println("  GET /panic ->", get(wrapped.Client(), wrapped.URL+"/panic"))
	fmt.Println("  GET /ok    ->", get(wrapped.Client(), wrapped.URL+"/ok"))
}

// ---- 5. panic(nil) ------------------------------------------------------------------------

func panicNil() {
	defer func() {
		r := recover()
		var pe *runtime.PanicNilError
		if err, ok := r.(error); ok && errors.As(err, &pe) {
			fmt.Printf("recovered %T: %v\n", r, pe)
		}
	}()
	panic(nil) // since Go 1.21 this panics with a *runtime.PanicNilError
}

// ---- 6. opt-in endings ----------------------------------------------------------------------

// rePanic recovers, logs and panics again with the same value (slide p07).
// Since Go 1.25 the crash report says "panic: ... [recovered, repanicked]"
// once instead of printing the panic twice. Re-panic only when continuing is
// unsafe, such as a broken invariant; a lost database connection is an
// error to retry, not a reason to crash.
func rePanic() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("unexpected state, re-panicking", "panic", r) // the slide's log.Critical doesn't exist
			panic(r)
		}
	}()
	panic("ledger invariant violated: debits != credits")
}

// fatalConcurrentMapWrites ends in "fatal error: concurrent map writes". A
// fatal error is not a panic: deferred functions don't run and recover can't
// stop it. Deadlocks, running out of memory and stack exhaustion behave the
// same way. This is deliberately racy code; it only runs behind -crash.
func fatalConcurrentMapWrites() {
	m := map[int]int{}
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			defer func() {
				recover() //nolint:errcheck // intentional: shows that recover can't stop a fatal error
			}()
			for j := 0; ; j++ {
				m[j%1024] = i
			}
		})
	}
	wg.Wait()
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
