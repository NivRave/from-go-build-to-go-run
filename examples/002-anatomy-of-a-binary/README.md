# Part 002: Anatomy of a binary

Day 01 · Week 1 · From package main to OS execution.

## What this shows

- The real startup order of a Go program, made visible with a `trace` slice: package-level variables are initialised in dependency order (not source order), then `init()` runs, and only then `main.main`. All of it happens after the runtime has started.
- The binary ties itself back to its module: `runtime.Version()` and the module path from `debug.ReadBuildInfo()`.

## Run

```sh
go run ./examples/002-anatomy-of-a-binary
go test ./examples/002-anatomy-of-a-binary
go build -o bin/hello ./examples/002-anatomy-of-a-binary
go build -ldflags="-s -w" -o bin/hello-small ./examples/002-anatomy-of-a-binary
go version -m bin/hello
```

On Windows, `go build` appends `.exe` only when `-o` is omitted, so name the outputs `hello.exe` and `hello-small.exe` there if you want to run them directly.

## What to look for

Expected output of `go run` (section 2 shows your toolchain version):

```text
== 1. Startup order (slides p04-p06) ==
1. var name     = loadName()     (declared second, initialised first)
2. var greeting = makeGreeting() (depends on name)
3. init()                        (greeting = Hello, Gopher!)
4. main.main

== 2. The binary knows where it came from (slide p07) ==
runtime.Version(): go1.26.4
module path:       github.com/NivRave/from-go-build-to-go-run
built with:        go1.26.4
```

- `greeting` is declared above `name` in the source, yet `name` is initialised first, because `greeting`'s initialiser reads it. Go sorts package initialisation by dependency.
- `init()` sees fully initialised package variables, and `main.main` comes last. `example_test.go` asserts steps 1-3: the test binary runs the same package initialisation, it just never calls `main`.
- Stripping the symbol table and DWARF debug info with `-ldflags="-s -w"` shrinks the binary noticeably: on one Windows/amd64 run, about 2.5 MB became about 1.7 MB. Sizes vary by platform and Go version.
- `go version -m bin/hello` prints the module path, the Go version, build settings such as `CGO_ENABLED`, `GOOS` and `GOARCH`, and (for `go build` inside a repository) the VCS revision, all read from the binary.

## Notes and corrections

- **Slide p06:** the OS doesn't look for `func main`. It jumps to the executable's entry point, which is runtime assembly (`_rt0_amd64_linux`, `_rt0_amd64_windows` and similar). That code sets up the heap, scheduler and GC, runs package initialisation and every `init()`, and only then calls `main.main`. The slide's code comment ("after the Go runtime is ready") is the accurate part.
- **Slide p05:** binary size comes from the linker's dead-code elimination, which drops unreachable code whether or not you import more packages. The unused-import error is about hygiene and build speed. Imports can have side effects through `init()`, which is why the blank import `import _ "pkg"` exists.
- **Slide p04:** `main` is not a reserved word; it is a package name that `go build` recognises. Building a non-main package produces no executable rather than a shared library; a shared library needs `-buildmode=c-shared`.
- **Slide p08:** prefer `go run .` and `go build .` (or a package path, as above) over `go run main.go`, which compiles only the named file and ignores the rest of the package. "Compile to a temp folder" is also out of date: since Go 1.24, `go run` caches the binaries it builds, so repeat runs start faster.

## Slides

[002-anatomy-of-a-binary.pdf](../../slides/002-anatomy-of-a-binary.pdf)
