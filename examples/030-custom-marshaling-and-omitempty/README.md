# Part 030: Custom marshaling & omitempty

Day 15 · Week 3 · When "zero" means "missing".

## What this shows

- The omitempty collision: `Sensor{Temperature: 0}` marshals as `{}`. The pointer fix, written with Go 1.26's `new(0)`, gives `{"temp":0}`, and nil gives `{}`. On the way in, a missing key and `null` both leave the pointer nil.
- Go 1.24's `omitzero` on a `time.Time` field and on a `Price` type with an `IsZero()` method, next to `omitempty`, which keeps `"0001-01-01T00:00:00Z"` and the zero struct.
- `UnixTime` with `MarshalJSON`/`UnmarshalJSON` round-tripping a JSON **number**, treating `null` as a no-op (the slide's version turns `null` into 1970). The built-in `json:",string"` option for quoted legacy numbers and booleans.
- The alias pattern on `User` with a value-receiver `MarshalJSON`, and the pointer-receiver gotcha: `json.Marshal(u)` and `json.Marshal(&u)` produce different output.
- The same instant marshalled in a fixed EST zone and in UTC.
- Benchmarks of `json.Marshal` against the slide's `MarshalWithPool` and a `json.Encoder` writing straight to `io.Discard`.

## Run

```sh
go run ./examples/030-custom-marshaling-and-omitempty
go test ./examples/030-custom-marshaling-and-omitempty
go test -run='^$' -bench=. -benchmem ./examples/030-custom-marshaling-and-omitempty
```

## What to look for

Expected output of `go run`:

```text
== 1. The zero-value collision and the pointer fix (slides p02-p03) ==
Sensor{Temperature: 0}       {}
NullableSensor{new(0)}       {"temp":0}
NullableSensor{nil}          {}
decode {}             Temperature = nil
decode {"temp":null}  Temperature = nil
decode {"temp":0}     Temperature = 0

== 2. omitzero (Go 1.24) vs. omitempty ==
Event, nothing set:          {"name":"unset","at_legacy":"0001-01-01T00:00:00Z","price_empty":{"cents":0,"currency":""}}
Event, all set (0 EUR):      {"name":"free","at":"2026-01-01T00:00:00Z","at_legacy":"2026-01-01T00:00:00Z","price":{"cents":0,"currency":"EUR"},"price_empty":{"cents":0,"currency":"EUR"}}

== 3. Custom Marshaler and Unmarshaler (slides p04-p05) ==
marshal:                     {"at":1767225600}
round trip equal:            true
null, this UnmarshalJSON:    2026-01-01T00:00:00Z
null, slide p05 version:     1970-01-01T00:00:00Z
",string" in:                {Count:42 Active:true}
",string" out:               {"count":"42","active":"true"}

== 4. The alias pattern and the pointer-receiver gotcha (slide p06) ==
value receiver, Marshal(u):  {"last_seen":1767225600,"name":"gopher"}
value receiver, Marshal(&u): {"last_seen":1767225600,"name":"gopher"}
ptr receiver, Marshal(u):    {"name":"gopher","last_seen":"2026-01-01T00:00:00Z"}
ptr receiver, Marshal(&u):   {"last_seen":1767225600,"name":"gopher"}

== 5. time.Time and time zones (slide p07) ==
in EST:                      "2025-12-31T19:00:00-05:00"
in UTC:                      "2026-01-01T00:00:00Z"
same instant:                true
with 1.5 s:                  "2026-01-01T00:00:01.5Z"
```

Benchmark shape on one machine (numbers vary by machine):

```text
BenchmarkMarshal            ~690 ns/op   304 B/op   3 allocs/op
BenchmarkMarshalWithPool    ~750 ns/op   304 B/op   3 allocs/op
BenchmarkEncoderToDiscard   ~630 ns/op   144 B/op   2 allocs/op
```

- `omitzero` drops `at` and `price` when they are unset but keeps a price of 0 EUR, because `Price.IsZero` decides. `omitempty` never omits a struct, so `at_legacy` and `price_empty` are always sent. Neither tag solves "0 is a valid reading" for an `int`; that still needs the pointer.
- `null, slide p05 version` is 1970-01-01: decoding `null` into an `int64` is a silent no-op, so the slide's `UnmarshalJSON` builds a time from 0.
- `ptr receiver, Marshal(u)` is the gotcha: `json.Marshal` receives a non-addressable copy, can't call `(*PtrUser).MarshalJSON`, and quietly uses the default encoding. A value receiver works either way.
- `MarshalWithPool` is no faster than `json.Marshal` and allocates the same. Only the Encoder, writing straight to its destination, skips the result slice. `TestTrailingNewline` checks the other difference: the Encoder appends `\n`.

## Notes and corrections

- **Slide p08:** `json.Marshal` already keeps its internal encode buffers in a `sync.Pool` and returns one copied slice, which is what `MarshalWithPool` re-implements, so it gains little and adds a trailing `\n`. The real saving is `json.NewEncoder(w).Encode(v)` straight into the `http.ResponseWriter` or other `io.Writer`. If you do pool buffers, don't `Put` very large ones back, or the pool keeps that memory alive; this example's version skips buffers over 64 KiB. A hot `json.encodeState` in pprof is the already-pooled buffer, not a reason to add a pool.
- **Slide p06:** a pointer-receiver `MarshalJSON` is skipped when marshalling a non-addressable value, so `json.Marshal(user)` silently ignores it; use a value receiver with `(*Alias)(&u)`, or always pass a pointer. `(*Alias)(u)` is a zero-copy pointer conversion; it copies nothing. The outer `LastSeen` hides the embedded one only because both use the JSON name `last_seen`.
- **Slide p05:** `UnmarshalJSON` should treat `null` as a no-op (`if string(b) == "null" { return nil }`), as the standard library's own types do. For legacy APIs that quote values (`"42"`, `"true"`), the `json:",string"` tag option needs no custom code.
- **Slide p04:** the method emits a JSON number (`1767225600`), not a "numeric timestamp string". `strconv.AppendInt(nil, t, 10)` avoids `fmt`. `type UnixTime time.Time` has none of `time.Time`'s methods.
- **Slides p02/p03:** Go 1.24 added the `omitzero` tag option, which omits zero values and anything whose `IsZero()` returns true, and so fixes `time.Time{}` and zero structs. Go 1.26's `new(0)` replaces the `temp := 0; &temp` dance. The real output is `{"temp":0}`, without a space. In the experimental `encoding/json/v2` (`GOEXPERIMENT=jsonv2`, Go 1.25+), `omitempty` means "omit if the JSON value is empty" (`""`, `null`, `[]`, `{}`), so `Sensor{0}` marshals as `{"temp":0}` there.
- **Slide p07:** `encoding/json` writes `time.Time` as RFC 3339 with nanoseconds and trims trailing zeros (`...00:01.5Z`). An RFC 3339 string with an offset is unambiguous; sending UTC is a good convention rather than a correctness requirement.

## Slides

[030-custom-marshaling-and-omitempty.pdf](../../slides/030-custom-marshaling-and-omitempty.pdf)
