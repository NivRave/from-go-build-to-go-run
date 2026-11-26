# Part 029: JSON: encoding & decoding

Day 15 · Week 3 · Standard library vs. performance.

## What this shows

- `json.Marshal` and `json.Unmarshal` on a tagged `User`, with every error checked, and that tags don't make key matching case-sensitive: `{"EMAIL":...}` still fills the field tagged `json:"email"`.
- A `json.Decoder` reading newline-delimited JSON with `for dec.More()`, and the trailing-data difference: `Unmarshal` rejects `{"id":1} garbage`, a single `Decode` accepts it, and a second `Decode` that must return `io.EOF` catches it.
- Mass assignment: `DisallowUnknownFields` still lets `"is_admin":true` into a `User`, because `IsAdmin` is a known field. A `CreateUserRequest` DTO is the fix, and under `DisallowUnknownFields` it rejects the field.
- A `json.RawMessage` envelope that dispatches on `type` to two payload structs.
- The float64 trap: 2^53 + 1 decoded into `map[string]any` comes back as 2^53; `UseNumber` and an `int64` struct field are exact.
- Benchmarks that decode the same document into a struct and into `map[string]any`.

## Run

```sh
go run ./examples/029-json
go test ./examples/029-json
go test -run='^$' -bench=. -benchmem ./examples/029-json
GOEXPERIMENT=jsonv2 go test -run='^$' -bench=. -benchmem ./examples/029-json
```

PowerShell form of the last command:

```powershell
$env:GOEXPERIMENT="jsonv2"; go test -run='^$' -bench=. -benchmem ./examples/029-json; Remove-Item Env:GOEXPERIMENT
```

## What to look for

Expected output of `go run`:

```text
== 1. Marshal vs. Unmarshal (slide p02) ==
marshal:   {"id":1,"email":"gopher@example.com","is_admin":false}
unmarshal: {ID:1 Email:gopher@example.com IsAdmin:false}
uppercase keys still match: {ID:2 Email:LOUD@example.com IsAdmin:false}

== 2. Decoding a stream of values (slide p03) ==
decoded user 1 <a@example.com>
decoded user 2 <b@example.com>
decoded user 3 <c@example.com>
Unmarshal:                 invalid character 'g' after top-level value
Decode:                    <nil>
decodeSingle:              unexpected data after the JSON value
decodeSingle, clean input: <nil>

== 3. DisallowUnknownFields vs. mass assignment (slide p04) ==
User, strict:              err=<nil> IsAdmin=true  <- mass assignment
CreateUserRequest:         err=<nil> IsAdmin=false
CreateUserRequest, strict: err=strict decode failed: json: unknown field "is_admin"

== 4. Delayed decoding with json.RawMessage (slide p05) ==
user.created: {ID:7 Email:new@example.com}
payment.received: {AmountCents:1999 Currency:EUR}
error: unknown event type "user.deleted"

== 5. The float64 trap (slide p06) ==
map[string]any: 9.007199254740992e+15 (float64) -> int64 9007199254740992
UseNumber:      9007199254740993 (json.Number) -> int64 9007199254740993, err=<nil>
int64 field:    9007199254740993
```

Benchmark shape on one machine (numbers vary by machine and Go version):

```text
                      encoding/json v1                GOEXPERIMENT=jsonv2
BenchmarkDecodeStruct ~2.2 µs/op  552 B/op 17 allocs  ~1.4 µs/op  224 B/op  4 allocs
BenchmarkDecodeMap    ~3.5 µs/op 1336 B/op 43 allocs  ~4.0 µs/op 1176 B/op 35 allocs
```

- `Decode: <nil>` is the surprise: a Decoder stops after the first value and never looks at the rest, so a request body with junk after the JSON passes unless you check for `io.EOF`, as `decodeSingle` does.
- `IsAdmin=true` under strict decoding shows that `DisallowUnknownFields` is about *unknown* keys. Only a type without the privileged field (or `json:"-"` on it) keeps a client from setting it.
- 9007199254740993 becomes 9007199254740992 in the map: every JSON number in `map[string]any` is a `float64`, which is exact only up to 2^53.
- Decoding into a struct allocates less and runs faster than into a map, and under the jsonv2 experiment the struct case improves the most.

## Notes and corrections

- **Slide p03:** `Decoder.Decode` reads the whole next value into memory before decoding it, so it isn't incremental for one large object; a 100 MB object still needs at least 100 MB. Its real wins are skipping `io.ReadAll` and reading sequences of values. Wrap request bodies with `http.MaxBytesReader` to bound memory, and reply with a generic message and `http.StatusBadRequest` rather than echoing `err.Error()` to the client.
- **Slide p04:** `DisallowUnknownFields` doesn't stop mass assignment of fields that exist in the struct. Use an input DTO without the privileged fields, or tag them `json:"-"`. `DisallowUnknownFields` exists only on `Decoder`, not on `json.Unmarshal`.
- **Slide p02:** tags don't turn off case-insensitive matching in `encoding/json` v1; an exact match is tried first, then a case-insensitive one. Tags are worth using for the API contract, not for speed. `data, _ := json.Marshal(user)` discards an error that can happen (for example with channels, functions or unsupported map keys).
- **Slide p08:** Go 1.25 added `encoding/json/v2` and `encoding/json/jsontext` as an experiment (`GOEXPERIMENT=jsonv2`); under it the v1 API runs on the new implementation. Try that before a third-party library. `json-iterator/go` is effectively unmaintained.
- **Slide p06:** precision loss starts above 2^53 (9007199254740992). Besides `UseNumber`, a struct field typed `int64` is exact, and `json:",string"` handles IDs sent as strings.
- **Slide p05:** a `RawMessage` is still scanned for validity and copied, so delaying the decode is not free.

## Slides

[029-json.pdf](../../slides/029-json.pdf)
