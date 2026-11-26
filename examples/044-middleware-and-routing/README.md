# Part 044: Mastering middleware & routing chains

Day 22 · Week 4 · Architecting clean, reusable logic for every request.

## What this shows

- A `Logging` middleware that records the status code through a `statusRecorder`, which implements `Unwrap()` so `http.ResponseController` (here, `Flush`) still reaches the real writer. It logs through `slog`.
- `Recovery` (a clean 500 plus a log line; `http.ErrAbortHandler` is re-panicked) and `Auth` (a bearer token becomes a user ID in the context under an unexported key, read back with `UserFrom(ctx)`), driven through an `httptest.NewServer`.
- A corrected `Chain` that applies middleware in the declared order, outermost first, next to the slide's version, which reverses it.
- Go 1.22 routing: `GET /users/{id}` with `PathValue`, and an `/api/v1/` sub-mux mounted with `http.StripPrefix`.

## Run

```sh
go run ./examples/044-middleware-and-routing
go test ./examples/044-middleware-and-routing
```

## What to look for

Expected output of `go run` (the demo logger drops `time` and `duration` so the output is stable):

```text
== 1. Logging with a status recorder (slide p03) ==
handler: Flush through the wrapper, err = <nil>
level=INFO msg=request method=GET path=/coffee status=418

== 2. Recovery and Auth through the full stack (slides p07-p08) ==
level=INFO msg=request method=GET path=/users/7 status=401
-> /users/7: 401 "Unauthorized"
level=INFO msg=request method=GET path=/users/7 status=401
-> /users/7: 401 "Unauthorized"
level=ERROR msg="panic recovered" value="database handle is nil" path=/panic
level=INFO msg=request method=GET path=/panic status=500
-> /panic: 500 "Internal Server Error"
-> /abort: connection aborted, no response

== 3. Chain order (slide p04) ==
Chain(h, A, B):        A-before B-before handler B-after A-after
SlideChain(h, A, B):   B-before A-before handler A-after B-after

== 4. Routes: method patterns and a sub-mux (slides p05-p06) ==
level=INFO msg=request method=GET path=/users/7 status=200
-> /users/7: 200 "user 7 (requested by user-42)"
level=INFO msg=request method=GET path=/api/v1/users/9 status=200
-> /api/v1/users/9: 200 "v1 user 9"
level=INFO msg=request method=GET path=/nope status=404
-> /nope: 404 "404 page not found"
```

- The logger sees `418`, `401`, `500` and `404` only because `statusRecorder` intercepts `WriteHeader` and `Write`. A plain `func(next http.Handler)` wrapper can't see the status at all.
- The stack is `Chain(mux, Logging, Recovery, Auth)`. Logging is outermost, so it also logs the 500 that Recovery writes. `/abort` panics with `http.ErrAbortHandler`; Recovery re-panics it, net/http drops the connection silently, and the unwinding panic skips the log line.
- `Chain` gives `A-before B-before handler B-after A-after`: the first middleware listed is the outermost. The slide's loop gives the reverse.
- `/api/v1/users/9` is handled by the sub-mux, which sees `/users/9` after `StripPrefix`; its `PathValue("id")` comes from the pattern it matched. The log line still shows the full path, because Logging runs before the prefix is stripped.

## Notes and corrections

- **Slide p04:** ranging forward over the middleware makes the **last** one the outermost, so `Chain(h, A, B, C)` runs C first. To get "the order they are defined, outer first", iterate in reverse, as `Chain` does here.
- **Slide p08:** `net/http` already recovers handler panics per connection, so the process survives without this middleware; the client just gets an aborted connection. The middleware's value is a proper 500 plus structured logging. It should re-panic `http.ErrAbortHandler`, and it can't catch panics in goroutines the handler starts.
- **Slide p07:** use an unexported key type (`type ctxKey struct{}`) with a typed accessor (`UserFrom`) so keys can't collide. The context is request-scoped, not "secure": any code holding it can read the value.
- **Slide p03:** capturing the status code needs a wrapping `ResponseWriter`. Give it an `Unwrap()` method so `http.ResponseController` keeps working through it (Go 1.20+).
- **Slides p05/p06:** method and wildcard patterns need Go 1.22+ **and** `go 1.22` or later in `go.mod`; with an older `go` line, the old matching rules (`GODEBUG=httpmuxgo121=1`) apply. `listUsers` isn't defined on the slide.

## Slides

[044-middleware-and-routing.pdf](../../slides/044-middleware-and-routing.pdf)
