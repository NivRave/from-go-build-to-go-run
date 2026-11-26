# Part 051: The art of the graceful shutdown

Day 26 · Week 4 · Ensuring your service exits as cleanly as it runs.

## What this shows

- An `App` with the three layers from slide p06: an HTTP server with a slow (1 s) `POST /orders` handler, a worker pool started with `wg.Go`, and a fake DB with `Close()`. A request is sent before the shutdown begins.
- The root context cancelled with a cause after 300 ms (`context.WithCancelCause(..., errors.New("simulated SIGTERM"))`), standing in for the signal so the demo works on every OS.
- `Run(ctx)` tearing down in reverse order of start-up, synchronously: log `context.Cause`, `server.Shutdown` with a 5 s limit (falling back to `server.Close()` on `DeadlineExceeded`), wait for the workers with a timeout, close the DB, say goodbye.
- The in-flight request still completing with `200 OK` during the drain, and its job being saved before the DB closes.
- `-wait`: the real thing, with `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` and `stop()` called after the first signal so a second Ctrl+C kills the process.

## Run

```sh
go run ./examples/051-graceful-shutdown
go test ./examples/051-graceful-shutdown
go run ./examples/051-graceful-shutdown -wait
```

With `-wait`, send a request from another terminal (`curl -X POST http://127.0.0.1:<port>/orders`, using the URL it prints) and press Ctrl+C within a second to watch it drain. The log then shows Go 1.26's signal cause, `shutdown started: interrupt signal received`.

## What to look for

Expected output of `go run`:

```text
== 1. Start the app and send a slow request (slides p03, p06) ==
client: POST /orders (the handler takes 1s)
app: serving, 3 workers started

== 2. Simulated SIGTERM after 300 ms: ordered shutdown (slides p04-p08) ==
app: shutdown started: simulated SIGTERM
app: [1/4] http: no new connections, draining in-flight requests (limit 5s)
app: handler: order 1 processed, replying 200
app: [1/4] http: drained
app: [2/4] workers: all 3 stopped
app: [3/4] db: closed (orders saved: 1)
app: [4/4] goodbye

== 3. The in-flight request finished during the drain (slide p05) ==
client: 200 OK "order 1 accepted"
db: saved=1 closed=true
```

- The handler finishes *after* the shutdown started and before "drained": `Shutdown` closed the listener at once but waited for the active request.
- The workers range over the job queue until it is closed, instead of stopping on `ctx`. The job enqueued by the in-flight request is still saved, and only then does the DB close. Closing the DB first would make that save fail.
- `TestRunForcesCloseAfterTimeout` uses a one-minute handler. Under `testing/synctest` it shows `Shutdown` giving up after exactly 5 s of fake time, `server.Close()` cutting the request, and the DB still being closed. All three tests run in milliseconds because the HTTP traffic goes over an in-memory `net.Pipe` listener and time is fake.

## Notes and corrections

- **Slide p07:** calling `Shutdown` in a goroutine that nobody waits for lets `main` return (and `defer cancel()` cancel the drain) before the requests finish. `Shutdown` already honours its context's deadline, so call it synchronously and fall back to `server.Close()` when it returns `context.DeadlineExceeded`.
- **Slide p05:** `log.Fatalf` exits without running deferred calls and skips the remaining teardown steps; log the error and continue.
- **Slide p05:** a 30 s shutdown timeout equals Kubernetes' default `terminationGracePeriodSeconds`, so SIGKILL races it. Use less, such as 20-25 s.
- **Slide p05:** `ListenAndServe` returns `ErrServerClosed` as soon as `Shutdown` starts, so the program must wait for `Shutdown` to return, not for `ListenAndServe`.
- **Slide p05:** `Shutdown` does not wait for hijacked connections such as WebSockets; use `RegisterOnShutdown` to tell them to close.
- **Slide p05:** zero-downtime deploys also need the pod to stop receiving traffic first (failing readiness, or a preStop sleep), because endpoints are removed asynchronously.
- **Slide p04:** the title says `context.WithCancel`, but the code uses `signal.NotifyContext`, which is the better choice. Call `stop()` once `ctx.Done()` fires so a second Ctrl+C gets the default behaviour. Since Go 1.26, `context.Cause(ctx)` names the signal, for example `interrupt signal received`.
- **Slide p04:** "Day 20" means the context days, Parts 039 and 040, not Part 020.
- **Slide p02:** after the grace period, SIGKILL comes from the kubelet and container runtime rather than the OS itself.

## Slides

[051-graceful-shutdown.pdf](../../slides/051-graceful-shutdown.pdf)
