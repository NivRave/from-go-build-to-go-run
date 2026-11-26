# Part 027: Packages and go mod

Day 14 · Week 2 · Beyond go get.

## What this shows

- A public package (`greet`) next to a private one (`internal/auth`). `main` may import both because it lives in the tree that contains `internal/`; code anywhere else may not.
- What `go.mod` turns into at runtime: `debug.ReadBuildInfo()` reports the main module, its version, the Go toolchain, the dependencies and the build settings.
- A small Minimal Version Selection simulator (`mvs.go`). Given a requirement graph, it selects the highest of the required minimums for each module, so you can watch the algorithm offline.

## Run

```sh
go run ./examples/027-packages-and-go-mod
go test ./examples/027-packages-and-go-mod/...
```

## What to look for

Expected output of `go run` (section 2 depends on how and where you build it):

```text
== 1. Public vs. internal packages (slide p03) ==
Hello, gopher!   admin=true
Hello, guest!    admin=false

== 2. What go.mod produces at runtime (slide p02) ==
main module: github.com/NivRave/from-go-build-to-go-run
version:     (devel)
package:     github.com/NivRave/from-go-build-to-go-run/examples/027-packages-and-go-mod
built with:  go1.26.4
deps:        none (this example imports only the standard library)
setting:     GOARCH=amd64
setting:     GOOS=windows

== 3. Minimal Version Selection (slide p04) ==
serviceA needs lib v1.2.0, serviceB needs lib v1.5.0, lib v1.9.0 exists:
  build list: myapp lib@v1.5.0 log@v1.1.0 serviceA@v1.0.0 serviceB@v1.0.0
  lib: the highest minimum (v1.5.0), not the newest release (v1.9.0)
  log: v1.1.0, because lib v1.2.0 is still in the graph and asks for it
after go get lib@v1.5.2 (myapp now requires it directly):
  build list: myapp lib@v1.5.2 log@v1.1.0 serviceA@v1.0.0 serviceB@v1.0.0
```

- `go run` reports the version as `(devel)`. Build the same program with `go build` inside a git checkout and the version becomes a pseudo-version such as `v0.0.0-20261126063258-2474ca7e2730+dirty`, with `vcs.revision`, `vcs.time` and `vcs.modified` settings next to it (Part 028 decodes pseudo-versions).
- MVS never picks lib v1.9.0, because nothing asks for it. The build only moves to a newer version when a `go.mod` raises a requirement, as `go get lib@v1.5.2` does in the main module.
- `log` lands on v1.1.0 even though the selected lib v1.5.0 needs only v1.0.0: every requirement reachable in the graph counts, including those of versions that lose.

### Importing internal/ from outside the tree

If any package outside `examples/027-packages-and-go-mod/`, even one in this same module, imports `.../027-packages-and-go-mod/internal/auth`, the build fails. This is the real error from Go 1.26 on Windows for a package at `examples/999-outsider` (Linux and macOS print the file path with forward slashes):

```text
package github.com/NivRave/from-go-build-to-go-run/examples/999-outsider
	examples\999-outsider\main.go:3:8: use of internal package github.com/NivRave/from-go-build-to-go-run/examples/027-packages-and-go-mod/internal/auth not allowed
```

### Commands

Run these from the repository root, where `go.mod` lives:

- `go mod tidy -diff`: prints the changes `tidy` would make and exits non-zero if there are any, without touching files. This is the CI check.
- `go mod verify`: checks that the modules in your module cache still match the hashes in `go.sum`.
- `go mod why -m golang.org/x/sync`: shows the shortest import chain from the main module that needs a module.
- `go mod graph`: prints the requirement graph as `module@version requirement@version` pairs, one edge per line.
- `go list -m all`: prints the build list MVS selected, the real version of `mvs.go`'s output.
- `go env -w GOPRIVATE=github.com/my-company`: skips the proxy and the checksum database for your private modules, and persists the setting (on Windows too).

## Notes and corrections

- **Slide p02:** `go.mod` already pins exact versions, because MVS needs no lockfile; `go.sum` verifies their content. The checksum database (`sum.golang.org`, set by `GOSUMDB`) verifies a module the first time anyone downloads it, so even a module that isn't in your `go.sum` yet can't be silently swapped.
- **Slide p04:** MVS picks the highest of the required minimum versions (v1.5 in the slide's example) and never a version newer than anyone asked for. `govulncheck` (`golang.org/x/vuln`) tells you when a bump is needed for security.
- **Slide p05:** since Go 1.23, `go mod tidy -diff` is the CI check: it prints the diff and exits non-zero without modifying files. `go get` and `go mod edit` also change `go.mod`, so `tidy` is not the only tool, just the one that reconciles it with the source.
- **Slide p06:** `GOPRIVATE` also implies `GONOSUMDB`, so private module paths are never sent to the checksum database. Patterns match path prefixes, so `github.com/my-company` is enough, and `go env -w GOPRIVATE=...` works on Windows, where `export` doesn't. Fetching still needs git credentials or a `.netrc`.
- **Slide p03/p07:** this example keeps its public package at `greet/` rather than `pkg/util/`, following slide p07's advice for small modules; the official layout guide (go.dev/doc/modules/layout) doesn't use `/pkg` either.
- **Slide p08:** `go mod graph` prints plain edge pairs, not Graphviz DOT. Convert it first: `go mod graph | modgraphviz | dot -Tsvg -o graph.svg`, with `modgraphviz` from `golang.org/x/exp/cmd/modgraphviz`.
- **Newer `go.mod` directives:** `toolchain` (Go 1.21) names the toolchain to use, `godebug` (Go 1.23) sets default GODEBUG values for the main module, and `tool` (Go 1.24, added with `go get -tool`) records developer tools you run with `go tool`.

## Slides

[027-packages-and-go-mod.pdf](../../slides/027-packages-and-go-mod.pdf)
