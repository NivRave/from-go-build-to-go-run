package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"sync/atomic"
	"time"
)

// Slide p03 uses `import _ "net/http/pprof"` and http.ListenAndServe(":6060", nil).
// That has two problems:
//
//  1. ":6060" listens on every network interface, not just an internal one.
//  2. The import registers the handlers on http.DefaultServeMux. If the main
//     application also serves DefaultServeMux (any handler passed as nil, or
//     any http.HandleFunc call), pprof is exposed on the public port too.
//
// Importing the package by name still runs its init, so DefaultServeMux gets
// the routes either way. The fix is never to serve DefaultServeMux: the app
// gets its own mux, and pprof is registered explicitly on a second mux bound
// to localhost.

const (
	appAddr   = "localhost:8080"
	pprofAddr = "localhost:6060"
)

// leaked counts goroutines started by /leak that will never exit.
var leaked atomic.Int64

// newAppMux returns the application routes (Go 1.22 method patterns). None of
// them is a pprof route.
func newAppMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cpu", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "hashed %d rounds\n", hashWork(50*time.Millisecond))
	})
	mux.HandleFunc("GET /alloc", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "marshalled %d bytes\n", allocWork(50*time.Millisecond))
	})
	mux.HandleFunc("GET /leak", leakHandler)
	mux.HandleFunc("GET /lock", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "counter reached %d\n", lockWork(50*time.Millisecond, 8))
	})
	return mux
}

// leakHandler is the bug from slide p07: a goroutine waits on a channel that
// nobody will ever send on or close, so it lives until the process exits.
// Every request adds one. In /debug/pprof/goroutine?debug=1 they all appear
// under the same stack, with a count that only grows.
func leakHandler(w http.ResponseWriter, _ *http.Request) {
	results := make(chan string) // nobody ever sends on this
	go func() {
		<-results // blocks forever: the leak
	}()
	fmt.Fprintf(w, "leaked goroutines so far: %d\n", leaked.Add(1))
}

// newPprofMux registers the net/http/pprof handlers explicitly. pprof.Index
// also serves the named runtime profiles (heap, allocs, goroutine, block,
// mutex, threadcreate) under /debug/pprof/<name>.
func newPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	return mux
}

// serve runs the app on appAddr and pprof on pprofAddr until ctx is
// cancelled (Ctrl+C), then shuts both down. With load set, a built-in load
// generator keeps hitting the app routes so the profiles have data.
func serve(ctx context.Context, load bool) error {
	servers := []*http.Server{
		{Addr: appAddr, Handler: newAppMux(), ReadHeaderTimeout: 5 * time.Second},
		// No WriteTimeout here: /debug/pprof/profile?seconds=30 must be able
		// to stream for 30 seconds.
		{Addr: pprofAddr, Handler: newPprofMux(), ReadHeaderTimeout: 5 * time.Second},
	}

	errc := make(chan error, len(servers))
	for _, srv := range servers {
		go func() {
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
	}
	log.Printf("app   on http://%s  (GET /cpu /alloc /leak /lock)", appAddr)
	log.Printf("pprof on http://%s/debug/pprof/", pprofAddr)

	if load {
		go generateLoad(ctx, "http://"+appAddr)
		log.Print("load generator running")
	}

	var err error
	select {
	case <-ctx.Done():
	case err = <-errc: // e.g. the port is already in use
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range servers {
		_ = srv.Shutdown(shutdownCtx)
	}
	return err
}

// generateLoad requests every app route in a loop until ctx is cancelled.
func generateLoad(ctx context.Context, base string) {
	client := &http.Client{Timeout: 5 * time.Second}
	paths := []string{"/cpu", "/alloc", "/lock", "/leak"}
	for i := 0; ctx.Err() == nil; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+paths[i%len(paths)], nil)
		if err != nil {
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(100 * time.Millisecond) // server not up yet, or shutting down
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
