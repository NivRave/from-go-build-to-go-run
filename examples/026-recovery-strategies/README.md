# Part 026: Recovery strategies

Day 13 · Week 2 · Panic, recover, and middleware logging.

## What this shows

- `SafeExecution` from slide p03 recovering and returning normally, and `runTask`, a boundary that turns any panic into an `error` through a named result.
- The pitfall: `recover` works only when called directly by the deferred function. `defer tryRecover()` recovers; `defer func() { tryRecover() }()` gets nil, and an outer protective `recover` catches the panic instead.
- `safeGo`, a goroutine wrapper with its own `recover` that logs `debug.Stack()` through `slog`, started with `sync.WaitGroup.Go` (Go 1.25).
- HTTP with `httptest`: without middleware a panicking handler drops the connection (the client sees `EOF`), but the server keeps serving. With `RecoveryMiddleware` the client gets a 500 and a body, and the panic is logged once. The middleware re-panics `http.ErrAbortHandler`.
- `panic(nil)` is recovered as a `*runtime.PanicNilError` (Go 1.21).
- Opt-in endings: `-repanic` shows Go 1.25's `[recovered, repanicked]` crash report, and `-crash` shows a `fatal error` that `recover` can't catch.

## Run

```sh
go run ./examples/026-recovery-strategies
go test ./examples/026-recovery-strategies
go run ./examples/026-recovery-strategies -repanic
go run ./examples/026-recovery-strategies -crash
```

The last two end the process on purpose with exit status 2 (`go run` itself then reports `exit status 2` and exits with 1).

## What to look for

Expected output of `go run` (the byte counts of the trimmed stacks vary):

```text
== 1. recover in a deferred function (slide p03) ==
Starting...
Rescued from: unrecoverable state
runTask(ok):     <nil>
runTask(divide): task divide panicked: runtime error: integer divide by zero

== 2. recover must be called directly by the deferred function (slide p03) ==
defer tryRecover(...):               recovered "boom 1"
defer func() { tryRecover(...) }():  recover() returned nil
  outer protective recover:          caught "boom 2"

== 3. Every goroutine needs its own recover (slide p04) ==
level=ERROR msg="goroutine panicked" worker=thumbnail panic="decoder: corrupt header" stack="<732 bytes, trimmed for the demo>"
2 workers finished, 1 panic was logged, and main is still running

== 4. HTTP: net/http's own recovery vs RecoveryMiddleware (slides p05-p06) ==
without middleware:
  GET /panic -> request failed, errors.Is(err, io.EOF) = true
  GET /ok    -> 200 "ok\n" (the server survived)
with RecoveryMiddleware:
level=ERROR msg="panic in handler" method=GET path=/panic panic="nil user record" stack="<1519 bytes, trimmed for the demo>"
  GET /panic -> 500 "Internal Server Error\n"
  GET /ok    -> 200 "ok\n"

== 5. panic(nil) is recoverable since Go 1.21 (slide p03) ==
recovered *runtime.PanicNilError: panic called with nil argument

Also try -repanic and -crash (both end the process on purpose).
```

With `-repanic` (the stack trace that follows is trimmed here):

```text
2026/11/26 08:55:33 ERROR unexpected state, re-panicking panic="ledger invariant violated: debits != credits"
panic: ledger invariant violated: debits != credits [recovered, repanicked]

goroutine 1 [running]:
main.rePanic.func1()
...
```

With `-crash`:

```text
fatal error: concurrent map writes

goroutine 11 [running]:
...
```

- `runTask(divide)` shows that runtime errors are panics too, and that a boundary can turn them into ordinary `error` values for its caller.
- Section 2 is the subtle one. In the first case `tryRecover` is itself the deferred function. In the second it is called by the deferred closure, one frame too deep, so its `recover` returns nil.
- Without middleware, the first request fails with `EOF` because net/http recovered the panic, logged it to `Server.ErrorLog` (silenced here) and closed the connection. The second request on the same server succeeds. With middleware, the client gets a real response instead of a dropped connection.
- Before Go 1.25, a re-panicked value appeared twice in the crash report. Now it appears once, marked `[recovered, repanicked]`.
- In `-crash`, each goroutine has a deferred `recover`, and none of them runs. `fatal error` is not a panic.

## Notes and corrections

- **Slide p07:** `log.Critical` doesn't exist in the standard library (`undefined: log.Critical`), and `slog` has no critical level either. Use `slog.Error`. A lost database connection is an operational error to return and retry, not a reason to re-panic. Re-panic only when continuing could corrupt state.
- **Slide p05:** `net/http` already recovers handler panics per connection, so one bad request doesn't take down the API. The middleware's real value is a proper 500 instead of a dropped connection, and the stack in your structured logs. A panic in a goroutine that a handler starts still crashes the whole process. Use `http.StatusInternalServerError` rather than `500`, re-panic `http.ErrAbortHandler`, and don't name the recovered `any` value `err`.
- **Slide p03:** `recover` must be called directly by the deferred function; from a helper it returns nil. Since Go 1.21, `panic(nil)` panics with `*runtime.PanicNilError`, so `if r := recover(); r != nil` no longer misses it. The line after `panic` on the slide is unreachable, and `go vet` reports it as `unreachable code`.
- **Slide p04:** fatal errors such as concurrent map writes, an all-goroutines-asleep deadlock, running out of memory and stack exhaustion can't be recovered at all.

## Slides

[026-recovery-strategies.pdf](../../slides/026-recovery-strategies.pdf)
