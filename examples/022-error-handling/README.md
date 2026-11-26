# Part 022: Error handling - wrapping & context

Day 11 · Week 2 · Don't just return the error; add value to it.

## What this shows

- `LoadConfig` on a missing file wraps with `fmt.Errorf("load config: %w", err)`. `err == fs.ErrNotExist` is false, `errors.Is` is true, and the program falls back to defaults.
- `errors.As` into `*fs.PathError`, and the same lookup with Go 1.26's generic `errors.AsType`, instantiated with `*fs.PathError`.
- `errors.Join` of three closers (two failing), and one `fmt.Errorf` with two `%w` verbs. `errors.Is` finds every sentinel in both.
- Handle or return: three layers each add context, and only the top layer logs, once, with `slog`.
- Sanitising at the API edge: internal errors keep every detail for the logs, while the client gets a generic message and a request ID.
- `errgroup.WithContext` with `SetLimit(2)`: one fetch fails, the others observe `ctx.Done()`, and `Wait` returns the first error. `sync.WaitGroup.Go` is shown for contrast.

## Run

```sh
go run ./examples/022-error-handling
go test ./examples/022-error-handling
```

## What to look for

Expected output of `go run`:

```text
== 1. Wrap with %w, detect with errors.Is (slides p02-p03) ==
err starts with "load config: open ": true
err == fs.ErrNotExist:          false
errors.Is(err, fs.ErrNotExist): true
using defaults: {Port:8080 LogLevel:info}

== 2. Extract details with errors.As and errors.AsType (slide p04) ==
errors.As:     Op="open" Path=.../config.json
errors.AsType: Op="open" Path=.../config.json

== 3. Multiple errors: errors.Join and multiple %w (slide p06) ==
errors.Join message (one line per error):
redis: close: broken pipe
file: close: disk full
errors.Is(errs, ErrRedisClose): true
errors.Is(errs, ErrFileClose):  true
multiple %w: shutdown: redis: close: broken pipe; file: close: disk full
errors.Is finds both: true

== 4. Handle or return: log once, at the top (slide p05) ==
level=ERROR msg="checkout failed" request_id=req-0042 err="checkout 42: load order: query orders where id=42: connection reset by peer"

== 5. Sanitise at the API edge (slide p08) ==
internal (logs only): db update failed for user 7: dial tcp 10.0.0.5:5432: connect: connection refused
public (to client):   {"error":"Update failed. Please try again.","request_id":"req-0001"}
internal (logs only): find order 99: order not found
public (to client):   {"error":"Order not found.","request_id":"req-0002"}

== 6. Concurrent errors with errgroup (slide p07) ==
errgroup outcomes:
  https://a.example      context canceled
  https://bad.example    fetch https://bad.example: 503 service unavailable
  https://c.example      context canceled
  https://d.example      context canceled
g.Wait() returned: fetch https://bad.example: 503 service unavailable
wg.Go collected 2 errors by hand:
fetch https://bad.example: 503 service unavailable
fetch https://bad2.example: 503 service unavailable
```

- Section 1 doesn't print the error itself, because the OS part of the message differs between Linux, macOS and Windows. `errors.Is` is the portable way to ask "was it missing?".
- Section 4 is one log line for one failure, and the message reads like a stack: checkout, then load order, then the query, then the cause.
- In section 6, `a` was cancelled mid-flight when `bad` failed. `c` and `d` had to wait for a free slot (`SetLimit(2)`), and when they started the context was already cancelled. The whole section takes about 10 ms, not the 1 s each successful fetch would take.
- `wg.Go` runs the same work but has no error result and no shared cancellation. The example collects errors with a mutex and `errors.Join` by hand.

## Notes and corrections

- **Slide p07:** `url := url` is obsolete since Go 1.22, because each loop iteration has its own variable when `go.mod` says `go 1.22` or later. `g.SetLimit(n)` bounds concurrency, and the `ctx` from `errgroup.WithContext` is cancelled on the first error. `errgroup` lives in `golang.org/x/sync`, not the standard library. Go 1.25's `sync.WaitGroup.Go` removes the `Add`/`Done` boilerplate but still doesn't propagate errors.
- **Slide p04:** Go 1.26 added the generic `errors.AsType`. Instantiated with `*fs.PathError`, it returns the typed error and a bool, so the pre-declared `var perr *fs.PathError` target is no longer needed. `main.go` shows both forms.
- **Slide p02:** `"config: open %s: %w"` repeats information, because `*fs.PathError` already renders as `open <path>: <reason>`. The result would read `config: open config.yaml: open config.yaml: ...`. Wrap with `"load config: %w"` instead.
- **Slide p03:** since Go 1.20, errors form a tree rather than a chain (`Unwrap() []error` from `errors.Join` and multiple `%w`). `errors.Is` walks the whole tree and also honours a custom `Is(error) bool` method.
- **Slide p06:** one `fmt.Errorf` may contain several `%w` verbs (Go 1.20). `errors.Join` returns nil when every argument is nil, and its message puts each error on its own line.

## Slides

[022-error-handling.pdf](../../slides/022-error-handling.pdf)
