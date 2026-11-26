# Part 009: Maps

Day 05 · Week 1 · Beyond the key-value pair.

## What this shows

- The comma-ok idiom: a key holding `0` and a missing key look the same to a plain lookup; `v, ok := m[k]` tells them apart.
- Maps passed to a function share the caller's data, but reassigning the parameter (`m = make(...)`) inside the function does not affect the caller.
- A `SafeMap` that compiles and works (field `mu sync.RWMutex`, `NewSafeMap()` constructor), hammered by 100 goroutines started with `wg.Go`, with the same final count every run.
- Key rules: struct and array keys work; an `any` key holding a slice panics at run time; `NaN` keys pile up as unreachable entries until `clear(m)`.
- Retention: deleting every entry, or `clear(m)`, leaves the map's memory allocated; only dropping the map lets the GC free it.
- `-crash`: unsynchronised concurrent writes end in `fatal error: concurrent map writes`, which `recover` cannot catch.

## Run

```sh
go run ./examples/009-maps
go test ./examples/009-maps
go test -run='^$' -bench=. -benchmem ./examples/009-maps
go run ./examples/009-maps -crash        # ends in a fatal error on purpose
go run -race ./examples/009-maps -crash  # the race detector reports it reliably (needs cgo)
```

## What to look for

Expected output of `go run` (the `HeapInuse` figures in section 5 vary by machine and Go version):

```text
== 1. "Comma ok": zero vs missing (slide p04) ==
m["zero"]    = 0
m["missing"] = 0
zero    -> present, value 0
missing -> not present

== 2. Maps are shared with the functions you pass them to (slide p05) ==
after update:  map[done:1 start:0]
inside replace, before: len = 2
inside replace, after:  map[replaced:1]
after replace: map[done:1 start:0]

== 3. A correct SafeMap (slide p06) ==
keys: 101
hits: 10000

== 4. Key rules (slide p07) ==
struct key: treasure
array key:  listUsers
any key:    recovered: runtime error: hash of unhashable type []int
NaN keys:   len = 3, lookup found = false
after clear: len = 0

== 5. Deleting does not shrink a map (slide p08) ==
after inserting 1M entries   len = 1000000  HeapInuse =  41.6 MB
after deleting all + GC      len =       0  HeapInuse =  41.4 MB
after clear(m) + GC          len =       0  HeapInuse =  41.4 MB
after m = nil + GC           len =       0  HeapInuse =   0.7 MB
```

- `replace` built a new map, but `after replace` still shows the caller's original map: the function only changed its own copy of the pointer.
- `hits: 10000` is exact because `SafeMap.Add` does the read-modify-write under a single lock. A `Get` followed by a `Set` would lose updates.
- In section 5 the heap stays at about 40 MB after `delete` and after `clear`; it drops only after `m = nil`. With 1M `int→int` entries the cost is tens of MB.
- `go run ./examples/009-maps -crash` starts like this, and the process exits with a non-zero status:

```text
fatal error: concurrent map writes

goroutine 23 [running]:
internal/runtime/maps.fatal({0x7ff7159033ae?, 0x0?})
```

Benchmark shape (numbers vary by machine):

```text
BenchmarkInsertNoHint     ~85 µs/op   ~109,000 B/op   20 allocs/op
BenchmarkInsertHint       ~23 µs/op    ~55,000 B/op    5 allocs/op
BenchmarkLookup8Slice     ~44 ns/op          0 B/op    0 allocs/op
BenchmarkLookup8Map       ~61 ns/op          0 B/op    0 allocs/op
```

- Pre-sizing 1000 entries is several times faster and allocates half as much, because the map never has to rehash into bigger tables.
- For eight short keys, a linear `slices.Contains` beats hashing (slide p02's constant-factor point).

## Notes and corrections

- **Slide p06:** the `SafeMap` does not compile: the receiver and the mutex field are both named `s`, so `s.RLock()` is undefined. It also panics on `Set` without a constructor, because the zero value's map is nil. This example names the field `mu` and adds `NewSafeMap()`.
- **Slide p06:** the crash snippet is unreliable: `main` usually returns before either goroutine runs, and the runtime's detection is best-effort. Wait for the goroutines (as `-crash` does) and use `go run -race` to catch the race every time.
- **Slide p04:** `val, ok := m["unknown_key"] if !ok {` on one line is a syntax error. Write `if _, ok := m[k]; !ok { ... }`.
- **Slide p03:** "buckets" is pre-1.24 terminology. Since Go 1.24 maps are Swiss tables (groups, tables and a directory). Growth also happens synchronously during the insert that triggers it, not in the background. The pre-sizing advice still holds.
- **Slide p07:** arrays and interfaces are also valid key types, but an interface key panics at run time if its dynamic type is not comparable. `NaN` keys never match, so each insert adds a new entry that only `clear(m)` (Go 1.21) removes.
- **Slide p08:** `clear(m)` empties a map but keeps its capacity, just like deleting every key. To release the memory, drop all references (`m = nil` or `m = make(...)`).

## Slides

[009-maps.pdf](../../slides/009-maps.pdf)
