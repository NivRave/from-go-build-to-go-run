// Command connection-pooling demonstrates how http.Transport pools
// keep-alive connections: what MaxIdleConnsPerHost really limits, why bodies
// must be read to EOF and closed, how httptrace proves reuse, and how
// MaxConnsPerHost caps dialing.
//
// Slides: slides/046-connection-pooling.pdf
//
// Run:
//
//	go run ./examples/046-connection-pooling
//	go test -run='^$' -bench=. -benchmem ./examples/046-connection-pooling
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	section("1. 100 concurrent requests, three clients (slides p04, p05, p08)")
	poolRounds()

	section("2. Proving reuse with httptrace (slide p05)")
	traceReuse()

	section("3. MaxConnsPerHost makes a burst wait (slides p04, p08)")
	maxConnsBurst()
}

// countingServer is an HTTP/1.1 test server that counts the TCP connections
// it accepts. httptest.NewServer speaks plain HTTP/1.1, where every
// concurrent request needs its own connection; over HTTP/2 they would share
// one, and per-host idle limits would matter far less.
type countingServer struct {
	*httptest.Server
	newConns atomic.Int64
}

func newCountingServer(delay time.Duration) *countingServer {
	cs := &countingServer{}
	cs.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay) // keep requests in flight long enough to overlap
		fmt.Fprint(w, "pong")
	}))
	cs.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			cs.newConns.Add(1)
		}
	}
	cs.Start()
	return cs
}

// defaultTransport is a fresh copy of http.DefaultTransport, so every demo
// starts with an empty pool. Its MaxIdleConnsPerHost is 0, which means
// http.DefaultMaxIdleConnsPerHost (2).
func defaultTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

// tunedTransport starts from DefaultTransport.Clone() and overrides fields.
// A bare &http.Transport{...} (slides p04, p07) would silently drop the
// defaults: Proxy from the environment, dial timeout and keep-alive,
// TLSHandshakeTimeout, ExpectContinueTimeout and ForceAttemptHTTP2.
func tunedTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 200        // across all hosts
	t.MaxIdleConnsPerHost = 100 // idle connections KEPT per host after use
	t.IdleConnTimeout = 90 * time.Second

	// A custom DialContext turns off automatic HTTP/2 unless
	// ForceAttemptHTTP2 is true. Clone keeps DefaultTransport's true, so
	// this is safe. KeepAliveConfig (Go 1.23+) tunes TCP keep-alive probes
	// in more detail than the older Dialer.KeepAlive interval.
	t.DialContext = (&net.Dialer{
		Timeout: 5 * time.Second,
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     30 * time.Second,
			Interval: 15 * time.Second,
			Count:    3,
		},
	}).DialContext
	return t
}

// burst sends n concurrent GETs. If drain is false, each body is closed
// without being read, which stops the connection from being reused.
func burst(client *http.Client, url string, n int, drain bool) {
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			resp, err := client.Get(url)
			if err != nil {
				fmt.Println("request failed:", err)
				return
			}
			defer resp.Body.Close()
			if drain {
				_, _ = io.Copy(io.Discard, resp.Body)
			}
		})
	}
	wg.Wait()
}

func poolRounds() {
	srv := newCountingServer(10 * time.Millisecond)
	defer srv.Close()

	clients := []struct {
		label     string
		transport *http.Transport
		drain     bool
	}{
		{"(a) default transport (MaxIdleConnsPerHost=2)", defaultTransport(), true},
		{"(b) cloned, MaxIdleConnsPerHost=100", tunedTransport(), true},
		{"(c) tuned, but bodies not drained", tunedTransport(), false},
	}
	for _, c := range clients {
		client := &http.Client{Transport: c.transport, Timeout: 5 * time.Second}
		var rounds []string
		for range 3 {
			before := srv.newConns.Load()
			burst(client, srv.URL, 100, c.drain)
			rounds = append(rounds, fmt.Sprint(srv.newConns.Load()-before))
		}
		c.transport.CloseIdleConnections()
		fmt.Printf("%-46s new connections per round: %s\n", c.label, strings.Join(rounds, " "))
	}
}

func traceReuse() {
	srv := newCountingServer(0)
	defer srv.Close()
	client := &http.Client{Transport: tunedTransport(), Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()

	steps := []struct {
		note  string
		drain bool
	}{
		{"first request: nothing to reuse", true},
		{"previous body read to EOF and closed", true},
		{"previous body read to EOF and closed; this one is closed unread", false},
		{"previous body closed WITHOUT reading it", true},
	}
	for i, step := range steps {
		var info httptrace.GotConnInfo
		trace := &httptrace.ClientTrace{
			GotConn: func(gci httptrace.GotConnInfo) { info = gci },
		}
		req, err := http.NewRequestWithContext(
			httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, srv.URL, nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		if step.drain {
			_, _ = io.Copy(io.Discard, resp.Body)
		}
		resp.Body.Close()
		fmt.Printf("request %d: Reused=%-5t WasIdle=%-5t (%s)\n", i+1, info.Reused, info.WasIdle, step.note)
	}
	fmt.Printf("server accepted %d connections for 4 requests\n", srv.newConns.Load())
}

func maxConnsBurst() {
	for _, limit := range []int{0, 2} {
		srv := newCountingServer(10 * time.Millisecond)
		t := tunedTransport()
		t.MaxConnsPerHost = limit // caps dials (and in-flight conns); 0 = unlimited
		client := &http.Client{Transport: t, Timeout: 5 * time.Second}

		start := time.Now()
		burst(client, srv.URL, 20, true)
		elapsed := time.Since(start)

		t.CloseIdleConnections()
		srv.Close()
		fmt.Printf("MaxConnsPerHost=%d: 20 requests took ~%v over %d connections\n",
			limit, elapsed.Round(10*time.Millisecond), srv.newConns.Load())
	}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
