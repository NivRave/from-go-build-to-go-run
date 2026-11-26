// Command graceful-shutdown demonstrates an ordered shutdown: stop the HTTP
// server and drain in-flight requests, wait for the workers with a timeout,
// close the database, then say goodbye.
//
// Slides: slides/051-graceful-shutdown.pdf
//
// Run:
//
//	go run ./examples/051-graceful-shutdown
//	go run ./examples/051-graceful-shutdown -wait
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	wait := flag.Bool("wait", false, "serve until a real Ctrl+C (or SIGTERM) instead of simulating one")
	flag.Parse()

	var err error
	if *wait {
		err = runWait()
	} else {
		err = runDemo()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runDemo is the self-demonstrating mode: it does not depend on real OS
// signals (Windows cannot send os.Interrupt to itself), so it cancels the root
// context with a cause instead, which is what signal.NotifyContext does when a
// signal arrives.
func runDemo() error {
	section("1. Start the app and send a slow request (slides p03, p06)")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	app := NewApp(ln, os.Stdout)

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	// The listener is already bound, so this request waits in the accept
	// backlog until Run starts serving.
	fmt.Println("client: POST /orders (the handler takes 1s)")
	resp := make(chan string, 1)
	go func() { resp <- postOrder("http://" + ln.Addr().String() + "/orders") }()

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(ctx) }()

	time.Sleep(300 * time.Millisecond)
	section("2. Simulated SIGTERM after 300 ms: ordered shutdown (slides p04-p08)")
	cancel(errors.New("simulated SIGTERM"))
	if err := <-runErr; err != nil {
		return err
	}

	section("3. The in-flight request finished during the drain (slide p05)")
	fmt.Println("client:", <-resp)
	saved, closed := app.DB.Stats()
	fmt.Printf("db: saved=%d closed=%t\n", saved, closed)
	return nil
}

// runWait serves until a real signal arrives. Press Ctrl+C once for a
// graceful shutdown, twice to kill the process.
func runWait() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Slide p04: once the first signal has cancelled ctx, call stop() so the
	// signal is no longer diverted. A second Ctrl+C then gets Go's default
	// behaviour, which exits the process immediately.
	context.AfterFunc(ctx, stop)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/orders"
	fmt.Printf("listening on %s\n", url)
	fmt.Printf("in another terminal: curl -X POST %s\n", url)
	fmt.Println("then press Ctrl+C within a second to see the request drain (twice to force-quit)")
	return NewApp(ln, os.Stdout).Run(ctx)
}

func postOrder(url string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "text/plain", http.NoBody)
	if err != nil {
		return "request failed: " + err.Error()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "read failed: " + err.Error()
	}
	return fmt.Sprintf("%s %q", resp.Status, string(body))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
