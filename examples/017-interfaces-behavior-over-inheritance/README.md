# Part 017: Interfaces - behavior over inheritance

Day 09 · Week 2 · Why Go's "implicit" interfaces change everything.

## What this shows

- Discovering an interface: `ConsoleWriter` and `memLogger` are written first, and the `Logger` interface is extracted afterwards. Neither type mentions it.
- "Accept interfaces": one `SaveData(w io.Writer, data []byte)` writes to `os.Stdout`, a `*bytes.Buffer`, a temporary file and one end of `net.Pipe()`, an in-process `net.Conn`.
- Composition: a `ReadWriter` built by embedding `io.Reader` and `io.Writer`, and a struct that embeds an `io.Writer` and overrides `Write` to count bytes.
- The compile-time check `var _ Logger = (*memLogger)(nil)`, and why the value type `memLogger` is not a `Logger` (pointer receiver).
- `any` versus generics: `SumAny([]any)` needs a type switch and fails at run time, while `Sum[T int | float64]` rejects bad input at compile time.

## Run

```sh
go run ./examples/017-interfaces-behavior-over-inheritance
go test ./examples/017-interfaces-behavior-over-inheritance
```

## What to look for

Expected output of `go run`:

```text
== 1. Discovering an interface (slides p02, p05) ==
hello, gopher
memLogger captured ["hello, gopher"]

== 2. Accept interfaces: one SaveData, four destinations (slides p03-p04) ==
os.Stdout     -> payload
*bytes.Buffer -> "payload\n"
*os.File      -> "payload\n" (temp file)
net.Conn      -> "payload\n" (net.Pipe)

== 3. Composition via embedding (slide p08) ==
ReadWriter round trip: "composed"
countingWriter saw 24 bytes: "hello, composition! more"

== 4. Compile-time satisfaction check (slide p07) ==
*memLogger, ConsoleWriter and *countingWriter were checked by the compiler
memLogger (value) is a Logger: false
*memLogger is a Logger:       true

== 5. any vs generics (slide p06) ==
Sum[int]:     6
Sum[float64]: 4.5
SumAny:       7.5 <nil>
SumAny:       0 element 2: unsupported type string
```

- In section 1 `Greet` runs twice: `ConsoleWriter` prints its line, and `memLogger` keeps its line for later inspection. `Greet` never knows which one it has.
- Section 2 passes four unrelated concrete types (`*os.File` twice, `*bytes.Buffer`, the pipe's `net.Conn`) to the same function, without changing a line of it.
- `countingWriter` is an `io.Writer` because it embeds one. Its own `Write` shadows the promoted method, counts, and delegates to the embedded writer. `fmt.Fprintf` accepts it like any other writer.
- `SumAny` compiles happily with a string in the slice and fails only when it runs. The same mistake with `Sum` is a compile error (quoted in `main.go`).

## Notes and corrections

- **Slide p03:** `*net.Conn` does not implement `io.Reader` or `io.Writer`. `net.Conn` is already an interface, and a pointer to an interface has no methods. The compiler says `type *net.Conn is pointer to interface, not interface`. Use `net.Conn`, or a concrete type such as `*net.TCPConn`.
- **Slide p04:** the two `SaveData` declarations are a side-by-side comparison. In one file they would be a redeclaration error. The example's `SaveData` also returns the write error.
- **Slide p07:** as written, the check fails to compile because `MyStruct` has no `DoWork` method (`*MyStruct does not implement MyInterface (missing method DoWork)`). Adding `func (*MyStruct) DoWork() {}` fixes it. The idiom is useful in any package, not only internal ones.
- **Slide p06:** for generic data, generics (Go 1.18) are the type-safe alternative to `any`. For logging on hot paths, `log/slog` (Go 1.21) offers typed `slog.Attr` values that avoid boxing into `any`.

## Slides

[017-interfaces-behavior-over-inheritance.pdf](../../slides/017-interfaces-behavior-over-inheritance.pdf)
