# Part 060: Resilience patterns: Context, timeouts, and retries

Day 30 · Week 5 · Architecting for failure in a flaky world.

## What this shows

- **Ghost load:** the slide's `select` + `time.After` version returns on time, but the request it started keeps running, and the fake provider finishes the work and charges for it after the caller has given up.
- **The fix:** `CallAI` puts the deadline in the context with `context.WithTimeoutCause` and sends the request with `http.NewRequestWithContext`, so the provider sees the cancellation and stops. The error carries the cause, `ErrAITimeout`.
- **Retries done properly:** `RetryWithBackoff(ctx, attempts, fn func(context.Context) error)` with exponential backoff, full jitter (`math/rand/v2`), no wait after the last attempt, and the last error wrapped. With one `Idempotency-Key` reused across attempts, a response lost after the provider charged costs one charge instead of two.
- **A circuit breaker** (Closed, Open, Half-Open) with gobreaker's vocabulary, generic like `gobreaker/v2`, so there is no `body.([]byte)` to panic, and an `OnStateChange` hook.
- **Metrics** as `atomic.Int64` values with Prometheus-style names: `retries_total`, `breaker_state_changes_total` and the `breaker_state` gauge.

This part composes the client mechanics of Part 045 into a policy around one flaky dependency; the provider is an `httptest` server on 127.0.0.1 with a fixed failure schedule, so every run behaves the same.

## Run

```sh
go run ./examples/060-resilience-patterns
go test ./examples/060-resilience-patterns
```

## What to look for

Expected output of `go run` (it takes about 2.5 seconds, mostly waiting for the ghost request):

```text
== 1. Ghost load: select + time.After (slides p02, p03) ==
CallFlakyAI returned: AI provider took too long
provider requests in flight right after: 1
...and later: in flight 0, cancelled 0, charged 1
the provider did the work and billed for it after the caller had given up

== 2. The fix: the deadline travels in the context ==
CallAI returned: call AI: AI provider took too long
errors.Is(err, ErrAITimeout): true
provider: in flight 0, cancelled 1, charged 0

== 3. Retries with backoff, jitter and an idempotency key (slides p04-p06) ==
with Idempotency-Key: false
  attempt 1 failed: call AI: provider returned 503; retrying
  attempt 2 failed: call AI: provider returned 502; retrying
  result: "summary of quarterly report", err: <nil>
  provider requests: 3, charges: 2
with Idempotency-Key: true
  attempt 1 failed: call AI: provider returned 503; retrying
  attempt 2 failed: call AI: provider returned 502; retrying
  result: "summary of quarterly report", err: <nil>
  provider requests: 3, charges: 1

== 4. Circuit breaker (slide p07) ==
call 1: err = call AI: provider returned 503
call 2: err = call AI: provider returned 503
  [AI-Service] closed -> open
call 3: err = call AI: provider returned 503
call 4: err = circuit breaker is open
call 5: err = circuit breaker is open
provider saw 3 of 5 calls; the rest failed fast
  [AI-Service] open -> half-open
  [AI-Service] half-open -> closed
after the open timeout: "summary of quarterly report", err = <nil>

== 5. Metrics (slide p08) ==
retries_total 4
breaker_state_changes_total 3
breaker_state 0
```

- Sections 1 and 2 hit the same hanging provider with the same 200 ms budget. Both callers get an error on time, but only the context version leaves the provider with `cancelled 1, charged 0`. The `select` version still costs a full generation and a charge.
- In section 3 the 502 is a response lost *after* the provider charged (a dropped connection, a gateway timeout). Without a key the retry is a new request and is charged again; with the key the provider replays the stored result for free.
- The breaker trips on the third consecutive failure, so calls 4 and 5 never reach the provider. After `Timeout` it lets one probe through, and that probe's success closes it. `call 3` prints after the state change because `OnStateChange` fires inside `Execute`.
- `resilience_test.go` checks the exact backoff schedule (100, 200, 400, 800 ms waits; none after the last attempt), the jitter bounds, cancellation during a wait, the breaker's open timeout to the nanosecond, and the half-open probe limit. They use `testing/synctest`, so the 30-second breaker timeout takes no real time.

## Notes and corrections

- **Slide p07:** `body.([]byte)` panics when the breaker is open, because `Execute` returns `nil, ErrOpenState` and a type assertion on a nil interface panics. Check `err` first or use the comma-ok form. `sony/gobreaker/v2` is generic (`NewCircuitBreaker[[]byte]`), which removes the assertion; this example's breaker follows the same design. Use `IsSuccessful` so the caller's own `context.Canceled` doesn't count as a provider failure.
- **Slide p03:** the goroutine keeps working after the timeout, which is exactly the ghost load slide p02 warns about. Pass `ctx` to the client instead; then the `select` and the goroutine are unnecessary.
- **Slide p05:** the loop sleeps after the final attempt, discards the last error, has no jitter despite slide p04, and `fn` gets no `ctx`. Using `time.After` in the loop is fine since Go 1.23, because unreferenced timers are now garbage-collected.
- **Slide p02:** newer context APIs help here: `WithCancelCause` (Go 1.20), and `AfterFunc`, `WithoutCancel`, `WithDeadlineCause` and `WithTimeoutCause` (Go 1.21).
- **Slide p08:** Prometheus counters end in `_total` (`retries_total`), and "breaker open" is a state, so expose it as a gauge (`breaker_state`) or count transitions (`breaker_state_changes_total`).

## Slides

[060-resilience-patterns.pdf](../../slides/060-resilience-patterns.pdf)
