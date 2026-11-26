# Part 016: Escape analysis

Day 08 · Week 2 · The compiler's secret logic.

## What this shows

Part 015 is about *where* data lives; this part is about *how the compiler decides*. Every line of output is a heap-allocation count per call, measured with `testing.AllocsPerRun`:

- Inlining kills escapes: the slide's `HeapAlloc` and `GetConfigPointer` allocate nothing once inlined. A `//go:noinline` copy of `GetConfigPointer` allocates once per call.
- The size rule: `make([]int, 8)` stays on the stack, `make([]int, 1_000_000)` goes to the heap, and `make([]int, n)` with a small run-time `n` uses a stack buffer (Go 1.25).
- Boxing: storing a `BigStruct` in an `any` allocates; storing `42`, or any run-time value from 0 to 255, does not; storing `1000` does.
- What the pointer is used for: `Process(&u)` only reads through it and nothing escapes; `store(&u)` keeps it in a global and `u` moves to the heap.

## Run

```sh
go run ./examples/016-escape-analysis
go test ./examples/016-escape-analysis
go build -gcflags=-m -o bin/ ./examples/016-escape-analysis
go build -gcflags=-m=2 -o bin/ ./examples/016-escape-analysis   # adds the reason for each decision
go test -run='^$' -bench=. -benchmem ./examples/016-escape-analysis
```

## What to look for

Expected output of `go run`:

```text
== 1. Inlining decides escapes (slides p03, p07, p08) ==
HeapAlloc() *int, inlined                    0 allocs/call
GetConfigPointer(), inlined                  0 allocs/call
GetConfigPointerNoInline(), //go:noinline    1 allocs/call
GetConfigValue() Config                      0 allocs/call
GetConfigPointer(), result stored globally   1 allocs/call

== 2. The size rule (slide p04) ==
make([]int, 8)                               0 allocs/call
make([]int, 1_000_000)                       1 allocs/call
make([]int, n), n = 4 at run time            0 allocs/call
make([]int, n), n = 100 at run time          1 allocs/call

== 3. Interface boxing (slides p05, p06) ==
sinkAny = bigStruct                          1 allocs/call
sinkAny = 42 (constant)                      0 allocs/call
sinkAny = smallValue (7 at run time)         0 allocs/call
sinkAny = largeValue (1000 at run time)      1 allocs/call
fmt.Println(x) prints: 42

== 4. What the pointer is used for (slide p03) ==
Process(&u)                                  0 allocs/call
store(&u)                                    1 allocs/call
```

The relevant lines of `go build -gcflags=-m -o bin/ ./examples/016-escape-analysis` (Windows prints the directory part with backslashes):

```text
examples/016-escape-analysis/main.go:62:6: can inline HeapAlloc
examples/016-escape-analysis/main.go:74:6: can inline GetConfigPointer
examples/016-escape-analysis/main.go:91:67: inlining call to HeapAlloc
examples/016-escape-analysis/main.go:92:75: inlining call to GetConfigPointer
examples/016-escape-analysis/main.go:98:90: inlining call to GetConfigPointer
examples/016-escape-analysis/main.go:92:75: &Config{...} does not escape
examples/016-escape-analysis/main.go:83:9: &Config{...} escapes to heap
examples/016-escape-analysis/main.go:98:90: &Config{...} escapes to heap
examples/016-escape-analysis/main.go:109:11: make([]int, 8) does not escape
examples/016-escape-analysis/main.go:115:11: make([]int, 1000000) escapes to heap
examples/016-escape-analysis/main.go:121:11: make([]int, n) does not escape
examples/016-escape-analysis/main.go:146:51: bigStruct escapes to heap
examples/016-escape-analysis/main.go:147:55: 42 escapes to heap
examples/016-escape-analysis/main.go:148:68: smallValue escapes to heap
examples/016-escape-analysis/main.go:149:71: largeValue escapes to heap
examples/016-escape-analysis/main.go:154:40: 42 escapes to heap
examples/016-escape-analysis/main.go:167:14: u does not escape
examples/016-escape-analysis/main.go:172:12: leaking param: u
examples/016-escape-analysis/main.go:182:3: moved to heap: u
examples/016-escape-analysis/main.go:63:2: moved to heap: x
examples/016-escape-analysis/main.go:75:9: &Config{...} escapes to heap
```

