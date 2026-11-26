# Part 019: Iteration - for & range

Day 10 · Week 2 · One loop to rule them all.

## What this shows

- The three `for` forms, plus `for i := range 3` (Go 1.22) and ranging over a custom `iter.Seq[int]` (Go 1.23).
- Ranging over `"héllo, 世界"`: the index is a byte offset and the value is a rune, so `len` (bytes) and `utf8.RuneCountInString` (runes) differ.
- The copy trap from slide p04, in a version that compiles: a value loop changes a copy, while `users[i].Active = true` changes the element.
- Per-iteration loop variables: collecting `&v` from each iteration gives three distinct pointers.
- Map iteration order is randomised. The same map is ranged three times, then printed deterministically with `slices.Sorted(maps.Keys(m))`.
- A manual in-place filter versus `slices.DeleteFunc` (which zeroes the tail), and ranging over a channel whose producer closes it.

## Run

```sh
go run ./examples/019-iteration
go test ./examples/019-iteration
```

## What to look for

Expected output of `go run` (the three `range #` lines in section 5 vary from run to run):

```text
== 1. The faces of for (slides p02-p03) ==
C-style:          [0 1 2]
while-style:      [1 2 4 8]
infinite + break: stopped after 3 tries
range over int:   [0 1 2]
range over func:  [3 2 1]

== 2. Ranging over a string: byte offsets and runes (slide p03) ==
0:'h' 1:'é' 3:'l' 4:'l' 5:'o' 6:',' 7:' ' 8:'世' 11:'界'
len (bytes):             14
utf8.RuneCountInString:  9

== 3. The copy trap (slide p04) ==
value loop: [{ID:1 Active:false} {ID:2 Active:false}]
index loop: [{ID:1 Active:true} {ID:2 Active:true}]

== 4. Loop variables are per iteration since Go 1.22 (slide p05) ==
values: 1 2 3
all distinct: true

== 5. Map order is randomised (slide p06) ==
range #1: Epsilon Alpha Beta Gamma Delta
range #2: Alpha Beta Gamma Delta Epsilon
range #3: Gamma Delta Epsilon Alpha Beta
sorted:   Alpha:1 Beta:2 Delta:4 Epsilon:5 Gamma:3

== 6. In-place filter, slices.DeleteFunc, and range over a channel (slides p07-p08) ==
manual filter:      [2 4 6] backing array: [2 4 6 4 5 6]
slices.DeleteFunc:  [2 4 6] backing array: [2 4 6 0 0 0]
Processing: build
Processing: test
Processing: deploy
after close: v="" ok=false
```

- The byte offsets jump from 1 to 3 after `é` (2 bytes in UTF-8) and from 8 to 11 after `世` (3 bytes). The string is 14 bytes but only 9 runes.
- `all distinct: true` holds because this module's `go.mod` says `go 1.26.0`. The same loop in a module that says `go 1.21` prints `3 3 3` and shares one pointer, even when built with Go 1.26. The language version comes from `go.mod`, not from the toolchain.
- In section 5, the three lines often differ within a single run. With a map this small, Go 1.26 picks a random starting point each time, so you see rotations of one order rather than arbitrary shuffles. Either way, never depend on it.
- The manual filter leaves `4 5 6` in the backing array beyond the new length. In a slice of pointers, those stale entries keep objects alive. `slices.DeleteFunc` overwrites them with zero values.
- After the channel is closed and drained, `range` ends, and a manual receive returns the zero value and `ok=false`.

## Notes and corrections

- **Slide p04:** the composite literal is missing its trailing commas after `Active: false` and the inner `}`, so it doesn't compile: `syntax error: unexpected newline in composite literal; possibly missing comma or }`.
- **Slide p05:** `for i, v := range` doesn't compile because `i` is unused (`declared and not used: i`); use `for _, v := range`. Per-iteration variables are selected per module by the `go` line in `go.mod` (1.22 or later), not by the toolchain version. "Result: [3, 3, 3]" describes the dereferenced values; printing `out` directly shows three equal addresses.
- **Slides p02-p03:** since Go 1.22, `range` also works over integers (`for i := range 10` replaces most C-style loops), and since Go 1.23 over iterator functions (`iter.Seq`, `maps.Keys`, `slices.Values`). Ranging over invalid UTF-8 yields U+FFFD.
- **Slide p06:** `fmt.Println(m)` prints maps with sorted keys (since Go 1.12), which can make the order look stable. The modern way to iterate in order is `slices.Sorted(maps.Keys(m))` (Go 1.23).
- **Slide p07:** the in-place filter is correct and allocation-free, but leaves stale elements in the tail. `slices.DeleteFunc` does the same job and, since Go 1.22, zeroes the obsolete tail.
- **Slide p08:** the manual form is `v, ok := <-queue`, not `if ok := <-queue`. As shown, the snippet would block forever because nothing sends or closes.

## Slides

[019-iteration.pdf](../../slides/019-iteration.pdf)
