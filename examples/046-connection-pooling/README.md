# Part 046: Connection pooling & keep-alives

Day 23 · Week 4 · Optimizing the hidden infrastructure of HTTP communication.

## What this shows

- An `httptest` HTTP/1.1 server that counts new TCP connections through `srv.Config.ConnState` (`StateNew`), hit with rounds of 100 concurrent requests from three clients: the default transport (2 idle connections kept per host) churns, a `DefaultTransport.Clone()` with `MaxIdleConnsPerHost = 100` reuses, and a client that closes bodies without reading them never reuses.
- `httptrace.ClientTrace` `GotConn` proving reuse (`Reused`, `WasIdle`) for sequential requests, and showing that one unread body costs the connection.
- `MaxConnsPerHost = 2` making a burst of 20 requests queue for two connections, timed against no limit.
- A benchmark of the default versus the tuned transport, reporting connections dialed per op.

## Run

```sh
go run ./examples/046-connection-pooling
go test ./examples/046-connection-pooling
go test -run='^$' -bench=. -benchmem ./examples/046-connection-pooling
```

## What to look for

Expected output of `go run` (connection counts in section 1 can differ by one or two between runs, and the burst times vary by machine):

```text
== 1. 100 concurrent requests, three clients (slides p04, p05, p08) ==
(a) default transport (MaxIdleConnsPerHost=2)  new connections per round: 100 98 98
(b) cloned, MaxIdleConnsPerHost=100            new connections per round: 100 0 0
(c) tuned, but bodies not drained              new connections per round: 100 100 100

== 2. Proving reuse with httptrace (slide p05) ==
request 1: Reused=false WasIdle=false (first request: nothing to reuse)
request 2: Reused=true  WasIdle=true  (previous body read to EOF and closed)
request 3: Reused=true  WasIdle=true  (previous body read to EOF and closed; this one is closed unread)
request 4: Reused=false WasIdle=false (previous body closed WITHOUT reading it)
server accepted 2 connections for 4 requests

== 3. MaxConnsPerHost makes a burst wait (slides p04, p08) ==
MaxConnsPerHost=0: 20 requests took ~10ms over 20 connections
MaxConnsPerHost=2: 20 requests took ~110ms over 2 connections
```

Benchmark shape (numbers vary by machine):

```text
BenchmarkDefaultTransport   ~1.5 ms/op   ~5.5 conns/op   ~180 KB/op
BenchmarkTunedTransport     ~0.4 ms/op      0 conns/op    ~80 KB/op
```

- (a): every round still opens about 98 new connections. `MaxIdleConnsPerHost=2` didn't make anyone wait; it let 100 connections be dialed and then closed all but 2 when they went idle. That's churn: handshakes plus sockets in TIME_WAIT.
- (b): after the first round, the pool holds all 100 connections and no new ones are dialed.
- (c): closing a body without reading it to EOF makes the transport close the connection, so the pool never fills, even with a generous limit. Section 2 shows the same thing one request at a time: request 4 can't reuse the connection that request 3 left unread.
- Section 3: only `MaxConnsPerHost` caps dialing. With a limit of 2, the burst runs two at a time and takes about ten times as long.

## Notes and corrections

- **Slides p04/p08:** `MaxIdleConnsPerHost` limits how many connections are **kept idle** after use, not how many can be dialed. Dialing is unbounded unless `MaxConnsPerHost` is set (0 means unlimited), so goroutines never wait because of `MaxIdleConnsPerHost`. The cost of a low value is churn: extra handshakes and TIME_WAIT sockets.
- **Slides p04/p07:** a bare `&http.Transport{...}` loses `DefaultTransport`'s settings: proxy from the environment, dial timeout and keep-alive, `TLSHandshakeTimeout`, `ExpectContinueTimeout` and `ForceAttemptHTTP2`. Start from `http.DefaultTransport.(*http.Transport).Clone()` and override fields. Setting a custom `DialContext` disables automatic HTTP/2 unless `ForceAttemptHTTP2` is true.
- **Slide p07:** Go 1.23 added `net.KeepAliveConfig` (`Dialer.KeepAliveConfig`) to set the TCP keep-alive idle time, probe interval and probe count; `Dialer.KeepAlive` sets only the interval. The slide's 30 s dial timeout is the same as the default.
- **Slide p05:** the connection goes back to the pool only after the body is read to EOF **and** closed. ("As we learned this morning" is a leftover phrase.)
- **Slide p02:** HTTP/2 multiplexes many requests over one connection, so per-host idle limits matter mostly for HTTP/1.1.
- The slides don't mention `net/http/httptrace`; `GotConnInfo.Reused` is the standard way to prove reuse.

## Slides

[046-connection-pooling.pdf](../../slides/046-connection-pooling.pdf)
