package main

import (
	"net/http"
	"testing"
	"time"
)

// Each op is a burst of 8 concurrent GETs against a local HTTP/1.1 server.
// The default transport keeps only 2 idle connections per host, so most of
// every burst dials (and later closes) a fresh connection; the tuned one
// reuses its pool. The conns/op metric shows the churn directly. Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/046-connection-pooling
//
// The default-transport benchmark leaves closed sockets in TIME_WAIT. The
// burst is kept small so a default 1 s run stays well inside the OS's
// ephemeral port range.
const benchBurst = 8

func benchTransport(b *testing.B, t *http.Transport) {
	srv := newCountingServer(0)
	defer srv.Close()
	client := &http.Client{Transport: t, Timeout: 5 * time.Second}
	defer t.CloseIdleConnections()

	burst(client, srv.URL, benchBurst, true) // warm up: fill the pool once
	start := srv.newConns.Load()
	b.ReportAllocs()
	for b.Loop() {
		burst(client, srv.URL, benchBurst, true)
	}
	b.ReportMetric(float64(srv.newConns.Load()-start)/float64(b.N), "conns/op")
}

func BenchmarkDefaultTransport(b *testing.B) { benchTransport(b, defaultTransport()) }
func BenchmarkTunedTransport(b *testing.B)   { benchTransport(b, tunedTransport()) }