- **Inlining (lines 63, 75, 92):** `moved to heap: x` (63) and `&Config{...} escapes to heap` (75) are about the out-of-line bodies of `HeapAlloc` and `GetConfigPointer`. At the inlined call site, the same `&Config{...}` `does not escape` (92), so the measured cost is 0. The `//go:noinline` copy escapes (83), and so does the inlined one whose result goes into a package variable (98).
- **Size (lines 109-121):** a constant 64-byte slice stays on the stack; the 8 MB one is over the 64 KB limit for `make`. `make([]int, n) does not escape` (121) gets a 32-byte stack buffer, so `n = 4` is free and `n = 100` falls back to the heap at run time.
- **Boxing (lines 146-154):** `-m` says every one of these "escapes to heap", but only `bigStruct` and `largeValue` allocate. Constants are boxed from read-only data, and the runtime keeps a static table for the values 0-255. Line 154 is the slide p06 snippet: Go 1.26 prints `42 escapes to heap` (the constant-propagated value), not `x escapes to heap`.
- **Pointer use (lines 167-182):** `Process`'s parameter `does not escape`, so the caller's `u` stays on the stack. `store` leaks its parameter, so the caller's `u` is `moved to heap` (182).

`-gcflags=-m=2` explains each decision as a chain of data flows. For the out-of-line constructor and for `store(&u)`:

```text
examples/016-escape-analysis/main.go:83:9: &Config{...} escapes to heap in GetConfigPointerNoInline:
examples/016-escape-analysis/main.go:83:9:   flow: ~r0 ← &{storage for &Config{...}}:
examples/016-escape-analysis/main.go:83:9:     from &Config{...} (spill) at examples/016-escape-analysis/main.go:83:9
examples/016-escape-analysis/main.go:83:9:     from return &Config{...} (return) at examples/016-escape-analysis/main.go:83:2
examples/016-escape-analysis/main.go:172:12: parameter u leaks to {heap} for store with derefs=0:
examples/016-escape-analysis/main.go:172:12:   flow: {heap} ← u:
examples/016-escape-analysis/main.go:172:12:     from global = u (assign) at examples/016-escape-analysis/main.go:173:9
examples/016-escape-analysis/main.go:182:3: u escapes to heap in pointerUse.func2:
examples/016-escape-analysis/main.go:182:3:   flow: {heap} ← &u:
examples/016-escape-analysis/main.go:182:3:     from &u (address-of) at examples/016-escape-analysis/main.go:183:9
examples/016-escape-analysis/main.go:182:3:     from store(&u) (call parameter) at examples/016-escape-analysis/main.go:183:8
```

Read it bottom-up: `&u` is passed to `store`, `store` assigns its parameter to a global (`{heap}`), so `u` must live on the heap.

Benchmark shape (numbers vary by machine):

```text
BenchmarkHeapAllocInlined           ~0.5 ns/op          0 B/op   0 allocs/op
BenchmarkGetConfigPointerInlined    ~0.7 ns/op          0 B/op   0 allocs/op
BenchmarkGetConfigPointerNoInline    ~16 ns/op         16 B/op   1 allocs/op
BenchmarkGetConfigValue             ~0.8 ns/op          0 B/op   0 allocs/op
BenchmarkMakeConst8                 ~2.1 ns/op          0 B/op   0 allocs/op
BenchmarkMakeConst1M               ~310 µs/op  ~8,000,000 B/op   1 allocs/op
BenchmarkMakeVariable4              ~2.2 ns/op          0 B/op   0 allocs/op
BenchmarkMakeVariable100            ~160 ns/op        896 B/op   1 allocs/op
BenchmarkBoxBigStruct                ~17 ns/op         32 B/op   1 allocs/op
BenchmarkBoxConstant                ~1.4 ns/op          0 B/op   0 allocs/op
BenchmarkBoxSmallValue              ~2.7 ns/op          0 B/op   0 allocs/op
BenchmarkBoxLargeValue               ~10 ns/op          8 B/op   1 allocs/op
BenchmarkProcessPointer             ~1.5 ns/op          0 B/op   0 allocs/op
BenchmarkStorePointer                ~20 ns/op         24 B/op   1 allocs/op
```

`TestBoxingAllocs` pins the allocation counts that do not depend on inlining, so `go test` fails if they ever change.

## Notes and corrections

- **Slides p07, p08:** the "1 alloc/op" results do not happen in practice. `HeapAlloc` and `GetConfigPointer` are inlined, and both measure **0 allocs/op** on Go 1.26.4. Showing the escape needs `//go:noinline`, or a caller that stores the result somewhere that outlives it.
- **Slide p06:** the compiler output on the slide is not what Go prints. The line numbers point at line 199 of a 7-line file, and Go 1.26 reports `42 escapes to heap` (the constant-propagated value) rather than `x escapes to heap`.
- **Slide p05:** boxing small integers and constants into an interface does not allocate, even though `-m` says "escapes to heap". Boxing larger, non-constant values does.
- **Slide p04:** the limits are 128 KB for explicitly declared variables and 64 KB for implicit allocations with `new`, `make` or `&T{}`. The reason is a stack-frame size policy, not overflow prevention, since goroutine stacks grow. Since Go 1.25, a small variable-size `make` can use the stack too.
- **Slide p08:** "return pointers only for structs over 256 bytes" is a heuristic, not a rule of the compiler. Measure with `-benchmem` before deciding.

## Slides

[016-escape-analysis.pdf](../../slides/016-escape-analysis.pdf)
