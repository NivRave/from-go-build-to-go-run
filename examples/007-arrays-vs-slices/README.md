# Part 007: Arrays vs. slices

Day 04 · Week 1 · Why we (almost) never use arrays.

## What this shows

- Arrays are values: assigning one or passing it to a function copies every element, so changes to the copy never reach the original. A slice passed to a function shares its backing array. `unsafe.Sizeof([1000]int{})` is 8000 bytes; a slice header is 24.
- Slices share backing arrays: `sub := original[1:3]` is a window, so writing through it changes `original`. `slices.Clone` makes an independent copy.
- A growth trace: append 0..2000 to a nil slice and print every reallocation. Capacity doubles up to 256 elements and then grows more gently.
- nil vs empty vs `make`: `== nil`, `len`, `cap` and the `encoding/json` output (`null` vs `[]`) of each, plus a struct showing what the `omitzero` tag option (Go 1.24) drops.
- Benchmarks: `append` with and without `make([]T, 0, n)`, and passing a 64 KiB array by value vs as a slice.

## Run

```sh
go run ./examples/007-arrays-vs-slices
go test ./examples/007-arrays-vs-slices
go test -run='^$' -bench=. -benchmem ./examples/007-arrays-vs-slices
go build -gcflags=-m ./examples/007-arrays-vs-slices   # see what escapes to the heap
```

## What to look for

Expected output of `go run` on a 64-bit platform with Go 1.26 (section 3's capacities can change between Go versions):

```text
== 1. Arrays are values (slides p02-p03) ==
after b := a; b[0] = 99:  a = [1 2 3]  b = [99 2 3]
after mutateArray(a):     a = [1 2 3]
after mutateSlice(s):     s = [100 2 3]
unsafe.Sizeof([1000]int{}) = 8000 bytes (copied on every assignment)
unsafe.Sizeof([]int{})     = 24 bytes (pointer, len, cap)

== 2. Slices share a backing array (slide p06) ==
sub[0] = 99:          original = [1 99 3 4]  sub = [99 3]
after slices.Clone:   original = [1 2 3 4]  independent = [99 3]

== 3. Growth trace: append to a nil slice (slide p07) ==
len    1: cap    0 ->    4 (first allocation)
len    5: cap    4 ->    8 (x2.00)
len    9: cap    8 ->   16 (x2.00)
len   17: cap   16 ->   32 (x2.00)
len   33: cap   32 ->   64 (x2.00)
len   65: cap   64 ->  128 (x2.00)
len  129: cap  128 ->  256 (x2.00)
len  257: cap  256 ->  512 (x2.00)
len  513: cap  512 ->  848 (x1.66)
len  849: cap  848 -> 1280 (x1.51)
len 1281: cap 1280 -> 1792 (x1.40)
len 1793: cap 1792 -> 2560 (x1.43)

== 4. nil vs empty vs make (slides p04, p05, p08) ==
var s []int         == nil: true  len 0 cap  0  []          JSON null
[]int{}             == nil: false len 0 cap  0  []          JSON []
make([]int, 0, 5)   == nil: false len 0 cap  5  []          JSON []
make([]int, 5, 10)  == nil: false len 5 cap 10  [0 0 0 0 0] JSON [0,0,0,0,0]
Payload: {"nil":null,"empty":[],"omitzero_empty":[]}
```

- `a` is unchanged after both `b[0] = 99` and `mutateArray(a)`: each received its own copy. `mutateSlice(s)` changed the caller's data, because only the 24-byte header was copied.
- After `sub[0] = 99`, `original[1]` is 99 too. After `slices.Clone`, the two are independent.
- The growth trace doubles exactly up to 256 elements and then tapers (x1.66, x1.51, x1.40 here) toward 1.25x. The odd numbers such as 848 come from rounding up to the allocator's size classes. The first append already gets room for 4 ints: `go build -gcflags=-m` reports `append does not escape` for this loop, and the compiler starts such a slice in a small stack buffer.
- `[]int{}` and `make([]int, 0, 5)` marshal as `[]`, a nil slice as `null`. `omitzero` omits only the nil slice, because the zero value of a slice is nil; an empty slice is kept.

Benchmark shape from one run (numbers vary by machine):

```text
BenchmarkAppendGrow          ~36,000 ns/op   ~357,000 B/op   19 allocs/op
BenchmarkAppendPrealloc      ~15,000 ns/op     81,920 B/op    1 allocs/op
BenchmarkPassArrayByValue       ~960 ns/op          0 B/op    0 allocs/op
BenchmarkPassSlice                ~1.5 ns/op        0 B/op    0 allocs/op
```

- Pre-allocating with `make([]int, 0, n)` turns 19 allocations (and about 4x the bytes, all copied along the way) into one, and roughly halves the time.
- Passing the 64 KiB array by value copies all of it on every call, which is hundreds of times slower than passing a slice header. Nothing is heap-allocated in either case; the cost is pure copying.
- `TestEmptySliceDoesNotAllocate` in `bench_test.go` checks that creating `[]int{}` costs zero heap allocations even when it escapes.

## Notes and corrections

- **Slides p02 and p08:** escape analysis and size decide where memory lives, not array vs slice. An array that escapes, or is too large for the stack, goes on the heap, and the backing array of a small slice that doesn't escape can live on the stack. Check with `go build -gcflags=-m`.
- **Slide p08:** `[]int{}` doesn't heap-allocate. It is non-nil, but its zero-length backing array points at a shared zero-size base in the runtime, so it costs nothing. (The p08 snippet also declares `s` three times in one scope, so it doesn't compile when pasted as one block.)
- **Slide p07:** growth doubles up to 256 elements, then tapers smoothly toward 1.25x and is rounded up to allocator size classes (since Go 1.18). Exact capacities are an implementation detail, not something to rely on.
- **Slide p08:** the JSON rule is for `encoding/json`. Under the `encoding/json/v2` experiment (Go 1.25, `GOEXPERIMENT=jsonv2`), `json/v2` marshals a nil slice as `[]`, while `encoding/json` keeps `null`. The `omitzero` tag option (Go 1.24) is another way to control what an API returns.

## Slides

[007-arrays-vs-slices.pdf](../../slides/007-arrays-vs-slices.pdf)
