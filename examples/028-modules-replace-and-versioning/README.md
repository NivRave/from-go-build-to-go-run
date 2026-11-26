# Part 028: Creating modules: replace & versioning

Day 14 · Week 2 · When go get isn't enough.

## What this shows

- A pseudo-version decoder (`pseudo.go`) for all three forms the go command generates: no tag yet, after a release tag, and after a pre-release tag. It prints the base tag, the commit time and the commit hash of each.
- Semantic import versioning in `workspace/`: three nested modules, where `example.com/app` imports `example.com/greeter` (v1, `Hello(name) string`) and `example.com/greeter/v2` (v2, `Hello(name, lang string) (string, error)`) side by side.
- `replace` directives in `workspace/app/go.mod` that point both modules at local directories, made visible through the `Replace` field of `debug.ReadBuildInfo()`.
- Illustrative `retract` lines in `workspace/greeter/go.mod`, with a comment on when retraction actually takes effect.

## Run

```sh
go run ./examples/028-modules-replace-and-versioning
go test ./examples/028-modules-replace-and-versioning

# The nested modules are separate modules, so run them from their own directory:
cd examples/028-modules-replace-and-versioning/workspace/app && go run .
```

The workspace alternative to `replace` (Go 1.18+). `go.work` is personal and is listed in `.gitignore`, so don't commit it:

```sh
cd examples/028-modules-replace-and-versioning/workspace
go work init ./app ./greeter ./greeter/v2
go run ./app
rm go.work          # when you are done
```

## What to look for

Expected output of `go run ./examples/028-modules-replace-and-versioning`:

```text
== 1. Decoding pseudo-versions (slide p07) ==
v0.0.0-20260101120000-abcdefabcdef  (no tag yet)
  base tag: (none)
  time:     2026-01-01T12:00:00Z
  commit:   abcdefabcdef
  sorts:    before every tagged release
v1.2.4-0.20260102093000-0123456789ab  (commit after tag v1.2.3)
  base tag: v1.2.3
  time:     2026-01-02T09:30:00Z
  commit:   0123456789ab
  sorts:    after v1.2.3, before the next release
v1.2.3-pre.0.20260103180000-fedcba987654  (commit after pre-release tag v1.2.3-pre)
  base tag: v1.2.3-pre
  time:     2026-01-03T18:00:00Z
  commit:   fedcba987654
  sorts:    after v1.2.3-pre, before the next release

== 2. Major versions side by side, with replace (slides p02-p03, p06) ==
workspace/ holds three nested modules: example.com/greeter, example.com/greeter/v2 and example.com/app.
Run them from their own directory:
  cd examples/028-modules-replace-and-versioning/workspace/app
  go run .
```

Expected output of `go run .` in `workspace/app`:

```text
== Two major versions in one build (slide p02) ==
v1: Hello, Gopher!
v2: Hola, Gopher!
v2: error: greeter/v2: unsupported language "fr"

== Dependencies and their replacements (slide p03) ==
main module: example.com/app
example.com/greeter v1.0.0 => ../greeter (devel)
example.com/greeter/v2 v2.0.0 => ../greeter/v2 (devel)
```

- The commit after tag v1.2.3 gets the pseudo-version `v1.2.4-0.…`: the patch is incremented and `-0.` makes it a pre-release of v1.2.4, so it sorts after v1.2.3 and before v1.2.4. You can't read the base tag off the pseudo-version without undoing that increment.
- `greeterv1` and `greeterv2` are two different modules in one binary, which is what lets a large codebase migrate from v1 to v2 one package at a time.
- The build information records both the required version (`v1.0.0`) and the directory that replaced it. A directory replacement has no version of its own, so it shows as `(devel)`.
- Under `go.work` the same program prints `example.com/greeter (devel)` with no `=>`: a module that is part of the workspace takes precedence over a `replace` for the same module path. Without either one (`GOWORK=off` and no `replace`), the build fails, because `example.com/greeter` can't be downloaded.

## Notes and corrections

- **Slide p07:** the base version depends on the latest tag before the commit. With no tag it is `v0.0.0-yyyymmddhhmmss-abcdefabcdef` (the form on the slide); after tag `vX.Y.Z` it is `vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdefabcdef`; after pre-release tag `vX.Y.Z-pre` it is `vX.Y.Z-pre.0.yyyymmddhhmmss-abcdefabcdef`.
- **Slide p05:** deleting a tag doesn't help, because the module proxy and the checksum database have already cached the version, so users keep getting it; re-tagging different content makes their builds fail with a checksum mismatch. A retraction takes effect only when you publish a **new** version (for example v1.0.6) whose `go.mod` contains the `retract` lines. The two lines on the slide overlap, since v1.0.1 is inside `[v1.0.0, v1.0.5]`. `go list -m -retracted` lists retracted versions.
- **Slide p03:** `replace` applies only in the main module and is ignored when your module is someone else's dependency, so a `replace` committed in a library silently does nothing for its users. For local multi-module work, `go.work` (slide p06) is usually the better tool; `GOWORK=off` disables it. In a monorepo where `../local-lib` is in the same repository, committing a relative `replace` is reasonable.
- **Slide p04:** `v1.2.4-patch` is a semver pre-release, so it sorts **before** v1.2.4. It works as a pinned `replace` target if the fork has that tag, but a name such as `v1.2.4-myfork.1`, or a pseudo-version, is less surprising.
- **Slide p02:** a module that tagged v2+ before adopting modules appears as `vX.Y.Z+incompatible`. The `/v2` code can live in a `v2/` subdirectory, as it does here, or on a branch whose `go.mod` declares the `/v2` path.
- **Slide p08:** since Go 1.14, when `vendor/` exists and `go.mod` says `go 1.14` or later, `-mod=vendor` is the default. Go 1.22 added `go work vendor` for workspaces.

## Slides

[028-modules-replace-and-versioning.pdf](../../slides/028-modules-replace-and-versioning.pdf)
