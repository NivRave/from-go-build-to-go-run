# Part 047: Dependency injection: The Go way

Day 24 · Week 4 · Building flexible, testable systems without the "magic" frameworks.

## What this shows

- A `Store` interface defined by its consumer (`Service`), an in-memory `MemStore`, and a `LoggingStore` decorator that adds logging without touching either.
- `NewService(store Store, opts ...Option) (*Service, error)`: a constructor that rejects a nil store, with functional options `WithLogger(*slog.Logger)` and `WithClock(func() time.Time)` for the optional dependencies.
- Explicit wiring in `main` (the composition root): `MemStore` → `LoggingStore` → `Service`, with a logger and a fixed clock injected.
- In `service.go`, a comment contrasting `http.Server` (configured through exported struct fields) with a functional-options API.
- Tests that inject a fake store, a fixed clock and a buffer-backed logger, run with `t.Parallel()`, and touch no globals.

## Run

```sh
go run ./examples/047-dependency-injection
go test ./examples/047-dependency-injection
```

## What to look for

Expected output of `go run`:

```text
== 1. A consumer-defined Store, MemStore and a LoggingStore decorator (slide p03) ==
level=INFO msg=store.Save id=greeting bytes=5 err=<nil>
level=INFO msg=store.Load id=greeting bytes=5 err=<nil>
Load(greeting) = "hello", err = <nil>
level=INFO msg=store.Load id=missing bytes=0 err="not found"
Load(missing): errors.Is(err, ErrNotFound) = true

== 2. A validating constructor with functional options (slides p04-p05) ==
NewService(nil): err = service: store is required
NewService(store, WithClock(nil)): err = service: WithLogger and WithClock need non-nil values
NewService(store): ok = true, err = <nil>

== 3. Explicit wiring in main, the composition root (slide p07) ==
level=INFO msg=store.Save id=n1 bytes=82 err=<nil>
level=INFO msg="note created" id=n1
level=INFO msg=store.Load id=n1 bytes=82 err=<nil>
GetNote(n1) = "wire dependencies in main" created 2026-11-26T09:30:00Z
level=INFO msg=store.Load id=n2 bytes=0 err="not found"
GetNote(n2): err = load note n2: not found, errors.Is(err, ErrNotFound) = true
```

- `MemStore` and `LoggingStore` never mention `Store`; they satisfy it implicitly. The `store.*` log lines come from the decorator, which was added purely in the wiring.
- The constructor turns a missing dependency into an error at startup, instead of a nil-pointer panic on the first request.
- The timestamp is stable because the clock is injected. Tests do the same with their own fixed clock, and capture logs in their own buffer, so they all run in parallel.
- `ErrNotFound` survives two layers of wrapping (`load note n2: not found`), so callers can still check it with `errors.Is`.

## Notes and corrections

- **Slide p05:** `http.Server` is configured through exported struct fields, not functional options, and has no constructor. gRPC is a real example of functional options (`grpc.NewServer(opts...)`). The slide also uses `Option` and `WithTimeout` without defining them; here `type Option func(*Service)` is spelled out.
- **Slide p08:** `github.com/google/wire` was archived by Google in August 2025 and is no longer maintained. Community forks exist; `go.uber.org/fx` is an alternative that wires at run time using reflection.
- **Slide p04:** Go can't force use of a constructor; a struct literal is always legal where the type is visible. A constructor can only encourage valid state, so validate in it and return an error. (A typed nil pointer such as `(*MemStore)(nil)` isn't caught by a `store == nil` check, as one of the tests shows.)
- **Slide p07:** `http.ListenAndServe(":80", ...)` discards its error and runs with no timeouts (see Part 043), and binding port 80 needs privileges on Unix. `initDB()` isn't defined on the slide.
- **Slide p06:** package-level state makes parallel tests harder, not impossible: tests that share a global have to be serialised.

## Slides

[047-dependency-injection.pdf](../../slides/047-dependency-injection.pdf)
