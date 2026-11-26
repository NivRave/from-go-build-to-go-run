# Part 045: HTTP clients: The resilient requester

Day 23 · Week 4 · Mastering timeouts, retries, and defensive networking.

## What this shows

- `Client.Timeout` (100 ms) firing against an in-process `/slow` endpoint that needs 300 ms: the error is a `*url.Error`, a `net.Error` with `Timeout() == true`, and `errors.Is(err, context.DeadlineExceeded)` holds.
- A per-request `context.WithTimeout` (50 ms) firing sooner: the effective deadline is the earlier of the two.
- `Retry(ctx, attempts, fn)`: exponential backoff with full jitter (`rand.N`), waits that respect `ctx`, no sleep after the last attempt, `Retry-After` honoured, the last error wrapped with `%w`, and every response body drained (capped at 64 KiB) and closed. It recovers `/flaky` (503, 503, 200) and gives up on a dead endpoint.
- A hand-rolled circuit breaker (closed, open, half-open) that stops sending requests to the dead endpoint after 3 consecutive failures, then lets one probe through after the open timeout.

## Run

```sh
go run ./examples/045-http-clients
go test ./examples/045-http-clients
```

## What to look for

Expected output of `go run` (the two "gave up after" times can vary slightly):

```text
== 1. Client.Timeout fires (slides p02-p03) ==
error type: *url.Error, Timeout() = true
errors.Is(err, context.DeadlineExceeded) = true
gave up after ~100ms (the handler needs 300ms)

== 2. A per-request context deadline fires sooner (slide p04) ==
errors.Is(err, context.DeadlineExceeded) = true
gave up after ~50ms: the effective deadline is min(ctx 50ms, Client.Timeout 100ms)

== 3. Retry with backoff, jitter and Retry-After (slides p05-p07) ==
/flaky attempt 1: server returned 503 Service Unavailable (Retry-After: 0s)
/flaky attempt 2: server returned 503 Service Unavailable (Retry-After: 0s)
/flaky attempt 3: "ok"
/flaky result: "ok", err = <nil>
dead endpoint: 3 attempts, err != nil: true, last error is a dial error: true

== 4. A circuit breaker fails fast (slide p08) ==
call 1: dial error                state=closed    requests sent=1
call 2: dial error                state=closed    requests sent=2
call 3: dial error                state=open      requests sent=3
call 4: circuit breaker is open   state=open      requests sent=3
call 5: circuit breaker is open   state=open      requests sent=3
after the open timeout: state=half-open
call 6: ok                        state=closed    requests sent=4
```

- Both timeouts surface as `context.DeadlineExceeded`, so one `errors.Is` check covers either source.
- `/flaky` sends `Retry-After: 0`, so `Retry` uses the server's delay instead of its own backoff and the three attempts run back to back.
- Against the dead endpoint, `Retry` still finds the `*net.OpError` through its `%w` wrapping. The demo prints that fact rather than the error text, which differs per OS.
- "requests sent" stops at 3 while the breaker is open: calls 4 and 5 never reach the network. After the open timeout, one probe goes through, succeeds, and closes the breaker.
- `retry_test.go` and `breaker_test.go` run inside `testing/synctest`, so they check exact backoff gaps, the `Retry-After` delay and the open timeout without waiting in real time.

## Notes and corrections

- **Slide p03:** `resp` and `err` are declared and never used, so the snippet doesn't compile as written. Here every response is used and closed.
- **Slide p07:** `retryFetch` swallows the final error, sleeps after the last attempt, has no jitter (synchronised clients still retry in lockstep), uses `time.Sleep`, which ignores cancellation, ignores `Retry-After`, and doesn't close or drain the 503 bodies before retrying. `Retry` fixes each of these.
- **Slide p04:** `req, _ :=` drops the `NewRequestWithContext` error, the `url` variable shadows the `net/url` package, and `http.MethodGet` is preferred over `"GET"`. The effective deadline is the minimum of the context deadline and `Client.Timeout`.
- **Slide p05:** an unclosed body leaks a connection and its file descriptor rather than memory. Drain with a cap (`io.CopyN(io.Discard, body, 64<<10)`), and only drain what you didn't already read.
- **Slide p06:** DELETE is idempotent too. A POST can be retried safely when it carries an idempotency key, and 429 is retryable (respecting `Retry-After`).
- **Slide p08:** there's no circuit breaker in the standard library. This one is hand-rolled; `github.com/sony/gobreaker` is a mature third-party option.

## Slides

[045-http-clients.pdf](../../slides/045-http-clients.pdf)
