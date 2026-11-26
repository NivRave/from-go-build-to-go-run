# Part 013: Structs & methods

Day 07 · Week 1 · Beyond grouping variables: composition, alignment, and state.

## What this shows

- Layout: `unsafe.Sizeof`, `Alignof` and `Offsetof` for `{bool, int64, bool}` (24 bytes) vs the same fields ordered `{int64, bool, bool}` (16 bytes).
- Useful zero values: `var c Config` with an unexported `mu sync.Mutex` and a nil slice that `append` works on, plus a zero `bytes.Buffer`.
- Embedding: `Admin{User}` gets `User`'s fields and methods, satisfies a `Greeter` interface through the promoted `Greet`, but cannot be passed where a `User` is expected.
- Struct tags: `json.Marshal` with `omitempty` vs `omitzero` (Go 1.24) on a `time.Time` and an `int` 0, and reading a `db` tag with `reflect`.
- Encapsulation: an `Account` that stores `int64` cents behind `NewAccount`, `Deposit` and `Balance`, and `NewDatabase("")` returning an error.
- `vetdemo.go`: copying a `Config` copies its mutex, which `go vet`'s `copylocks` check reports.

## Run

```sh
go run ./examples/013-structs-and-methods
go test ./examples/013-structs-and-methods
go vet -tags vetdemo ./examples/013-structs-and-methods   # reports the copied locks on purpose
```

## What to look for

Expected output of `go run` on a 64-bit platform:

```text
== 1. Field order and padding (slide p02) ==
Padded    size 24, align 8, offsets A=0 B=8 C=16
Optimized size 16, align 8, offsets B=0 A=8 C=9

== 2. Useful zero values (slide p03) ==
zero Config: Paths = [], nil = true
after AddPath: [/etc/app /usr/local/etc/app]
bytes.Buffer works from its zero value too

== 3. Embedding (slides p04, p06) ==
promoted field:  a.Name = Ada
promoted method: a.Greet() = Hello, Ada
as a Greeter:    Hello, Ada
a.User:          user #1 Ada

== 4. Struct tags: omitempty vs omitzero (slide p05) ==
{"id":1,"created":"0001-01-01T00:00:00Z"}
field ID: json="id" db="user_id"

== 5. Encapsulation and constructors (slides p07, p08) ==
rejected: deposit must be positive, got -500
ada's balance: 1050 cents ($10.50)
float64: 0.1 + 0.2 = 0.30000000000000004; cents: 10 + 20 = 30
NewDatabase(""): url required
NewDatabase(url): url = postgres://localhost/app, err = <nil>
```

- `Padded` wastes 7 bytes after `A` so that `B` starts on an 8-byte boundary, and 7 more at the end. On a 32-bit platform (`GOARCH=386`) `int64` is only 4-byte aligned, and the same program prints sizes 16 and 12: alignment depends on the type and the platform, not on the field's size.
- Passing `a` to `describeUser(u User)` does not compile:

```text
cannot use a (variable of struct type Admin) as User value in argument to describeUser
```

- In the JSON, `email`, `retries` and `updated` are gone, but `created` is still there: `omitempty` never omits a struct, and `omitzero` does. `retries` disappeared too, which is wrong if `0` retries is a real setting.
- `go vet -tags vetdemo ./examples/013-structs-and-methods` reports every copy of the lock (Windows prints the file paths with backslashes):

```text
examples/013-structs-and-methods/vetdemo.go:16:18: pathCount passes lock by value: github.com/NivRave/from-go-build-to-go-run/examples/013-structs-and-methods.Config contains sync.Mutex
examples/013-structs-and-methods/vetdemo.go:24:14: assignment copies lock value to snapshot: github.com/NivRave/from-go-build-to-go-run/examples/013-structs-and-methods.Config contains sync.Mutex
examples/013-structs-and-methods/vetdemo.go:25:24: call of pathCount copies lock value: github.com/NivRave/from-go-build-to-go-run/examples/013-structs-and-methods.Config contains sync.Mutex
```

## Notes and corrections

- **Slide p06:** `Greet` uses `u.Name`, but the `User` on that slide only has `ID` and `Email`, so it does not compile. This example's `User` has a `Name` field.
- **Slide p05:** "always use `omitempty`" needs two caveats. `omitempty` never omits structs such as `time.Time{}`, and it drops meaningful zeros such as `0` or `false`. Go 1.24 added `omitzero`, which omits zero structs (and uses an `IsZero()` method when the type has one); for a meaningful zero, drop the option or use a pointer.
- **Slide p03:** an exported `Mu sync.Mutex` lets any caller lock or unlock your internals. Keep it unexported (`mu`), and do not copy the struct: the copy gets its own lock, which `go vet`'s `copylocks` check reports.
- **Slide p02:** fields are aligned to their type's alignment, not its size. They are often the same on 64-bit platforms, but not always (`int64` is 4-byte aligned on 386, and a struct or array aligns to its most-aligned element).
- **Slide p07:** store money as integer cents (`int64`), not `float64`. The slide mentions cents as a possible later change; this example starts with them.

## Slides

[013-structs-and-methods.pdf](../../slides/013-structs-and-methods.pdf)
