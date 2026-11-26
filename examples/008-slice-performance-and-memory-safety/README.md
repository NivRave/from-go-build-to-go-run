# Part 008: Slice performance & memory safety

Day 04 · Week 1 · From destructive filtering to protecting your garbage collector.

## What this shows

- The slide's in-place `FilterInPlace` (fixed so it compiles) next to `slices.DeleteFunc`: both return the right answer and both overwrite their input. Cloning first with `slices.Clone` keeps the original intact.
- The append "ghost": appending to `parent[:2]` overwrites `parent[2]`, so `parent` becomes `[1 2 99 4]`. The full slice expression `parent[:2:2]` caps the capacity and forces append to reallocate.
- The slide p05 three-index demo with `fmt` and the comments corrected, printing `len`/`cap` and reproducing the slide's output exactly.
- The backing-array leak, measured: a 10-byte slice of a 64 MB array keeps all 64 MB alive after a full GC; `bytes.Clone` of the same 10 bytes lets it go.
- nil vs empty slices in JSON: `null` vs `[]`, on their own and as a struct field.
- Benchmarks: in-place vs allocating filter, and the cost of a full GC with 1M `[]Item` (values with a string field) vs `[]*Item` vs a pointer-free `[]Point`.

## Run

```sh
go run ./examples/008-slice-performance-and-memory-safety
go test ./examples/008-slice-performance-and-memory-safety
go test -run='^$' -bench=. -benchmem ./examples/008-slice-performance-and-memory-safety
```

## What to look for

Expected output of `go run` (the heap figures in section 4 can differ by a few MB between machines and runs):

```text
== 1. In-place filtering clobbers its input (slides p02-p03) ==
FilterInPlace:       result [2 4 6]  input now [2 4 6 4 5 6]
slices.DeleteFunc:   result [2 4 6]  input now [2 4 6 0 0 0]
Clone, then filter:  result [2 4 6]  input now [1 2 3 4 5 6]

== 2. The append ghost bug (slide p04) ==
child := parent[:2]:   parent [1 2 99 4]  child [1 2 99] (the 3 is gone)
child := parent[:2:2]: parent [1 2 3 4]  child [1 2 99]

== 3. The defensive fix: 3-index slicing (slide p05) ==
s1: len 3 cap 5; s2: len 3 cap 3
s1 after append to s1: [1 2 3 10 11]
s2 after append to s1: [1 2 3]
array after s1 append: [0 1 2 3 10 11]
s1 after append to s2: [1 2 3 10 11]
s2 after append to s2: [1 2 3 20]
array after s2 append: [0 1 2 3 10 11]

== 4. The massive backing-array leak (slide p06) ==
baseline:                HeapInuse =   0 MB, len(kept) = 0
kept hugeData[:10]:      HeapInuse =  64 MB, len(kept) = 10
kept bytes.Clone(...):   HeapInuse =   0 MB, len(kept) = 10

== 5. nil vs empty: the API contract (slide p07) ==
[]int(nil)                         -> null
[]int{}                            -> []
main.Response{Items:[]string(nil)} -> {"items":null}
main.Response{Items:[]string{}}    -> {"items":[]}
```

- `FilterInPlace` leaves `[2 4 6 4 5 6]` behind in its input: the first three elements were overwritten, the rest are stale. `slices.DeleteFunc` zeroes the stale tail instead (since Go 1.22), which matters when elements hold pointers.
- With `parent[:2]` the child has capacity 4, so `append` writes 99 into the parent's third element. With `parent[:2:2]` the capacity is 2, so `append` copies to a new array and the parent keeps its 3.
- In section 3, `s2`'s capacity is 3 (`max - low` = 4 - 1), so appending to it allocates and leaves `array` alone, while `s1` (capacity 5) writes 10 and 11 into `array`.
- Section 4 calls `runtime.GC()` before each reading. The only reference to the 64 MB array is the 10-byte `kept`, and that is enough to keep it all in use.

Benchmark shape from one run (numbers vary by machine):

```text
BenchmarkFilterInPlace          ~5,300 ns/op         0 B/op   0 allocs/op
BenchmarkFilterAlloc           ~13,600 ns/op    81,920 B/op   1 allocs/op
BenchmarkGCSliceOfValues        ~1.9 ms per GC   (1M Item values, each with a string)
BenchmarkGCSliceOfPointers      ~4.6 ms per GC   (1M *Item, 1M separate objects)
BenchmarkGCSliceNoPointers      ~0.14 ms per GC  (1M Point, no pointers at all)
```

- The in-place filter allocates nothing, even though it also pays to refill its buffer every iteration.
- A full GC with 1M `*Item` live takes a few times longer than with 1M `Item` values, and a pointer-free `[]Point` costs next to nothing, because its backing array is never scanned. The cost is mark work (CPU time, mostly done concurrently with your program), not a longer stop-the-world pause.

## Notes and corrections

- **Slide p02:** two statements on one line don't compile without a separator: `n := 0 for ...` fails with `syntax error: unexpected keyword for at end of statement`. For element types that contain pointers, also `clear(s[n:])` so the stale tail doesn't keep garbage reachable; `slices.DeleteFunc` does this since Go 1.22.
- **Slide p03:** `copy` doesn't make anything thread-safe. It is itself a read of `original`, so it races with any goroutine writing `original` at the same time. It is safe only if nobody mutates the original concurrently (or the copy is synchronised). `slices.Clone(original)` is the one-line form.
- **Slide p04:** Go prints the result as `[1 2 99 4]`, without commas.
- **Slide p05:** `mt.Printf` should be `fmt.Printf` (`undefined: mt`), and the comment on the `s2` line describes `s2`, not `s1`. The printed output on the slide is correct.
- **Slide p06:** `bytes.Clone(hugeData[:10])` (Go 1.20) or `slices.Clone` (Go 1.21) replace `append([]byte{}, hugeData[:10]...)`. Note that `bytes.Clone(nil)` returns nil.
- **Slide p08:**
  - The GC marks concurrently with your program. Pointers add mark work, not "stop the world" time; stop-the-world pauses are short phase transitions that don't grow with the number of pointers.
  - The real win is a pointer-free element type: its backing array is allocated "noscan" and the GC never scans it. A `[]MyStruct` whose struct has pointer fields (strings, slices, maps) is still scanned field by field.
  - The 256-byte threshold for switching to pointers is a rule of thumb, not a Go rule. Measure.
  - Go 1.26's default Green Tea GC scans small objects span by span for better memory locality. That changes the constants, but not the principle.

## Slides

[008-slice-performance-and-memory-safety.pdf](../../slides/008-slice-performance-and-memory-safety.pdf)
