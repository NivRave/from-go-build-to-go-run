# Part 006: Advanced string tactics beyond the basics

Day 03 · Week 1 · Memory leaks, high-speed comparisons, and safe conversions.

## What this shows

- The substring leak, measured: a 10-byte substring of a 64 MB string keeps all 64 MB alive after a full GC, while `strings.Clone` of the same 10 bytes lets it go.
- What `strings.EqualFold` really compares: `"Go"`/`"GO"` match, `"ß"`/`"SS"` do **not**, and the Kelvin sign (U+212A) matches `"k"`.
- Zero-copy views with `unsafe`: `unsafe.Slice(unsafe.StringData(s), len(s))` and the inverse `unsafe.String(&b[0], len(b))`, used read-only, next to the copying `[]byte(s)`.
- Validating external data: `utf8.ValidString` rejects `"\xff\xfe"`, `strings.ToValidUTF8` sanitises it, and `range` decodes it as U+FFFD without panicking.
- `net.JoinHostPort` as the right tool for host:port: `"::1"` and `"8080"` become `[::1]:8080`, where `Sprintf("%s:%d")` produces an ambiguous `::1:8080`.
- Benchmarks: `ToLower(a) == ToLower(b)` vs `EqualFold`; `Sprintf` vs `+` vs `strings.Join` vs `strings.Builder`; `[]byte(s)` vs the unsafe view.

## Run

```sh
go run ./examples/006-advanced-string-tactics
go test ./examples/006-advanced-string-tactics
go test -run='^$' -bench=. -benchmem ./examples/006-advanced-string-tactics
```

## What to look for

Expected output of `go run` (the heap figures in section 1 can differ by a few MB between machines and runs):

```text
== 1. The substring memory leak and strings.Clone (slides p02-p03) ==
baseline:                          HeapInuse =   0 MB, len(kept) = 0
kept = large[:10]:                 HeapInuse =  64 MB, len(kept) = 10
kept = strings.Clone(large[:10]):  HeapInuse =   0 MB, len(kept) = 10

== 2. What strings.EqualFold really compares (slide p04) ==
"Go" vs "GO"           EqualFold=true  ToLower==ToLower=true
"\u00df" vs "SS"       EqualFold=false ToLower==ToLower=false
"\u212a" vs "k"        EqualFold=true  ToLower==ToLower=true

== 3. Zero-copy views with unsafe, read-only (slide p05) ==
copy  []byte(s):      senior Go (shares memory with s: false)
view  unsafe.Slice:   Senior Go (shares memory with s: true)
Println of the view:  [83 101 110 105 111 114]
view  unsafe.String:  built at run time (shares memory with buf: true)

== 4. Validating external data (slide p06) ==
validatePayload("ok") = <nil>
validatePayload("ok\xff\xfeok") = malformed UTF-8
strings.ToValidUTF8 -> "ok\ufffdok"
range over "\xff\xfe": 0:U+FFFD 1:U+FFFD

== 5. Building host:port (slide p07) ==
example.com  Sprintf: example.com:8080  JoinHostPort: example.com:8080
::1          Sprintf: ::1:8080          JoinHostPort: [::1]:8080
strings.Join([]string{part1, part2}, "-") = go-build
```

- Section 1 calls `runtime.GC()` before each reading. After `keepSubstring` returns, the only reference to the 64 MB array is the 10-byte `kept`, and that is enough to keep it all in use. After `keepClone`, the same 10 bytes cost almost nothing.
- `EqualFold` and `ToLower` agree on all three pairs. `ß` upper-cases to the two letters `SS`, which simple (rune-by-rune) case folding can't express, so neither function matches it. The Kelvin sign folds to `k`, so both do.
- `[]byte(s)` is a copy (modifying it leaves `s` alone, and it doesn't share memory); both unsafe views share memory with their source. `Println` of a `[]byte` prints byte values, not text.
- `%q` prints invalid bytes as `\x` escapes, and `%+q` also prints every non-ASCII rune as a `\u` escape. That is what makes the invalid input, the Kelvin sign and the U+FFFD replacement visible.

Benchmark shape from one run (numbers vary by machine):

```text
BenchmarkToLowerEqualMixed          ~250 ns/op    96 B/op   2 allocs/op
BenchmarkEqualFoldMixed              ~35 ns/op     0 B/op   0 allocs/op
BenchmarkToLowerEqualLower           ~60 ns/op     0 B/op   0 allocs/op
BenchmarkEqualFoldLower              ~25 ns/op     0 B/op   0 allocs/op
BenchmarkJoinSprintf                ~120 ns/op    56 B/op   3 allocs/op
BenchmarkJoinPlus                    ~60 ns/op    36 B/op   2 allocs/op
BenchmarkJoinStringsJoin             ~50 ns/op    36 B/op   2 allocs/op
BenchmarkJoinBuilder                 ~45 ns/op    36 B/op   2 allocs/op
BenchmarkBytesConversionReadOnly    ~320 ns/op     0 B/op   0 allocs/op
BenchmarkBytesConversionMutated     ~370 ns/op  1024 B/op   1 allocs/op
BenchmarkUnsafeView                 ~370 ns/op     0 B/op   0 allocs/op
```

- `EqualFold` never allocates and is several times faster. With inputs that are already lowercase ASCII, `ToLower` returns them unchanged and allocates nothing, but it still has to scan both strings.
- `Sprintf` is the slowest way to build host:port, but the others all pay for `strconv.Itoa` plus the result. The differences are tens of nanoseconds, so pick for correctness (`net.JoinHostPort`) first.
- `[]byte(payload)` passed to a function that only reads it costs nothing: the compiler proves the bytes are never written and skips the copy (`go test -gcflags=-m` reports `zero-copy string->[]byte conversion`). The copy only happens when the slice is written to or escapes. Check this before reaching for `unsafe`, which in this case buys nothing.

## Notes and corrections

- **Slide p05:** the snippet doesn't compile as shown: `declared and not used: b`. Writing through the unsafe slice of a string literal is not merely "unsafe": the bytes live in read-only memory, and the write crashes the process with `unexpected fault address` and `fatal error: fault` (an access violation on Windows, SIGSEGV on Linux and macOS). For a string built at run time, the write silently changes an "immutable" string.
- **Slide p04:** `EqualFold` uses simple Unicode case folding, one rune to one rune, so `EqualFold("ß", "SS")` is false; full case folding is in `golang.org/x/text/cases`. `ToLower` doesn't allocate when its input is already lowercase ASCII. `EqualFold` is still the faster choice.
- **Slide p06:** Go string operations don't panic on invalid UTF-8: `range` and the `utf8` functions decode it as U+FFFD. The real risk is corrupted data, for example invalid bytes stored as-is or silently replaced by `encoding/json`. Validate, or sanitise with `strings.ToValidUTF8`.
- **Slide p07:** two `s :=` in the same scope don't compile (`no new variables on left side of :=`). For host:port use `net.JoinHostPort`, which also brackets IPv6 addresses.
- **Slide p08:** `bytes.Buffer` is an append-and-read buffer with no insert or delete in the middle. For edits in the middle, use a `[]byte` with `slices.Insert` and `slices.Delete` (Go 1.21). Prefer `buf.WriteString("Initial data")` over `buf.Write([]byte("Initial data"))`.
- **Slide p03:** also worth knowing: `unique.Make` (Go 1.23) interns repeated strings, so many copies of the same value share one allocation.

## Slides

[006-advanced-string-tactics.pdf](../../slides/006-advanced-string-tactics.pdf)
