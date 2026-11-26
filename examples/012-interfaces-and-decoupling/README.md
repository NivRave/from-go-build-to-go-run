# Part 012: Interfaces & the decoupling secret

Day 06 · Week 1 · Why you should accept interfaces and return concrete structs.

## What this shows

- Implicit satisfaction: a `Notifier` interface defined where it is consumed, satisfied by `EmailSender` and `SMSSender`, which never mention it.
- `Save(w io.Writer, u User)` called with `os.Stdout`, a `*bytes.Buffer` and a fake writer (and, in `main_test.go`, a writer that always fails).
- Return concrete types: `OpenFile` returns `*File`, so the caller can use `Name()` and `Size()` and still pass it to a function that takes an `io.Reader`.
- A type switch, a comma-ok assertion that fails with `ok == false`, and a single-result assertion whose panic is recovered and printed.
- The method-set trap: a pointer-receiver `Area` on `*Square`, the compile-time check `var _ Shaper = (*Square)(nil)`, and why `Square{5}` does not satisfy `Shaper`.

## Run

```sh
go run ./examples/012-interfaces-and-decoupling
go test ./examples/012-interfaces-and-decoupling
go build -gcflags=-m ./examples/012-interfaces-and-decoupling   # optional: where &File{} escapes
```

## What to look for

Expected output of `go run`:

```text
== 1. Implicit satisfaction (slide p02) ==
email to ops@example.com: disk 90% full
sms to +1-555-0100: disk 90% full

== 2. Accept interfaces: Save(w io.Writer) (slide p03) ==
os.Stdout:     user id=1 name=Ada
bytes.Buffer:  holds "user id=1 name=Ada\n"
fake writer:   1 write, 19 bytes

== 3. Return structs: OpenFile returns *File (slide p04) ==
Name() = notes.txt, Size() = 33
countBytes(f) = 33, err = <nil>
from io.Reader, after an assertion: other.txt

== 4. Type switches and assertions (slide p07) ==
Integer: 42
String: hello
Other: float64
i.(int) with comma-ok: n = 0, ok = false
i.(int) without comma-ok: recovered: interface conversion: interface {} is string, not int

== 5. The method-set trap (slide p08) ==
(&Square{5}).Area() via Shaper = 25
sq.Area() on an addressable value = 9
```

- `Save` never changes, yet it writes to the terminal, to memory and to a fake. That is what makes it testable without files.
- Returning `io.Reader` hides `Name()`; getting it back needs a type assertion. Returning `*File` costs callers nothing, because `*File` still satisfies `io.Reader`.
- The value form of the compliance check does not compile. Go 1.26 reports:

```text
cannot use Square{…} (value of struct type Square) as Shaper value in variable declaration: Square does not implement Shaper (method Area has pointer receiver)
```

- Return types do not decide where data lives. In this `go build -gcflags=-m` excerpt (directory prefix removed), the `*File` from `OpenFile` escapes at line 144 because `countBytes` hands it to `io.Copy`, while the one returned as an `io.Reader` at line 151 stays on the stack, because the inlined call's result is only type-asserted:

```text
main.go:122:6: can inline OpenFile
main.go:144:15: inlining call to OpenFile
main.go:151:20: inlining call to OpenFile
main.go:144:15: &File{...} escapes to heap
main.go:151:20: &File{...} does not escape
main.go:220:56: devirtualizing s.Area to *Square
```

## Notes and corrections

- **Slide p05:** the subtitle "Stop creating one-method interfaces" contradicts its own Rob Pike quote. One-method interfaces such as `io.Reader`, `fmt.Stringer` and `error` are the ideal; the actual advice is not to create interfaces speculatively, before a second implementation needs one.
- **Slide p04:** returning a pointer does not keep data on the stack. Whether `&File{...}` escapes depends on inlining and on what the caller does with it (see the `-m` output above). The real benefits of returning a concrete type are that callers see every method, and that calls avoid interface boxing and dynamic dispatch, so they can be inlined.
- **Slide p04:** the snippets on p02 and p04 use different `Read` signatures (`Read() string` vs `io.Reader`'s `Read([]byte) (int, error)`), so they do not compile together. This example uses `io.Reader` throughout.
- **Slide p07:** "always use comma-ok" is too strong. A single-result assertion is fine when a failure can only mean a programmer bug, and a type switch never panics.
- **Slide p08:** the compliance check assigns "a nil pointer of type `*Square`", not of type `Square`.

## Slides

[012-interfaces-and-decoupling.pdf](../../slides/012-interfaces-and-decoupling.pdf)
