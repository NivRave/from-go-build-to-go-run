package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// syncBuffer is a bytes.Buffer safe for concurrent use. After a forced
// server.Close(), Run returns while an abandoned handler may still be logging,
// so the test must not read a plain bytes.Buffer the logger is writing to.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// pipeListener is an in-memory net.Listener built on net.Pipe. A synctest
// bubble cannot tell whether a goroutine blocked on a real socket will wake
// up, so the tests serve HTTP over pipes instead.
type pipeListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

func (l *pipeListener) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	server, client := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.done:
		return nil, errors.New("connection refused")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

type response struct {
	status int
	body   string
	err    error
}

// postOrderAsync sends POST /orders over the pipe listener in the background.
func postOrderAsync(ln *pipeListener) <-chan response {
	client := &http.Client{Transport: &http.Transport{DialContext: ln.DialContext, DisableKeepAlives: true}}
	out := make(chan response, 1)
	go func() {
		resp, err := client.Post("http://app/orders", "text/plain", http.NoBody)
		if err != nil {
			out <- response{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		out <- response{status: resp.StatusCode, body: string(body), err: err}
	}()
	return out
}

// startApp runs app.Run in the background and returns its result channel.
func startApp(ctx context.Context, app *App) <-chan error {
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	return done
}

func TestRunDrainsInFlightRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ln := newPipeListener()
		var logs syncBuffer
		app := NewApp(ln, &logs)

		ctx, cancel := context.WithCancelCause(t.Context())
		runErr := startApp(ctx, app)
		resp := postOrderAsync(ln)

		// 300 ms of fake time: the request is inside the 1 s handler.
		time.Sleep(300 * time.Millisecond)
		synctest.Wait()
		start := time.Now()
		cancel(errors.New("test SIGTERM"))

		if err := <-runErr; err != nil {
			t.Fatalf("Run: %v", err)
		}
		if elapsed := time.Since(start); elapsed >= app.ShutdownTimeout {
			t.Errorf("shutdown took %v, want less than the %v limit", elapsed, app.ShutdownTimeout)
		}

		r := <-resp
		if r.err != nil || r.status != http.StatusOK || r.body != "order 1 accepted" {
			t.Errorf("in-flight request = %d %q (err %v), want 200 \"order 1 accepted\"", r.status, r.body, r.err)
		}
		if saved, closed := app.DB.Stats(); saved != 1 || !closed {
			t.Errorf("db: saved=%d closed=%t, want saved=1 closed=true", saved, closed)
		}

		// The steps must appear in this order.
		out := logs.String()
		last := -1
		for _, step := range []string{
			"shutdown started: test SIGTERM",
			"handler: order 1 processed",
			"[1/4] http: drained",
			"[2/4] workers: all 3 stopped",
			"[3/4] db: closed (orders saved: 1)",
			"[4/4] goodbye",
		} {
			i := strings.Index(out, step)
			if i < 0 || i < last {
				t.Fatalf("log is missing %q or has it out of order:\n%s", step, out)
			}
			last = i
		}
	})
}

func TestRunForcesCloseAfterTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ln := newPipeListener()
		var logs syncBuffer
		app := NewApp(ln, &logs)
		app.SlowDelay = time.Minute // longer than ShutdownTimeout

		ctx, cancel := context.WithCancelCause(t.Context())
		runErr := startApp(ctx, app)
		resp := postOrderAsync(ln)

		time.Sleep(300 * time.Millisecond)
		synctest.Wait()
		start := time.Now()
		cancel(errors.New("test SIGTERM"))

		err := <-runErr
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run = %v, want an error wrapping context.DeadlineExceeded", err)
		}
		if elapsed := time.Since(start); elapsed != app.ShutdownTimeout {
			t.Errorf("shutdown took %v, want exactly the %v limit", elapsed, app.ShutdownTimeout)
		}
		if r := <-resp; r.err == nil {
			t.Errorf("request survived a forced Close: %d %q", r.status, r.body)
		}
		if _, closed := app.DB.Stats(); !closed {
			t.Error("db was not closed after a forced shutdown")
		}
		if !strings.Contains(logs.String(), "forcing server.Close()") {
			t.Errorf("log does not mention the forced close:\n%s", logs.String())
		}
	})
}

func TestRunWithCancelledContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		app := NewApp(newPipeListener(), io.Discard)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := app.Run(ctx); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if _, closed := app.DB.Stats(); !closed {
			t.Error("db was not closed")
		}
	})
}
