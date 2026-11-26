# Part 015: Pointers - where does your data live?

Day 08 · Week 2 · Decoding the stack and the heap.

## What this shows

Each demo calls a small `//go:noinline` function 1000 times and counts the heap allocations (`runtime.MemStats.Mallocs`, measured around the loop only):

- Returning a pointer moves the value to the heap (`newUserPtr() *User`); returning a value does not (`newUserVal() User`).
- Passing a pointer **down** the call stack does not escape (`process(u *User)`); storing it in a global does (`keep(u *User)`).
- Interface boxing: passing a 64-byte struct to a function that takes `any` and keeps it allocates; passing the struct itself does not.
- Slices: a constant-size `make([]int, 8)` stays on the stack, a `make([]int, n)` whose size is only known at run time goes to the heap, and so does a returned slice.
- Deep recursion grows the goroutine stack (`StackInuse` goes from a fraction of a MB to about 16 MB) instead of overflowing.

## Run

```sh
go run ./examples/015-pointers
go test ./examples/015-pointers
go build -gcflags=-m ./examples/015-pointers
go test -run='^$' -bench=. -benchmem ./examples/015-pointers
```

## What to look for

Expected output of `go run` (the `StackInuse` figures vary a little by platform):

```text
== 1. Returning a pointer vs a value (slide p04) ==
newUserPtr() *User                 1000 heap allocations in 1000 calls
newUserVal() User                     0 heap allocations in 1000 calls

== 2. Passing a pointer down vs storing it (slides p04, p05) ==
process(&u): pointer passed down      0 heap allocations in 1000 calls
keep(&u): pointer stored globally  1000 heap allocations in 1000 calls

== 3. Interface boxing (slide p05) ==
logReport(r Report)                   0 heap allocations in 1000 calls
logValue(r) taking any             1000 heap allocations in 1000 calls

== 4. Slices: stack or heap? (slide p08) ==
make([]int, 8), local                 0 heap allocations in 1000 calls
make([]int, n) n=1000, local       1000 heap allocations in 1000 calls
make([]int, 8), returned           1000 heap allocations in 1000 calls

== 5. The stack grows; it does not overflow (slide p02) ==
StackInuse before:            0.25 MB
StackInuse at depth 100000:  16.22 MB
no stack overflow: the runtime copied the stack to a bigger one as it grew
```

The compiler explains each result. These are the relevant lines of `go build -gcflags=-m ./examples/015-pointers` (Windows prints the directory part with backslashes):

```text
examples/015-pointers/main.go:84:2: moved to heap: u
examples/015-pointers/main.go:100:14: u does not escape
examples/015-pointers/main.go:105:11: leaking param: u
examples/015-pointers/main.go:115:3: moved to heap: u
examples/015-pointers/main.go:128:15: leaking param: v
examples/015-pointers/main.go:140:53: r escapes to heap
examples/015-pointers/main.go:148:11: make([]int, 8) does not escape
examples/015-pointers/main.go:160:11: make([]int, size) does not escape
examples/015-pointers/main.go:167:13: make([]int, 8) escapes to heap
```

- `moved to heap: u` at line 84 is `newUserPtr`'s local; at line 115 it is the caller's `u`, because `keep` leaks its parameter (`leaking param: u`, line 105).
- `u does not escape` (line 100) is `process`'s parameter. Passing `&u` to it costs nothing.
- `r escapes to heap` (line 140) is the conversion of `Report` to `any` for `logValue`, whose parameter leaks (line 128).
- `make([]int, size) does not escape` (line 160) and yet it allocates: since Go 1.25 the compiler gives a non-escaping variable-size `make` a 32-byte stack buffer and falls back to the heap when the run-time size does not fit. 1000 ints do not; 4 would.

Benchmark shape (numbers vary by machine):

```text
BenchmarkNewUserPtr            ~23 ns/op     48 B/op   1 allocs/op
BenchmarkNewUserVal           ~7.6 ns/op      0 B/op   0 allocs/op
BenchmarkProcessPassDown      ~1.5 ns/op      0 B/op   0 allocs/op
BenchmarkKeepGlobal            ~23 ns/op     48 B/op   1 allocs/op
BenchmarkPassStruct           ~1.5 ns/op      0 B/op   0 allocs/op
BenchmarkBoxInterface          ~21 ns/op     64 B/op   1 allocs/op
BenchmarkSliceLocalConst      ~1.5 ns/op      0 B/op   0 allocs/op
BenchmarkSliceLocalVariable  ~1100 ns/op   8192 B/op   1 allocs/op
BenchmarkSliceReturned         ~19 ns/op     64 B/op   1 allocs/op
```

- Every heap allocation costs roughly 15-20 ns here, on top of the GC work it causes later. Copying a 40-byte `User` by value is cheaper.

## Notes and corrections

- **Slides p04, p05, p09:** passing a pointer down the call stack usually does **not** cause an escape (demo 2). Values escape when a pointer to them is returned, stored in a global or a heap object, sent on a channel, captured by a goroutine, or boxed into an interface whose use the compiler cannot see.
- **Slide p08:** a slice's backing array is not "almost always on the heap". Non-escaping constant-size slices up to 64 KB live on the stack, and Go 1.25 added stack buffers for small variable-size ones (demo 4). Since Go 1.24, map internals are Swiss tables rather than buckets (see Part 010).
- **Slides p03, p07:** Go's allocator does not search a free list. Each P (processor) has a cache of spans sorted into size classes, so a small allocation is close to a pointer bump. The cost of the heap is mostly GC work.
- **Slide p07:** the GC is concurrent; its stop-the-world pauses are short, and its mark cost scales with the *live* heap, not with how much garbage was created. Garbage makes GC cycles more frequent.
- **Slide p02:** large locals are moved to the heap automatically, so big data does not overflow the stack. Goroutine stacks start small and grow by copying (demo 5); a Go stack overflow comes from unbounded recursion hitting the 1 GB maximum on 64-bit platforms.

## Slides

[015-pointers.pdf](../../slides/015-pointers.pdf)
