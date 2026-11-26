# Part 049: Slog: Structured logging

Day 25 · Week 4 · Moving from text for humans to data for machines.

## What this shows

- `slog.NewJSONHandler` with `HandlerOptions{Level: &levelVar, ReplaceAttr: ...}`, logging with loose key-value pairs and with `LogAttrs`. `ReplaceAttr` removes `time` so the output is deterministic.
- `slog.Group` for nested attributes, `logger.With(...)` for attributes on every line, and `WithGroup` for scoping.
- A `ContextHandler` that wraps any `slog.Handler` and adds `request_id` from the context (stored under an unexported key), and keeps working after `With`.
- Flipping a `slog.LevelVar` from Info to Debug at run time, so Debug lines start to appear.
- `slog.SetDefault`, after which `log.Printf` comes out as JSON too.
- Go 1.26's `slog.NewMultiHandler` sending one record to a text handler and a JSON handler.
- A benchmark of loose arguments versus `LogAttrs` against `slog.DiscardHandler` and against a real JSON handler.

## Run

```sh
go run ./examples/049-slog
go test ./examples/049-slog
go test -run='^$' -bench=. -benchmem ./examples/049-slog
```

## What to look for

Expected output of `go run`:

```text
== 1. JSON handler: loose key-values vs LogAttrs (slides p03-p04) ==
{"level":"INFO","msg":"incoming_request","method":"GET","path":"/users","status":200}
{"level":"INFO","msg":"upload","size":1024,"user":"niv"}

== 2. Groups, With and WithGroup (slide p06) ==
{"level":"INFO","msg":"user_update","user":{"id":42,"email":"user@example.com"}}
{"level":"INFO","msg":"charged","service":"billing","amount_cents":999}
{"level":"INFO","msg":"request done","http":{"method":"GET","status":200}}

== 3. A ContextHandler adds request_id from the context (slide p05) ==
{"level":"INFO","msg":"fetching user","user_id":42,"request_id":"req-7f3a"}
{"level":"INFO","msg":"query ok","component":"db","rows":1,"request_id":"req-7f3a"}
{"level":"INFO","msg":"no context, no request_id"}

== 4. Changing the level at run time with slog.LevelVar (slide p08) ==
{"level":"INFO","msg":"serving","min_level":"INFO"}
{"level":"DEBUG","msg":"cache miss","key":"user:42"}
{"level":"INFO","msg":"serving","min_level":"DEBUG"}

== 5. slog.SetDefault also redirects the log package (slide p07) ==
{"level":"INFO","msg":"via slog.Info","who":"new code"}
{"level":"INFO","msg":"via log.Printf from legacy code"}

== 6. Fan-out with slog.NewMultiHandler (Go 1.26, slide p07) ==
level=INFO msg=fan-out sinks=2
{"level":"INFO","msg":"fan-out","sinks":2}
```

Benchmark shape (numbers vary by machine):

```text
BenchmarkLooseArgs/discard   ~55 ns/op   40 B/op   3 allocs/op
BenchmarkLooseArgs/json     ~700 ns/op   40 B/op   3 allocs/op
BenchmarkLogAttrs/discard    ~20 ns/op    0 B/op   0 allocs/op
BenchmarkLogAttrs/json      ~850 ns/op    0 B/op   0 allocs/op
```

- Both calls in section 1 produce the same JSON. The difference is only in how the arguments get there.
- In section 3 the request ID comes from the context, not from the call. `logger.Info` without a context has none. The handler overrides `WithAttrs` and `WithGroup`; without that, `logger.With(...)` would return the inner handler and lose the wrapper.
- Section 4 logs the same Debug call twice; only the one after `levelVar.Set(slog.LevelDebug)` is printed. Nothing is rebuilt or restarted.
- The `log.Printf` line in section 5 comes out as JSON at INFO level, because `SetDefault` redirects the classic `log` package to the new handler.
- The benchmark shows the boxing cost: three non-constant values passed as `...any` allocate 3 times per call (40 B), even when the handler is disabled and nothing is written. `LogAttrs` allocates nothing. With the full JSON handler the time difference is small, so the gain is modest; it matters on hot paths and when logs are mostly filtered out.

## Notes and corrections

- **Slide p06:** `JSONHandler` always writes `"time"` first unless a `ReplaceAttr` function removes it, so the real output is `{"time":"...","level":"INFO","msg":"user_update",...}`. The example uses `user@example.com` in place of the personal address on the slide.
- **Slide p08:** the slide says the level can change at run time without showing how: `var lvl slog.LevelVar`, `HandlerOptions{Level: &lvl}`, then `lvl.Set(slog.LevelDebug)`, as in section 4. Its subtitle, "Plugging in the Infrastructure", is the subtitle of Part 048.
- **Slide p04:** it's `LogAttrs` that avoids the `[]any` boxing; typed attributes passed to `Info` still go through `...any`. The gain is modest, as the benchmark shows. `slog.Attr` is a type, not a constructor, and `ctx` isn't defined in the snippet.
- **Slide p07:** `slog.SetDefault` also redirects the `log` package's output. Third-party code benefits only if it logs through `slog.Default()` (or the `log` package); libraries with their own loggers are unaffected. Go 1.26 added `slog.NewMultiHandler` for fan-out, and Go 1.24 added `slog.DiscardHandler`.
- **Slide p05:** the handler wrapper the slide describes is `ContextHandler` in `contexthandler.go`. With OpenTelemetry, the official bridge is `go.opentelemetry.io/contrib/bridges/otelslog`.
- **Slide p03:** until you call `SetDefault`, `slog.Default()` writes through the `log` package in its plain text format, not in Text or JSON handler format.

## Slides

[049-slog.pdf](../../slides/049-slog.pdf)
