// Command dependency-injection demonstrates manual dependency injection in
// Go: a consumer-defined interface, a decorator, a validating constructor
// with functional options, and explicit wiring in main.
//
// Slides: slides/047-dependency-injection.pdf
//
// Run:
//
//	go run ./examples/047-dependency-injection
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"
)

func main() {
	section("1. A consumer-defined Store, MemStore and a LoggingStore decorator (slide p03)")
	decoratorDemo()

	section("2. A validating constructor with functional options (slides p04-p05)")
	constructorDemo()

	section("3. Explicit wiring in main, the composition root (slide p07)")
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// newLogger writes text logs to stdout without the time attribute, so the
// output is identical on every run.
func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	}))
}

func decoratorDemo() {
	ctx := context.Background()
	var store Store = NewLoggingStore(NewMemStore(), newLogger())

	_ = store.Save(ctx, "greeting", []byte("hello"))
	data, err := store.Load(ctx, "greeting")
	fmt.Printf("Load(greeting) = %q, err = %v\n", data, err)
	_, err = store.Load(ctx, "missing")
	fmt.Printf("Load(missing): errors.Is(err, ErrNotFound) = %t\n", errors.Is(err, ErrNotFound))
}

func constructorDemo() {
	_, err := NewService(nil)
	fmt.Printf("NewService(nil): err = %v\n", err)

	_, err = NewService(NewMemStore(), WithClock(nil))
	fmt.Printf("NewService(store, WithClock(nil)): err = %v\n", err)

	svc, err := NewService(NewMemStore()) // defaults: discard logger, time.Now
	fmt.Printf("NewService(store): ok = %t, err = %v\n", svc != nil, err)
}

// run is the composition root (slide p07). Every concrete type is chosen and
// connected here, in plain code that the compiler checks: swap MemStore for a
// database-backed store and nothing else changes. Compare the slide's
// http.ListenAndServe(":80", ...): it discards the returned error, runs with
// no timeouts (see Part 043), and port 80 needs privileges on Unix.
//
// Code generators such as github.com/google/wire (archived by Google in
// August 2025) write a function like this one for you. For most programs,
// writing it by hand stays shorter and clearer.
func run(ctx context.Context) error {
	logger := newLogger()

	// Low-level infrastructure first.
	mem := NewMemStore()
	store := NewLoggingStore(mem, logger)

	// A fixed clock keeps the demo's timestamps stable. A real main would
	// leave WithClock out and get time.Now.
	clock := func() time.Time { return time.Date(2026, 11, 26, 9, 30, 0, 0, time.UTC) }

	svc, err := NewService(store, WithLogger(logger), WithClock(clock))
	if err != nil {
		return err
	}

	if _, err := svc.CreateNote(ctx, "n1", "wire dependencies in main"); err != nil {
		return err
	}
	n, err := svc.GetNote(ctx, "n1")
	if err != nil {
		return err
	}
	fmt.Printf("GetNote(n1) = %q created %s\n", n.Text, n.CreatedAt.Format(time.RFC3339))

	_, err = svc.GetNote(ctx, "n2")
	fmt.Printf("GetNote(n2): err = %v, errors.Is(err, ErrNotFound) = %t\n", err, errors.Is(err, ErrNotFound))
	return nil
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
