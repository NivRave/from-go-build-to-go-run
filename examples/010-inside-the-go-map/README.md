# Part 010: Inside the Go map

Day 05 · Week 1 · Buckets, overflows, and the chaos of random iteration.

## What this shows

- A map value is one pointer: `unsafe.Sizeof` is 8 for a nil map and for a map made with a hint of 100.
- Iteration order is randomised on every `range`, and `slices.Sorted(maps.Keys(m))` is the way to get a deterministic order.
- Maps never shrink: 1M `int→int` entries hold tens of MB after `delete`-ing everything and after `clear`, until the map itself is dropped.
- The measured memory per entry of `map[string]bool`, `map[string]struct{}` and `map[int64]struct{}` sets.
- Benchmarks: insert growth with and without a hint, lookups with 8-byte vs 1 KB keys, and GC time for `map[int]User` vs `map[int]*User`.

## Run

```sh
go run ./examples/010-inside-the-go-map
go test ./examples/010-inside-the-go-map
go test -run='^$' -bench=. -benchmem ./examples/010-inside-the-go-map
GODEBUG=gctrace=1 go run ./examples/010-inside-the-go-map   # optional: one line per GC
```

PowerShell form of the last command: `$env:GODEBUG="gctrace=1"; go run ./examples/010-inside-the-go-map`.

## What to look for

Output of one `go run` (the iteration orders in section 2 and the MB figures in sections 3 and 4 change from run to run and machine to machine):

```text
== 1. A map value is one pointer (slide p02) ==
nil map:        Sizeof = 8 bytes, len = 0
map, hint 100:  Sizeof = 8 bytes, len = 0

== 2. Random iteration order (slide p03) ==
3 keys (one group):
  run 1: Alpha Beta Gamma
  run 2: Gamma Alpha Beta
  run 3: Alpha Beta Gamma
  run 4: Gamma Alpha Beta
  run 5: Alpha Beta Gamma
10 keys (a table):
  run 1: Zeta Kappa Gamma Delta Epsilon Eta Theta Iota Alpha Beta
  run 2: Eta Theta Iota Alpha Beta Zeta Kappa Gamma Delta Epsilon
  run 3: Eta Theta Iota Alpha Beta Zeta Kappa Gamma Delta Epsilon
  run 4: Gamma Delta Epsilon Eta Theta Iota Alpha Beta Zeta Kappa
  run 5: Kappa Gamma Delta Epsilon Eta Theta Iota Alpha Beta Zeta
  sorted: Alpha Beta Delta Epsilon Eta Gamma Iota Kappa Theta Zeta

== 3. Maps grow but never shrink (slides p05, p09) ==
after inserting 1M int->int   len = 1000000  HeapInuse =  44.6 MB
after deleting all + GC       len =       0  HeapInuse =  44.7 MB
after clear(m) + GC           len =       0  HeapInuse =  44.6 MB
after m = nil + GC            len =       0  HeapInuse =   0.6 MB

== 4. Memory per entry of a set (slide p08) ==
unsafe.Sizeof: bool = 1 byte, struct{} = 0 bytes
map[string]bool       53.3 MB total, 55.8 bytes per entry
map[string]struct{}   53.2 MB total, 55.8 bytes per entry
map[int64]struct{}    36.0 MB total, 37.7 bytes per entry
```

- The slide's 3-key map often repeats the same order. A map of up to 8 entries is one group of 8 slots filled in insertion order; each `range` starts at a random slot and wraps around, so with 3 entries most starting points give the same order, and the others are rotations of it. In the 10-key map the orders are rotations too (it is one small table), but the starting point varies more. Larger maps add a random starting table as well. Never rely on any of these orders.
- 1M `int→int` entries take about 44 MB, not 1 GB. The figure does not move after `delete` or `clear`, only after `m = nil`.
- A `struct{}` value saves nothing next to a `string` key: the slot is padded to the same size as with a `bool`. Most of the per-entry cost is the key, one control byte per slot, and the empty slots that a 7/8 maximum load factor leaves. `struct{}` is still the idiomatic set value because it says "no value here".

Benchmark shape (numbers vary by machine):

```text
BenchmarkInsertNoHint        ~530 µs/op   ~590,000 B/op   79 allocs/op
BenchmarkInsertHint          ~190 µs/op   ~295,000 B/op   33 allocs/op
BenchmarkLookupKey8B         ~540 ns/op   (64 lookups)
BenchmarkLookupKey1KB       ~2600 ns/op   (64 lookups)
BenchmarkGCMapOfValues       ~2.2 ms/op   ~17,000 pause-ns/gc
BenchmarkGCMapOfPointers     ~5.1 ms/op    ~9,000 pause-ns/gc
```

- A hint for 10,000 entries makes inserting about 3x faster with half the bytes allocated.
- Lookups hash and compare the whole key, so 1 KB keys cost several times more than 8-byte keys.
- A forced GC takes more than twice as long with a million `*User` values, because the GC has to visit each pointed-to object. The stop-the-world pause per GC (`pause-ns/gc`) stays in the microseconds for both: the extra cost is concurrent mark work, not a longer pause.

## How Go maps work since Go 1.24

The slides describe the original hash map. Go 1.24 replaced it with a Swiss table (`internal/runtime/maps`). None of this is visible through the public API, only through memory use and benchmarks like the ones above.

- **Groups:** slots are grouped 8 at a time, with an 8-byte control word holding one control byte per slot.
- **Split hash:** the hash is split into H1 (57 bits, used to choose a table and probe for a group) and H2 (7 bits, stored in the slot's control byte).
- **Parallel match:** a lookup compares all 8 control bytes of a group against H2 at once (SIMD on amd64), and compares full keys only for matching slots.
- **Growth:** the maximum load factor is 7/8. A table holds at most 1024 slots; a full table is rehashed into one twice its size, and a table at the limit splits in two under a directory that doubles as needed.
- **No overflow buckets:** collisions probe into other groups instead, there is no incremental evacuation, and deletes leave tombstones.
- **Small maps:** a map with 8 or fewer entries is a single group with no table or directory.

## Notes and corrections

- **Slides p04-p06:** these pages describe the pre-1.24 implementation (`hmap`, 8-slot buckets, overflow buckets, `tophash`, load factor 6.5, incremental evacuation). Go 1.24 replaced it with Swiss tables, summarised above.
- **Slide p02:** a map value is still 8 bytes on 64-bit, now a pointer to `maps.Map` rather than `hmap`. The hash seed is per map, so even two maps in the same process place the same keys differently.
- **Slide p03:** iteration order is still randomised. Since Go 1.23 the idiomatic ordered loop is `for _, k := range slices.Sorted(maps.Keys(m))`.
- **Slide p05:** 1M `int→int` entries take tens of MB (about 44 MB in this run), not 1 GB.
- **Slide p07:** the GC marks concurrently, so more pointers mean more mark work rather than longer stop-the-world pauses. Keys and values larger than 128 bytes are stored indirectly by the map anyway, so a "use pointers above 256 bytes" rule has little effect.
- **Slide p08:** the overhead figures ("1M keys = 1MB" for `bool`, "0B" for `struct{}`) do not match either implementation. Section 4 measures the real per-entry cost.
- **Slide p09:** maps still never shrink in Go 1.26. Copying the live entries into a new map is the usual way to compact one.

## Slides

[010-inside-the-go-map.pdf](../../slides/010-inside-the-go-map.pdf)
