// Command http-server demonstrates a production-shaped net/http entry point:
// a local ServeMux with struct and function handlers, an http.Server with
// explicit timeouts, correct handling of http.ErrServerClosed, and a
// slowloris client being cut off by ReadHeaderTimeout.
//
// Slides: slides/043-http-server.pdf
//
// Run:
//
//	go run ./examples/043-http-server
//	go run ./examples/043-http-server -serve
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	serve := flag.Bool("serve", false, "run on localhost:8080 until Ctrl+C")
	flag.Parse()

	if *serve {
		if err := serveUntilInterrupt("localhost:8080"); err != nil {
			log.Fatal(err)
		}
		return
	}

	section("1. A local ServeMux with Handler and HandlerFunc (slides p03-p05)")
	localMux()

	section("2. A configured http.Server on 127.0.0.1:0 (slides p06-p07)")
	if err := configuredServer(); err != nil {
		log.Fatal(err)
	}

	section("3. Slowloris vs ReadHeaderTimeout (slide p07)")
	if err := slowloris(200 * time.Millisecond); err != nil {
		log.Fatal(err)
	}
}

// localMux calls the mux directly with httptest.NewRecorder: no port, no
// goroutines. This is what a local mux buys you in tests.
func localMux() {
	mux := newMux(demoStore())
	for _, target := range []string{"/", "/health", "/users?id=1", "/users?id=99"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		fmt.Printf("GET %-13s -> %d %s\n", target, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}

// configuredServer runs the real server on an OS-chosen loopback port, calls
// it with an http.Client, and shuts it down gracefully.
func configuredServer() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := newServer(newMux(demoStore()), 5*time.Second)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }() // Serve owns ln and closes it.

	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://" + ln.Addr().String()
	for _, path := range []string{"/health", "/users?id=2"} {
		resp, err := client.Get(base + path)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		fmt.Printf("GET %-11s -> %s %s\n", path, resp.Status, strings.TrimSpace(string(body)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}

	// Serve (like ListenAndServe) always returns a non-nil error. After
	// Shutdown it is http.ErrServerClosed, which means "stopped as asked".
	// The slide's `if err != nil { log.Fatal(err) }` would treat a clean
	// shutdown as a crash.
	err = <-serveErr
	fmt.Printf("Serve returned %q; errors.Is(err, http.ErrServerClosed) = %t\n",
		err, errors.Is(err, http.ErrServerClosed))
	return nil
}

// slowloris opens a TCP connection and sends nothing, like a slow-client
// attack. With ReadHeaderTimeout set, the server gives up on the connection
// and closes it; without any timeout the connection (and its goroutine)
// would be held forever.
func slowloris(readHeaderTimeout time.Duration) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := newServer(newMux(demoStore()), readHeaderTimeout)
	go func() { _ = srv.Serve(ln) }() // the error is ErrServerClosed after Close below
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		return err
	}
	defer conn.Close()

	// A client-side safety net so the demo can never hang.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	start := time.Now()
	_, err = conn.Read(make([]byte, 1)) // blocks until the server hangs up
	elapsed := time.Since(start)

	fmt.Printf("ReadHeaderTimeout = %v\n", readHeaderTimeout)
	fmt.Printf("server closed the silent connection after ~%v (read error is EOF: %t)\n",
		elapsed.Round(10*time.Millisecond), errors.Is(err, io.EOF))
	return nil
}

// serveUntilInterrupt is the -serve mode: a real server on addr that shuts
// down gracefully on Ctrl+C, letting in-flight requests finish.
func serveUntilInterrupt(addr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Listening separately from serving reports "port in use" before we
	// claim to be up. srv.ListenAndServe() would do both in one call.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := newServer(newMux(demoStore()), 5*time.Second)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Printf("listening on http://%s (try /, /health, /users?id=1); Ctrl+C to stop", addr)

	select {
	case err := <-serveErr:
		return err // Serve failed on its own
	case <-ctx.Done():
	}

	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Print("stopped cleanly")
	return nil
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
