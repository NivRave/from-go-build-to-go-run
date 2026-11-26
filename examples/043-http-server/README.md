# Part 043: The HTTP server

Day 22 · Week 4 · Building production-ready entry points in Go.

## What this shows

- A local `http.NewServeMux()` with a struct handler (`UserHandler` with an injected fake store), a `HandlerFunc` route and `/health`, exercised directly with `httptest.NewRecorder`.
- A configured `http.Server` (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) serving on `127.0.0.1:0`, called by an `http.Client`, then shut down with `Shutdown`, treating `http.ErrServerClosed` as a clean stop.
- A slowloris client (a raw TCP connection that sends nothing) being cut off by a 200 ms `ReadHeaderTimeout`.
- `-serve`: the same server on `localhost:8080` until Ctrl+C, with graceful shutdown.

## Run

```sh
go run ./examples/043-http-server
go test ./examples/043-http-server
go run ./examples/043-http-server -serve   # then: curl 'localhost:8080/users?id=1'
```

## What to look for

Expected output of `go run` (the slowloris time can vary by a few milliseconds):

```text
== 1. A local ServeMux with Handler and HandlerFunc (slides p03-p05) ==
GET /             -> 200 Welcome to the API
GET /health       -> 200 ok
GET /users?id=1   -> 200 user 1: Ada
GET /users?id=99  -> 404 user not found

== 2. A configured http.Server on 127.0.0.1:0 (slides p06-p07) ==
GET /health     -> 200 OK ok
GET /users?id=2 -> 200 OK user 2: Grace
Serve returned "http: Server closed"; errors.Is(err, http.ErrServerClosed) = true

== 3. Slowloris vs ReadHeaderTimeout (slide p07) ==
ReadHeaderTimeout = 200ms
server closed the silent connection after ~200ms (read error is EOF: true)
```

- Section 1 never opens a port. Because the mux is local, a test can build one with a fake store and call `ServeHTTP` directly, which is what `handlers_test.go` does.
- `homeHandler` never calls `WriteHeader`, and the response is still 200: the first `Write` sends it implicitly.
- `Serve` returns an error even on a clean shutdown. `errors.Is(err, http.ErrServerClosed)` is how you tell "stopped as asked" from "failed".
- The silent connection is closed by the server after `ReadHeaderTimeout`. With the zero-value timeouts that `http.ListenAndServe` uses, it would be held open, together with its goroutine, indefinitely.

## Notes and corrections

- **Slide p09:** the summary slide (fan-out, fan-in, backpressure, WaitGroups) belongs to the worker-pools part and doesn't summarise this one. This part's takeaways are: `Handler`/`HandlerFunc`, a local mux, and an `http.Server` with explicit timeouts.
- **Slide p07:** `ReadHeaderTimeout` is the field aimed at slowloris clients, and the slide's configuration leaves it out. "DefaultServer" isn't a real identifier: `http.ListenAndServe` builds a plain `&http.Server{Addr, Handler}`, so every timeout is zero (unlimited).
- **Slide p07:** `WriteTimeout` also cuts off streaming and SSE responses. Since Go 1.20 a handler can relax it for itself with `http.NewResponseController(w).SetWriteDeadline`.
- **Slide p06:** `ListenAndServe` always returns a non-nil error; after `Shutdown` it is `http.ErrServerClosed`, so `log.Fatal(err)` would fire on a normal stop. Check `!errors.Is(err, http.ErrServerClosed)`. `healthHandler` is defined here.
- **Slide p08:** one goroutine per connection is true for HTTP/1.x. With HTTP/2, each stream is served in its own goroutine.
- **Slide p04:** `w.WriteHeader(http.StatusOK)` is redundant; the first `Write` sends 200 implicitly.

## Slides

[043-http-server.pdf](../../slides/043-http-server.pdf)
