# Part 001: Why Go?

Day 01 · Week 1 · Philosophy & strengths.

## What this shows

- "One self-describing binary": the program prints the OS and architecture it was compiled for, the Go version, and the CPU count and `GOMAXPROCS` it sees at run time.
- The binary carries its own build metadata: `debug.ReadBuildInfo()` returns the main module, the Go version and build settings, and VCS info when it was stamped.
- "Honest concurrency" in miniature: four workers started with `sync.WaitGroup.Go` (Go 1.25) square numbers and send them on a channel, and the results are summed.

## Run

```sh
go run ./examples/001-why-go
go test ./examples/001-why-go
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/app-linux-arm64 ./examples/001-why-go
```

PowerShell form of the cross-compile:

```powershell
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="arm64"; go build -o bin/app-linux-arm64 ./examples/001-why-go
```

## What to look for

Output of `go run` on a Windows/amd64 machine. Sections 1 and 2 depend on the machine and the build; section 3 is always the same:

```text
== 1. One self-describing binary (Deployment Heaven, slide p06) ==
GOOS/GOARCH: windows/amd64
Go version:  go1.26.4
NumCPU:      16
GOMAXPROCS:  16

== 2. Build info embedded in the binary (slide p06) ==
main module:  github.com/NivRave/from-go-build-to-go-run
built with:   go1.26.4
setting:      CGO_ENABLED=0
setting:      GOARCH=amd64
setting:      GOOS=windows
vcs:          not stamped (expected under go run; use go build)

== 3. Honest concurrency in miniature (slide p05) ==
4 workers squared [1 2 3 4 5 6 7 8]; sum of squares = 204
```

- `GOOS/GOARCH` is baked in at build time. Cross-compile with the command above and `go version -m bin/app-linux-arm64` reads the embedded `GOOS=linux` and `GOARCH=arm64` settings without running the binary.
- `GOMAXPROCS` defaults to the number of CPUs, but since Go 1.25 it also respects a container's CPU limit, so inside a CPU-limited container it can be lower than `NumCPU`.
- `go run` doesn't stamp VCS information. Build the binary with `go build` inside the repository and section 2 also prints `vcs`, `vcs.revision`, `vcs.time` and `vcs.modified`.
- The workers finish in any order, but the sum (204) is deterministic, which is why it is the only output asserted by `example_test.go`.

## Notes and corrections

- **Slide p06 "run it anywhere":** a Go binary runs only on the GOOS/GOARCH it was built for. Targeting another platform means cross-compiling, which is a one-liner (see Run). It is fully static only without cgo: build with `CGO_ENABLED=0`, because on Linux packages such as `net` and `os/user` can otherwise link libc dynamically.
- **Slide p04 "almost zero implicit behaviour":** a fair summary, with a footnote. `init()` functions, package initialisation order and method promotion through struct embedding are implicit mechanisms (Part 002 shows the initialisation order).

## Slides

[001-why-go.pdf](../../slides/001-why-go.pdf)
