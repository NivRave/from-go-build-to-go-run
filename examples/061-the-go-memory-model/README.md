# Part 061: The Go memory model & struct alignment

Day 31 · Week 5 · Optimizing Go at the hardware level.

## What this shows

- `unsafe.Sizeof`, `Alignof` and `Offsetof` for the slides' `BadStruct`, `GoodStruct`, `LastField` and `FirstField`, plus `runtime.GOARCH`, and what changes on 32-bit.
- That fields align to their **alignment**, not their size: `[3]int64`, `string` and `[]byte` are 24, 16 and 24 bytes but all 8-aligned.
- The Go memory model, which the slides' title promises but they don't cover: correct publication of data from one goroutine to another through a channel, a `sync.Mutex` and an `atomic.Bool`. `-race-demo` runs the unsynchronised version.
- Benchmarks: summing a field over one million `BadStruct`s versus `GoodStruct`s, and false sharing between adjacent `atomic.Int64` counters versus counters padded to 128 bytes.

## Run

```sh
go run ./examples/061-the-go-memory-model
go test ./examples/061-the-go-memory-model
go test -run='^$' -bench=. -benchmem ./examples/061-the-go-memory-model
GOARCH=386 go run ./examples/061-the-go-memory-model
go run -race ./examples/061-the-go-memory-model -race-demo
go test -race -run=Publication ./examples/061-the-go-memory-model
```

PowerShell form of the 32-bit run: `$env:GOARCH="386"; go run ./examples/061-the-go-memory-model` (then `Remove-Item Env:GOARCH`). A 386 binary runs natively on 64-bit Windows and Linux. The race detector needs cgo, so on Windows it needs a C compiler; CI runs `go test -race` on Linux.

Optional, outside the module (it downloads `golang.org/x/tools`): `go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest ./examples/061-the-go-memory-model` reports structs whose fields could be reordered to save space.

## What to look for

Expected output of `go run` on amd64 or arm64:

```text
== 1. Platform ==
GOARCH=amd64, pointer size 8 bytes

== 2. Struct padding and field order (slides p03, p05, p07) ==
BadStruct  size 24  align 8
  A  offset  0  size 1  align 1
  B  offset  8  size 8  align 8
  C  offset 16  size 4  align 4
GoodStruct size 16  align 8
  B  offset  0  size 8  align 8
  C  offset  8  size 4  align 4
  A  offset 12  size 1  align 1
LastField  size 16  align 8
  A  offset  0  size 8  align 8
  B  offset  8  size 0  align 1
FirstField size  8  align 8
  B  offset  0  size 0  align 1
  A  offset  0  size 8  align 8

== 3. Alignment is not size (slides p02, p04) ==
[3]int64  size 24  align 8
string    size 16  align 8
[]byte    size 24  align 8

== 4. The memory model: publishing data safely (not on the slides) ==
channel: 42
mutex:   42
atomic:  42
```

With `GOARCH=386`, `float64` and `int64` align to 4, so the padding before `B` shrinks to 3 bytes and the struct to 16, the same as `GoodStruct`. `LastField` becomes 12 bytes:

```text
GOARCH=386, pointer size 4 bytes
BadStruct  size 16  align 4
  A  offset  0  size 1  align 1
  B  offset  4  size 8  align 4
  C  offset 12  size 4  align 4
LastField  size 12  align 4
```

Benchmark shape (AMD Ryzen 7 5800X, three runs; numbers vary by machine and with load):

```text
BenchmarkSumBadStruct       ~630,000 ns/op   24.00 slice-MB
BenchmarkSumGoodStruct      ~470,000-530,000 ns/op   16.00 slice-MB
BenchmarkCountersAdjacent   ~740,000 ns/op
BenchmarkCountersPadded      ~45,000 ns/op
```

- Reordering the fields saves a third of the memory and, here, roughly 15-25% of the time for a sequential scan. On this CPU both slices fit in the 32 MB L3 cache, so the gap is smaller than the size ratio; in a loaded run it occasionally disappeared. Measure on your target hardware before reordering structs for speed (slide p08).
- False sharing is the much bigger effect: eight goroutines, each touching only its own counter, are about **15x slower** when the counters share cache lines. Padding each counter to 128 bytes removes the contention without changing the logic.
- `Example_memoryModel` and `TestPublication` run the three correct patterns; under `go test -race` they report no race. `-race-demo` does report one (a `WARNING: DATA RACE` between the writer goroutine and the reader), and without `-race` it may print 0 or 42.

## The Go memory model in brief

- **Happens-before.** A goroutine is guaranteed to see another goroutine's write only if the write *happens before* the read. Inside one goroutine, program order provides that. Between goroutines, only synchronisation does: a channel send happens before the corresponding receive completes; an `Unlock` happens before the next `Lock` returns; an atomic store happens before an atomic load that observes it.
- **DRF-SC.** A program with no data races behaves as if its goroutines' operations were interleaved in one sequential order (sequential consistency for data-race-free programs). Make the program race-free and you can reason about it step by step; `go test -race` finds the races.
- **The synchronisation primitives** are channels, `sync.Mutex`/`RWMutex`, `sync/atomic`, `sync.Once`, `sync.WaitGroup` and `sync.Cond`. A plain `bool` flag is not one of them, which is why `-race-demo` is broken even though it usually prints 42.
- **Go 1.19 revised the memory model** to match C, C++, Java and Rust: it formally defines `sync/atomic` operations as sequentially consistent, and it added the typed atomics (`atomic.Bool`, `atomic.Int64`, `atomic.Pointer[T]`, ...) used here. See [go.dev/ref/mem](https://go.dev/ref/mem).

## Notes and corrections

- **Title:** the slides cover memory **layout** (padding, alignment, field order), not the memory **model** (what one goroutine is guaranteed to see of another's writes). This example adds the latter in section 4 and in the section above.
- **Slide p05:** the subtitle promises false sharing but the slide doesn't explain it. Two cores writing different variables on the same cache line force the line to bounce between them; `sharing_test.go` measures it and pads counters apart.
- **Slides p02 and p04:** a field's address is a multiple of its **alignment** (`unsafe.Alignof`), not its size. `[3]int64` is 24 bytes but 8-aligned, and a struct aligns to its most-aligned field. So sort fields by alignment, not by size.
- **Platform:** the slides' numbers (24, 16, 16 and 8) are for 64-bit targets. On 32-bit, `int64` and `float64` align to 4, which is also why 64-bit atomic operations on plain `int64` fields could crash on 32-bit platforms; `atomic.Int64` (Go 1.19) guarantees 8-byte alignment everywhere.
- **Slide p06:** cache lines are 64 bytes on most x86 and Arm servers but 128 bytes on Apple M-series, which is why the padded counter here is 128 bytes.
- **Slide p04:** the `fieldalignment` analyzer (`golang.org/x/tools/go/analysis/passes/fieldalignment`) automates the reordering advice, and also reports how many pointer bytes the GC has to scan.

## Slides

[061-the-go-memory-model.pdf](../../slides/061-the-go-memory-model.pdf)
