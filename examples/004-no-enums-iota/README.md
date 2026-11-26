# Part 004: We don't have enum, we got iota

Day 02 · Week 1 · Building type-safe enums and state machines in Go.

## What this shows

- `type OrderStatus int` with an `iota` const block that puts `Unknown` at 0, a bounds-safe `String()` that prints `OrderStatus(42)` instead of panicking, and `IsValid()`.
- The limit of the type safety: `HandleOrder(2)` compiles because untyped constants convert implicitly; an `int` variable does not (shown in a comment with the real compiler message); an explicit conversion always does.
- `HandleOrder` as the slide p09 state machine, with a case for every state and a `default:` branch that rejects invalid values.
- A bitmask `Role` built with `1 << iota`: combine with `|`, test with `&`, remove with `&^`, printed in binary with `%04b`.
- iota tricks: `_` to skip 0, an `iota + 10` offset, and `KB`/`MB`/`GB` with `1 << (10 * iota)`.

## Run

```sh
go run ./examples/004-no-enums-iota
go test ./examples/004-no-enums-iota
```

## What to look for

Expected output of `go run`:

```text
== 1. OrderStatus with a bounds-safe String (slides p04, p05, p07) ==
Unknown    = 0
Pending    = 1
Processing = 2
Shipped    = 3
Delivered  = 4
OrderStatus(42) valid=false

== 2. The type-safety limit (slide p04) ==
HandleOrder(2): processing: pack the box
HandleOrder(OrderStatus(99)): rejected: invalid status OrderStatus(99)

== 3. State machine with a default branch (slide p09) ==
unknown status: something is wrong upstream, alert
pending: charge the card
processing: pack the box
shipped: send the tracking email
delivered: ask for a review
rejected: invalid status OrderStatus(-1)

== 4. Bitmask roles with 1 << iota (slide p08) ==
Admin  = 1 (0001)
Editor = 2 (0010)
Viewer = 4 (0100)
Admin | Viewer      = 0101
has Admin?  (&)     = true
has Editor? (&)     = false
after &^= Admin     = 0100, has Admin? false

== 5. iota tricks: skip, offset, sizes (slides p02, p06) ==
skip with _:      A, B, C = 1 2 3
offset iota + 10: Small, Medium, Large = 10 11 12
sizes 1 << (10 * iota): KB, MB, GB = 1024 1048576 1073741824
```

- `%v` calls `String()`, `%d` prints the underlying integer. `OrderStatus(42)` prints its number instead of crashing, the same fallback `stringer` generates.
- `HandleOrder(2)` and `HandleOrder(OrderStatus(99))` both compile. A named type documents intent and catches an `int` variable passed by mistake, but it does not stop invalid values, which is why `IsValid()` and the `default:` branch exist.
- `&^` is Go's "AND NOT" (bit clear) operator: `perms &^= Admin` turns the Admin bit off and leaves the others alone.
- In the sizes block, the blank `_ = iota` line consumes 0, so `KB` sees `iota == 1`.
- `enum_test.go` has table-driven tests for `String()`/`IsValid()` (including out-of-range values), `HandleOrder`'s default branch and `Role.Has`; every section's output is asserted by an `Example`.

## Notes and corrections

- **Slide p02:** the constants are declared as `PackageStatus`, but the type is `OrderStatus`, so the snippet fails with `undefined: PackageStatus`. Slide p04 has it right.
- **Slide p04:** "strictly require an OrderStatus" is overstated. Untyped constants convert implicitly (`HandleOrder(2)` compiles), only typed variables are rejected (`cannot use n (variable of type int) as OrderStatus value in argument to HandleOrder`), and Go enums are open sets. Validate values that come from outside.
- **Slide p07:** indexing `[...]string{...}[s]` panics for any out-of-range value: `panic: runtime error: index out of range [42] with length 3`. Through `fmt` the panic is caught, but the log line becomes `%!v(PANIC=String method: runtime error: index out of range [42] with length 3)`. Bounds-check it, as `stringer`'s generated code does.
- **Slide p09:** the switch has no `default:`, so `Processing`, `Delivered` and invalid values are silently ignored. Add a `default:` branch (the `exhaustive` linter can also check that every constant has a case).
- **Slide p10:** the `stringer` tool is `golang.org/x/tools/cmd/stringer`. Since Go 1.24 it can be pinned in go.mod with the `tool` directive: `go get -tool golang.org/x/tools/cmd/stringer`, then `//go:generate go tool stringer -type=OrderStatus`. This example keeps the hand-written `String()` to stay standard-library only.

## Slides

[004-no-enums-iota.pdf](../../slides/004-no-enums-iota.pdf)
