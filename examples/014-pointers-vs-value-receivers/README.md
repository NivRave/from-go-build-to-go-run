# Part 014: Pointer vs. value receivers

Day 07 · Week 1 · Choosing the right receiver for your methods.

## What this shows

- `Counter` with a value-receiver `Increment` (11 inside, 10 outside) next to a pointer-receiver `SharedCounter` (11 and 11).
- Method sets: `var _ Namer = &User{}` compiles, the value form does not, and `u.UpdateName("x")` still works on an addressable variable.
- Value receivers make shallow copies: a value-receiver method that writes an element of a slice field changes the caller's data.
- A nil receiver: `(*User)(nil).GetName()` returns `"Guest"`. Then the typed-nil gotcha: a nil `*User` stored in an interface is `!= nil`.
- `-race-demo`: one goroutine writes a struct while another calls a value-receiver method on it. The race detector reports the copy, which disproves the "thread-safety bonus".
- Benchmarks: a small struct passed by value vs by pointer (both free), and returned by value vs by pointer (the pointer allocates).

## Run

```sh
go run ./examples/014-pointers-vs-value-receivers
go test ./examples/014-pointers-vs-value-receivers
go test -run='^$' -bench=. -benchmem ./examples/014-pointers-vs-value-receivers
go run -race ./examples/014-pointers-vs-value-receivers -race-demo   # needs cgo (a C compiler)
```

## What to look for

Expected output of `go run`:

```text
== 1. Value receiver copies, pointer receiver shares (slides p02, p03) ==
Inside method (copy): 11
Original caller: 10
Inside method (pointer): 11
Original caller: 11

== 2. Method sets and interfaces (slide p04) ==
u.UpdateName on a variable: Grace
rename(&u, ...) via Namer:  Linus

== 3. Value receivers make shallow copies (slides p02, p08) ==
inside Shout:  MIX [INTRO outro]
caller after:  mix [INTRO outro]

== 4. Nil receivers and the typed-nil gotcha (slide p07) ==
(*User)(nil).GetName() = Guest
findUser("") == nil:      false (dynamic type *main.User, GetName "Guest")
findUserFixed("") == nil: true
```

- The value form of the interface check, and calling a pointer method on a non-addressable value, fail to compile with:

```text
cannot use User{} (value of struct type User) as Namer value in variable declaration: User does not implement Namer (method UpdateName has pointer receiver)
cannot call pointer method UpdateName on User
```

- In section 3 the copy's `Name` change stays inside `Shout`, but `INTRO` reaches the caller: the copied slice header points at the same backing array.
- `findUser("")` returns an interface holding `(*User)(nil)`. It has a type, so it is not `nil`, and a caller's `if u == nil` check passes it through. Return a literal `nil` interface (as `findUserFixed` does) when there is nothing to return. Staticcheck's `SA4023` reports this pattern, so `main.go` suppresses it on purpose.
- Without `-race`, `-race-demo` just prints `done; run with go run -race to see the report`. With `-race`, the detector reports a `DATA RACE` between the write to `cfg.Timeout` and the read that copies `cfg` for `Describe`. (The race detector needs cgo; that run is not reproduced here.)

Benchmark shape (numbers vary by machine):

```text
BenchmarkPassByValue      ~1.3 ns/op    0 B/op   0 allocs/op
BenchmarkPassByPointer    ~1.4 ns/op    0 B/op   0 allocs/op
BenchmarkReturnValue      ~1.6 ns/op    0 B/op   0 allocs/op
BenchmarkReturnPointer   ~15.6 ns/op   24 B/op   1 allocs/op
```

- Passing a 24-byte struct by value costs the same as passing a pointer to it, and neither allocates: a pointer passed *down* the call stack does not escape.
- Returning a pointer from a function that is not inlined moves the value to the heap: one allocation and about ten times the cost.

## Notes and corrections

- **Slide p02:** the "thread-safety bonus" does not exist. Making the receiver copy reads the caller's struct with no synchronisation, so a concurrent write to it is a data race. A value receiver only isolates the method from writes made *after* the copy.
- **Slides p02, p08:** value receivers make shallow copies. Slices, maps, pointers and channels inside the struct still refer to the caller's data, so a value receiver does not guarantee immutability.
- **Slide p03:** a pointer is 8 bytes on 64-bit platforms and 4 bytes on 32-bit ones.
- **Slide p06:** "don't mix receivers" is the usual guidance, but the standard library mixes them on purpose where it has to: `time.Time` uses value receivers except for methods that must modify it, such as `UnmarshalJSON`.
- **Slide p07:** a nil receiver is safe to call, but a nil pointer stored in an interface makes the interface non-nil (section 4).

## Slides

[014-pointers-vs-value-receivers.pdf](../../slides/014-pointers-vs-value-receivers.pdf)
