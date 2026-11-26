# Part 005: String vs. rune vs. byte

Day 03 · Week 1 · Memory efficiency.

## What this shows

- The length trap: `len` counts bytes, `utf8.RuneCountInString` counts runes, for ASCII, an accented word (`"coöts"`, which reproduces the slide's 6 and 5), Hebrew, and an emoji (`"Go 🌏"` is 7 bytes).
- Runes are not characters: the thumbs-up with a skin tone, `"👍🏽"`, is one visible symbol but two runes and eight bytes.
- Indexing a string yields bytes (slide p05), while `range` decodes runes and their byte offsets (slide p06), over the same `"Go 🌏"`.
- Invalid UTF-8 doesn't panic: `"\xff"` decodes as `U+FFFD` (`utf8.RuneError`) with width 1, through `range` and through `utf8.DecodeRuneInString`.
- Header sizes: a string header is 16 bytes and a slice header 24 on 64-bit platforms; a `rune` is 4 bytes and a `byte` is 1.
- Benchmarks for building strings (`+=` vs `strings.Builder` vs `Builder` with `Grow` vs `strings.Repeat`) and for converting with `[]byte(s)` vs working on the string directly.

## Run

```sh
go run ./examples/005-string-rune-and-byte
go test ./examples/005-string-rune-and-byte
go test -run='^$' -bench=. -benchmem ./examples/005-string-rune-and-byte
```

## What to look for

Expected output of `go run` (section 5 is for a 64-bit platform):

```text
== 1. The length trap: bytes vs runes (slide p04) ==
"Go"     len=2  utf8.RuneCountInString=2
"coöts"  len=6  utf8.RuneCountInString=5
"שלום"   len=8  utf8.RuneCountInString=4
"Go 🌏"   len=7  utf8.RuneCountInString=4

== 2. Runes are not characters (slide p06) ==
"👍🏽": len=8 runes=2
  U+1F44D
  U+1F3FD

== 3. Byte loop vs range loop (slides p05-p06) ==
for i := 0; i < len(s); i++ (bytes):
  s[0] = 'G'      0x47
  s[1] = 'o'      0x6f
  s[2] = ' '      0x20
  s[3] = 'ð'      0xf0
  s[4] = '\u009f' 0x9f
  s[5] = '\u008c' 0x8c
  s[6] = '\u008f' 0x8f
for index, runeVal := range s (runes):
  G starts at byte 0
  o starts at byte 1
    starts at byte 2
  🌏 starts at byte 3

== 4. Invalid UTF-8 decodes as U+FFFD (slide p06) ==
range: offset 0 -> U+0061
range: offset 1 -> U+FFFD
range: offset 2 -> U+0062
DecodeRuneInString("\xff") = U+FFFD, width 1, RuneError? true

== 5. Header sizes (slides p02-p03) ==
GOARCH: amd64
unsafe.Sizeof("")          = 16
unsafe.Sizeof([]byte(nil)) = 24
unsafe.Sizeof(rune)        = 4
unsafe.Sizeof(byte)        = 1
```

- `"coöts"` is 6 bytes but 5 runes because `ö` takes 2 bytes in UTF-8. Hebrew letters take 2 bytes each, the globe emoji 4.
- `"👍🏽"` is `U+1F44D` (thumbs up) followed by `U+1F3FD` (a skin-tone modifier). Counting what a reader sees as one character (a grapheme cluster) needs a segmentation library, not `utf8`.
- The byte loop turns the emoji's four bytes into four unrelated values: `0xf0` happens to be `ð` in Latin-1, and the rest are control codes. The range loop prints the emoji once, at byte offset 3.
- The invalid byte in `"a\xffb"` still occupies offset 1, and decoding moves on to `b` at offset 2: Go replaces what it can't decode instead of failing.

Benchmark shape from one run (numbers vary by machine; the allocation columns are the point):

```text
BenchmarkConcatPlus              ~10,700 B/op   99 allocs/op
BenchmarkBuilder                    ~500 B/op    6 allocs/op
BenchmarkBuilderGrow                 208 B/op    1 allocs/op
BenchmarkRepeat                      208 B/op    1 allocs/op
BenchmarkCountViaBytes             1,152 B/op    1 allocs/op
BenchmarkCountOnString                 0 B/op    0 allocs/op
BenchmarkRangeBytesConversion          0 B/op    0 allocs/op
```

- `+=` allocates a new string for nearly every append and copies everything built so far, so it is also roughly 10x slower than a `Builder` here. `Grow` cuts the `Builder` to a single allocation, the same as `strings.Repeat`.
- `bytes.Count([]byte(text), ...)` pays one 1 KB copy per call; `strings.Count(text, ...)` pays nothing.
- `for _, c := range []byte(text)` allocates nothing: the compiler recognises the pattern and iterates the string's bytes in place.

## Notes and corrections

- **Slide p03:** the types are lowercase `rune` and `byte`. `var r Rune = 'a'` fails with `undefined: Rune` (and likewise `undefined: Byte`).
- **Slide p02:** a string header is a pointer and a length, 16 bytes on 64-bit platforms. Unlike a slice it has no capacity, which is why a slice header is 24 bytes. The public way to reach the pointer is `unsafe.StringData`.
- **Slide p06:** `range` is idiomatic but not the only safe way: `utf8.DecodeRuneInString` (and `[]rune(s)`, at the cost of a copy) decode correctly too. A rune is a code point, not a user-perceived character: flags, skin tones and combining accents are several runes.
- **Slide p07:** `[]byte(s)` usually copies, but the compiler optimises some conversions away: `range []byte(s)` (see the benchmark), map lookups `m[string(b)]`, comparisons such as `string(b) == "x"`, and small results that don't escape, which use a 32-byte stack buffer.
- **Slide p08:** if you know the final size, `b.Grow(n)` pre-sizes a `strings.Builder` so it allocates once. A `strings.Builder` must not be copied after first use; writing to a copy panics.

## Slides

[005-string-rune-and-byte.pdf](../../slides/005-string-rune-and-byte.pdf)
