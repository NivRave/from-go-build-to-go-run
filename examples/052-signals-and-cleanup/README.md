# Part 052: Deep dive: Signals & resource cleanup

Day 26 · Week 4 · Mastering the mechanics of a clean exit.

## What this shows

- The `run()` pattern: `main` only calls `run(ctx, os.Args[1:], os.Stdout)` and chooses the exit code, while `run` owns every `defer`.
- Waiting for workers with a safety net: one worker ignores `ctx` and hangs, so the "zombie timeout" path logs and cleanup proceeds anyway.
- Idempotent teardown with `sync.OnceValue(func() error {...})`, called from two goroutines at once. It closes a channel (a second close would panic, shown with a recovered panic) and a fake DB, and both callers get the same result.
- Releasing the signal handler after cleanup with `signal.Stop` (via `NotifyContext`'s `stop()`) and `signal.Reset`.
- `-wait`: the real two-stage Ctrl+C. The first press starts the graceful shutdown; `stop()` then restores the default handler, so a second press kills the process. The default mode simulates the signal by cancelling the context.

## Run

```sh
go run ./examples/052-signals-and-cleanup
go test ./examples/052-signals-and-cleanup
go run ./examples/052-signals-and-cleanup -wait
```

With `-wait`, press Ctrl+C once and the hanging worker keeps cleanup busy for 5 s; press it again within that time and the process exits immediately through Go's default handler.

## What to look for

Expected output of `go run`:

```text
== 1. Start workers; one will hang (slide p03) ==
simulating a signal in 200ms (use -wait for a real Ctrl+C)
workers: 3 started (worker 3 ignores ctx)

== 2. Signal received: stop the workers, with a safety net (slides p02, p03) ==
signal: simulated SIGINT
workers: 2 of 3 stopped
zombie timeout: gave up after 500ms, proceeding with cleanup

== 3. Idempotent teardown with sync.OnceValue (slide p05) ==
teardown: closing events channel
teardown: closing db
signal path got: <nil>
main path got:   <nil>
teardown body ran 1 time(s)
without Once, a second close(events) panics: close of closed channel

== 4. Release the signal handler (slide p04) ==
default mode registered no handler; with -wait, stop() calls signal.Stop and run then calls signal.Reset here

== 5. Return to main (slide p08) ==
run: deferred calls ran before main chose the exit code
main: exited cleanly. Goodbye!
```

- A bare `wg.Wait()` would block forever on the zombie worker. The timeout lets the program log it and carry on closing everything else.
- Both teardown callers print, but "closing db" appears once: `sync.OnceValue` runs the body once and hands its return value to every caller. The fake DB returns an error on a second `Close`, and no caller ever sees it.
- `run`'s deferred line prints before `main`'s goodbye. Had `run` called `os.Exit`, it would never print.
- The tests call `run` with a cancelled context, wait for it to return, and check that cleanup ran exactly once. Under `testing/synctest` the 500 ms zombie timeout takes no real time, and the test checks that it lasted exactly 500 ms of fake time.

## Notes and corrections

- **Slide p07:** the test returns right after `close(stop)`, so `shutdown()` may not have run yet, and it asserts nothing. Inject a context (`run(ctx, ...)`), wait for `run` to return, then assert, as `run_test.go` does.
- **Slide p06:** `sigs :=` is declared twice in one scope, which does not compile (`no new variables on left side of :=`). The claim is right: `signal.Notify` never blocks, so an unbuffered channel drops signals, and `go vet` reports `misuse of unbuffered os.Signal channel as argument to signal.Notify`.
- **Slide p05:** the global `db` contradicts Part 047; this example passes its resources explicitly.
- **Slide p05:** since Go 1.21, `sync.OnceValue` (or `sync.OnceFunc`) is the concise form, and it returns the close error to every caller instead of discarding it.
- **Slide p05:** `*sql.DB.Close` is already safe to call twice; the double-close panic applies to channels, which is what this example closes.
- **Slide p04:** a second Ctrl+C uses the default handler only if nothing else (another channel or `NotifyContext`) is still registered for that signal. `signal.Reset` restores the default explicitly.
- **Slide p08:** `os.Exit` skips deferred calls, which is exactly why the `run()` pattern exists. The final `os.Exit(0)` is redundant, since returning from `main` exits with status 0.
- Windows cannot send `os.Interrupt` to its own process, which is another reason to inject a context in tests rather than signal yourself. A real Ctrl+C in a console works on every OS.

## Slides

[052-signals-and-cleanup.pdf](../../slides/052-signals-and-cleanup.pdf)
