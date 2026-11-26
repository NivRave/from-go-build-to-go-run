package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ---- 6. errgroup ------------------------------------------------------------------

var errUnavailable = errors.New("503 service unavailable")

// fetch simulates a request. URLs containing "bad" fail after 10 ms; the
// others would take a second, but give up as soon as ctx is cancelled.
func fetch(ctx context.Context, url string) error {
	if err := ctx.Err(); err != nil {
		return err // cancelled before we even started
	}
	if strings.Contains(url, "bad") {
		time.Sleep(10 * time.Millisecond)
		return fmt.Errorf("fetch %s: %w", url, errUnavailable)
	}
	select {
	case <-time.After(time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// outcomes collects per-URL results from several goroutines.
type outcomes struct {
	mu   sync.Mutex
	rows []string
}

func (o *outcomes) add(url string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rows = append(o.rows, fmt.Sprintf("  %-22s %v", url, err))
}

func (o *outcomes) print() {
	slices.Sort(o.rows) // goroutines finish in any order; sort for stable output
	for _, r := range o.rows {
		fmt.Println(r)
	}
}

var urls = []string{"https://a.example", "https://bad.example", "https://c.example", "https://d.example"}

func withErrgroup() {
	// The derived ctx is cancelled as soon as any goroutine returns an
	// error; that is what makes WithContext useful.
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(2) // at most two fetches in flight; g.Go blocks for a free slot

	var res outcomes
	for _, url := range urls {
		// No `url := url` here: since Go 1.22 each iteration has its own url.
		g.Go(func() error {
			err := fetch(ctx, url)
			res.add(url, err)
			return err
		})
	}

	// Wait returns the first non-nil error; the others are dropped.
	err := g.Wait()
	fmt.Println("errgroup outcomes:")
	res.print()
	fmt.Println("g.Wait() returned:", err)
}

func withWaitGroupGo() {
	// sync.WaitGroup.Go (Go 1.25) removes the Add/Done boilerplate, but its
	// functions return nothing: errors and cancellation are up to you.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for _, url := range []string{"https://bad.example", "https://bad2.example"} {
		wg.Go(func() {
			if err := fetch(context.Background(), url); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	slices.SortFunc(errs, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
	fmt.Printf("wg.Go collected %d errors by hand:\n%v\n", len(errs), errors.Join(errs...))
}

func concurrentErrors() {
	withErrgroup()
	withWaitGroupGo()
}
