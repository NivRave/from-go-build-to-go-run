// Command mastering-defer demonstrates when deferred calls run and what they
// see: LIFO order, eager argument evaluation, decorating named results,
// defer inside loops, entry/exit tracing, and os.Exit skipping defers.
//
// Slides: slides/025-mastering-defer.pdf
//
// Run:
//
//	go run ./examples/025-mastering-defer
//	go run ./examples/025-mastering-defer -exit
//	go test -run='^$' -bench=. -benchmem ./examples/025-mastering-defer
//	go vet -tags vetdemo ./examples/025-mastering-defer
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	exit := flag.Bool("exit", false, "end by calling os.Exit with a deferred call pending")
	flag.Parse()

	section("1. LIFO: last deferred, first run (slide p02)")
	lifo()
	loopLIFO()

	section("2. Arguments are evaluated at the defer statement (slide p04)")
	evalTrap()
	timingTrap()

	section("3. Decorating named results (slide p05)")
	namedResults()

	section("4. Defer in a loop (slide p07)")
	deferInLoop()

	section("5. Entry/exit tracing with one defer (slide p08)")
	ComplexLogic()

	if *exit {
		section("6. os.Exit skips deferred calls (slide p03)")
		exitWithPendingDefer()
	}
}

// ---- 1. LIFO -------------------------------------------------------------------

func lifo() {
	defer fmt.Println("First")  // executed last
	defer fmt.Println("Second") // executed middle
	defer fmt.Println("Third")  // executed first
}

func loopLIFO() {
	// Each iteration pushes one deferred call; they all run when loopLIFO
	// returns, newest first.
	for i := range 3 {
		defer fmt.Println("loop defer", i)
	}
}

// ---- 2. evaluation trap -----------------------------------------------------------

func evalTrap() {
	x := 1
	defer fmt.Println("arg:    ", x)              // x is evaluated now: 1
	defer func() { fmt.Println("closure:", x) }() // x is read when the closure runs: 2
	x = 2
}

// elapsed is time.Since behind a function of our own. The slide's direct
// `defer fmt.Printf("...", time.Since(start))` is such a common mistake that
// go vet rejects it (see vetdemo.go); going through elapsed keeps the same
// behaviour runnable.
func elapsed(start time.Time) time.Duration { return time.Since(start) }

func report(label string, d time.Duration) {
	fmt.Printf("%s %v\n", label, d.Round(time.Millisecond))
}

// timingTrap's output depends on the clock, so no Example asserts it.
func timingTrap() {
	start := time.Now()
	defer func() { report("closure, evaluated at exit: ", elapsed(start)) }()
	defer report("argument, evaluated at defer:", elapsed(start))

	time.Sleep(50 * time.Millisecond)
}

// ---- 3. named results ----------------------------------------------------------------

var (
	errDiskFull    = errors.New("disk full")
	errCloseFailed = errors.New("close: device busy")
)

// dbExecute stands in for the slide's db.Execute.
func dbExecute(fail bool) error {
	if fail {
		return errDiskFull
	}
	return nil
}

// Save decorates its error in a deferred function, which is the only place
// that runs after the return value is set. Unlike slide p05 it does not also
// log the error: the caller handles it, so one failure is one log entry
// (Part 022, "handle or return").
func Save(fail bool) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("save: %w", err)
		}
	}()
	return dbExecute(fail)
}

// resource is a fake file whose Close can fail.
type resource struct{ closeErr error }

func (r *resource) Close() error { return r.closeErr }

// writeReport folds the deferred Close error into its named result with
// errors.Join, so neither the write error nor the close error is lost. A
// plain `defer r.Close()` would silently drop the close error, which for a
// written file can mean the data never reached the disk.
func writeReport(writeErr, closeErr error) (err error) {
	r := &resource{closeErr: closeErr}
	defer func() {
		err = errors.Join(err, r.Close())
	}()
	return writeErr
}

func namedResults() {
	fmt.Println("Save ok:      ", Save(false))
	err := Save(true)
	fmt.Println("Save failing: ", err, "| errors.Is(err, errDiskFull):", errors.Is(err, errDiskFull))

	fmt.Println("write ok, close ok:     ", writeReport(nil, nil))
	fmt.Println("write ok, close fails:  ", writeReport(nil, errCloseFailed))
	err = writeReport(errDiskFull, errCloseFailed)
	fmt.Printf("both fail (joined):      %q\n", err.Error())
}

// ---- 4. defer in a loop --------------------------------------------------------------

// openTracker counts open files so the demo can report the peak.
type openTracker struct{ open, peak int }

func (t *openTracker) opened() {
	t.open++
	t.peak = max(t.peak, t.open)
}

func (t *openTracker) closed() { t.open-- }

// readAllDeferInLoop is the slide's anti-pattern, with the os.Open error
// handled: every Close waits until the function returns, so all the files
// are open at once. With enough files that is a file-descriptor leak
// ("too many open files").
func readAllDeferInLoop(paths []string, t *openTracker) error {
	var errs []error
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		t.opened()
		defer func() { f.Close(); t.closed() }() // runs at function exit, not iteration end
		// ... read f ...
	}
	return errors.Join(errs...)
}

// readOne scopes the defer to one file. Unlike the slide's anonymous-function
// fix, it returns the open error instead of silently skipping the file.
func readOne(path string, t *openTracker) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	t.opened()
	defer func() { f.Close(); t.closed() }() // runs when readOne returns
	// ... read f ...
	return nil
}

func readAllWithHelper(paths []string, t *openTracker) error {
	var errs []error
	for _, p := range paths {
		errs = append(errs, readOne(p, t)) // errors.Join drops the nils
	}
	return errors.Join(errs...)
}

func deferInLoop() {
	dir, err := os.MkdirTemp("", "025-defer-in-loop-*")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer os.RemoveAll(dir)

	const n = 5
	paths := make([]string, n)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("file-%d.txt", i))
		if err := os.WriteFile(paths[i], []byte("data"), 0o600); err != nil {
			fmt.Println("error:", err)
			return
		}
	}

	var bad, good openTracker
	errBad := readAllDeferInLoop(paths, &bad)
	errGood := readAllWithHelper(paths, &good)
	fmt.Printf("defer in loop:   peak open files = %d (err = %v)\n", bad.peak, errBad)
	fmt.Printf("helper per file: peak open files = %d (err = %v)\n", good.peak, errGood)
}

// ---- 5. tracing ------------------------------------------------------------------------

// trace runs immediately, because it is an argument of the deferred call;
// un runs at exit. One line instruments both ends of the function.
func trace(s string) string {
	fmt.Println("entering:", s)
	return s
}

func un(s string) {
	fmt.Println("leaving: ", s)
}

func ComplexLogic() {
	defer un(trace("ComplexLogic"))
	fmt.Println("  ... actual work ...")
}

// ---- 6. os.Exit ---------------------------------------------------------------------------

// exitWithPendingDefer shows that os.Exit ends the process immediately:
// deferred calls do not run (nor do they for log.Fatal, which calls
// os.Exit). They do run on return, on panic and on runtime.Goexit.
func exitWithPendingDefer() {
	defer fmt.Println("deferred: this line is never printed")
	fmt.Println("calling os.Exit(0) with one deferred call pending")
	os.Exit(0)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
