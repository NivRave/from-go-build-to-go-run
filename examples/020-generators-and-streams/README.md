# Part 020: Generators & Streams

Day 10 · Week 2 · Processing "infinite" data with constant memory.

## What this shows

- The closure-based generator and pipeline from the slides (`IntGenerator`, `FilterEven`, `Multiply`), with the elided stage bodies filled in.
- The same pipeline written with Go 1.23 range-over-func iterators (`iter.Seq`). This is the idiomatic form today.
- That iterators are lazy: `break` in the consumer stops the producer, so a range of one billion costs only what you read.
- Why a stream API needs `Err()`: a line longer than 64 KiB silently ends a `bufio.Scanner` loop unless you check `Err()`, and `Scanner.Buffer` fixes it.
- Memory cost of collect-all versus streaming, measured with benchmarks.

## Run

```sh
go run ./examples/020-generators-and-streams
go test ./examples/020-generators-and-streams
go test -run='^$' -bench=. -benchmem ./examples/020-generators-and-streams
```

## What to look for

Expected output of `go run`:

```text
== 1. Closure pipeline (slides p03-p06) ==
20 40 60 80 100

== 2. The same pipeline with iter.Seq (Go 1.23+) ==
20 40 60 80 100

== 3. Laziness: break stops the producer ==
consumed up to 3, producer generated 3 values

== 4. Streams must report errors (slides p05, p07) ==
default buffer: 1 lines, err = bufio.Scanner: token too long
1 MiB buffer:   3 lines, err = <nil>
```

Benchmark shape (numbers vary by machine):

```text
BenchmarkCollectAll         ~4,100,000 B/op   ~28 allocs/op
BenchmarkClosureGenerator            0 B/op     0 allocs/op
BenchmarkIterSeq                     0 B/op     0 allocs/op
```

- `CollectAll` allocates proportionally to `n`; the streaming versions allocate nothing, whatever `n` is.
- The closure generator shows **0 allocs** even though slide p03 says `current` escapes. The compiler inlined `IntGenerator`, so the closure never left the caller's frame. Run `go build -gcflags=-m ./examples/020-generators-and-streams` and you'll see both `moved to heap: current` (for the standalone function) and `inlining call to IntGenerator` (at the call sites, where it stays on the stack).

## Notes and corrections

- **Slide p03/p09 "state lives on the stack":** captured, mutated variables of a closure that outlives its function are moved to the heap unless inlining prevents the escape. What a generator guarantees is O(1) memory, not stack residency.
- **Slide p03 "thread-safe":** creating separate generators is safe; *sharing one* generator between goroutines is a data race on `current`.
- **Slide p05:** the `Scanner` interface needs an `Err() error` method, like `bufio.Scanner` and `sql.Rows`.
- **Slide p07:** check `scanner.Err()` after the loop, and use `scanner.Bytes()` instead of `Text()` if you need zero per-line allocation.
- **Slide p08 "Generators need custom function signatures":** no longer true since Go 1.23. `iter.Seq[T]` is the standard signature, and `slices.Values`, `maps.Keys` and `strings.Lines` all return iterators.

## Slides

[020-generators-and-streams.pdf](../../slides/020-generators-and-streams.pdf)
