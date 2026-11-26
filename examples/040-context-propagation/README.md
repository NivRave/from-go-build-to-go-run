# Part 040: Context Propagation: The Invisible Thread

Day 20 · Week 3 · Synchronizing concurrency across network boundaries.

## What this shows

- A client `http.RoundTripper` that **clones** the request, then sets `X-Trace-ID` and the remaining budget as `X-Timeout-Ms` (computed with `time.Until(deadline)`). The caller's request is left untouched, and a nil base falls back to `http.DefaultTransport`.
- Server middleware that puts the trace ID back into the context (unexported key) and applies the budget with `context.WithTimeout(r.Context(), remaining)`.
- Budgeting across three in-process services (`httptest.NewServer`, Go 1.22 `ServeMux` patterns): the gateway has 1 s, auth spends 300 ms, and business sees about 700 ms. A business handler that needs 2 s gets `context.DeadlineExceeded`.
- Cancellation: the client gives up mid-flight and the business handler, two hops away, sees `r.Context().Done()`. A handler that wrongly uses `context.Background()` keeps working for nobody.
- A `slog.Handler` wrapper that adds `trace_id` to every line logged with a context, so all three services' logs for one request share an ID.

## Run

```sh
go run ./examples/040-context-propagation
go test ./examples/040-context-propagation
```

## What to look for

Expected output of `go run` (sections 3 and 4 use real time over loopback HTTP, so the rounded durations can differ slightly on a slow machine):

```text
== 1. Client: a RoundTripper that clones and injects (slide p04) ==
sent:     X-Trace-ID=abc-123, X-Timeout-Ms=~500ms
original: 0 headers (the caller's request was not modified)

== 2. Server: middleware rehydrates the trace ID and the budget (slide p05) ==
handler sees trace_id=abc-123, budget ~700ms
handler sees trace_id=<new, 26 chars>, budget none

== 3. Budgeting: gateway 1s -> auth 300ms -> business (slide p03) ==
level=INFO msg="user verified" service=auth trace_id=trace-fast
level=INFO msg="handling order" service=business id=42 budget=700ms trace_id=trace-fast
client: GET /orders/42?work=100ms -> 200 OK after ~400ms: order 42: shipped
level=INFO msg="user verified" service=auth trace_id=trace-slow
level=INFO msg="handling order" service=business id=43 budget=700ms trace_id=trace-slow
level=INFO msg="stopped early" service=business err="context deadline exceeded" trace_id=trace-slow
client: GET /orders/43?work=2s -> 504 Gateway Timeout after ~1s: context deadline exceeded

== 4. Cancellation travels by aborting connections (slides p07, p08) ==
level=INFO msg="user verified" service=auth trace_id=trace-cancel
level=INFO msg="handling order" service=business id=44 budget=700ms trace_id=trace-cancel
level=INFO msg="stopped early" service=business err="context canceled" trace_id=trace-cancel
client: GET /orders/44?work=2s -> error after ~400ms: context canceled
level=INFO msg="user verified" service=auth trace_id=trace-cancel
level=INFO msg="detached handler finished work nobody is waiting for" service=business id=45 trace_id=trace-cancel
client: GET /detached/orders/45?work=300ms -> error after ~400ms: context canceled

== 5. slog lines carry trace_id from the context ==
level=INFO msg="payment captured" service=payments amount=42 trace_id=abc-123
level=INFO msg="cache warmed" service=payments
```

- Section 3: nobody configured a 700 ms timeout on the business service. It inherited what was left of the gateway's second, so it stops when the caller would have stopped waiting anyway, instead of becoming a "zombie" request.
- Each hop reserves 10 ms of its budget (`Transport.Reserve`) so it can still receive and pass back the downstream 504 before its own deadline fires. Without a reserve, all three deadlines expire at almost the same instant.
- Section 4: the business handler was cancelled although no "cancel" message exists on the wire. The client closed its connection, which cancelled the gateway's `r.Context()`, which aborted the gateway's request to auth, and so on. The detached handler finished its 300 ms of work after the client had left.
- `propagation_test.go` checks, inside `testing/synctest`, that `Transport` never mutates the caller's request and sends exactly the remaining budget, and that `Middleware` applies it to the millisecond.

## Notes and corrections

- **Slide p04:** an `http.RoundTripper` must not modify the request it is given; set headers on `req.Clone(ctx)`. Sending an absolute RFC3339 deadline assumes synchronised clocks and drops sub-second precision, so send the remaining duration instead, as gRPC does with `grpc-timeout`. The standard trace header is W3C `traceparent` (Part 050), not `X-Trace-ID`.
- **Slide p05:** the middleware reads the trace ID but never applies the deadline, so the budget from p03 is lost at the first hop. It also stores an empty trace ID when the header is missing; this version starts a new trace instead.
- **Slide p07:** to detach on purpose (an audit log that must outlive the request), use `context.WithoutCancel(r.Context())` (Go 1.21). It drops cancellation but keeps the values, unlike `context.Background()`.
- **Slide p08:** cancellation is not sent as a message. It travels because the client aborts the connection (HTTP/1.1) or resets the stream (HTTP/2), which cancels the server's `r.Context()`.

## Slides

[040-context-propagation.pdf](../../slides/040-context-propagation.pdf)
