# Contributing

Thanks for helping improve **From Go Build To Go Run**. Issues and pull requests are welcome, whether it's a mistake on a slide, a bug in an example, or a clearer way to show an idea.

## Layout

| Path | What lives there |
| --- | --- |
| [`slides/`](slides/) | One PDF per part, named `NNN-kebab-case-title.pdf`. Only PDFs belong here. |
| [`examples/`](examples/) | One runnable example per part, in a directory with the **same name** as its PDF (without `.pdf`). |
| [`internal/repocheck/`](internal/repocheck/) | Tests that enforce the conventions below. They run with `go test ./...`. |
| [`compose.yaml`](compose.yaml) | Postgres for the database examples (055, 056). |

The whole repository is one Go module (`go.mod` at the root). The only exception is `examples/028-modules-replace-and-versioning/workspace/`, which contains nested modules on purpose because that part is about modules.

## Example conventions

Use [`examples/020-generators-and-streams/`](examples/020-generators-and-streams/) as the template.

1. **Every example has a `main.go`** with `package main` and a doc comment that starts with `// Command <slug-without-number> ...` and contains the line `// Slides: slides/<slug>.pdf`.
2. **`go run ./examples/<slug>` must finish on its own** within a few seconds and exit 0. Long-running modes (a server, waiting for Ctrl+C) sit behind a flag such as `-serve` or `-wait`.
3. **Deliberately broken code never runs by default.** Races, deadlocks and crashes that a part demonstrates sit behind an explicit flag (`-race-demo`, `-deadlock`, ...) and are never called from tests. Code that intentionally fails `go vet` lives in a file with a build tag and is documented in the README.
4. **Tests are fast and deterministic.** No network beyond `httptest` on localhost, no sleeps longer than ~100 ms (use `testing/synctest` for time), no dependence on the working directory. Prefer `Example` functions with `// Output:` for anything the README shows as expected output.
5. **Benchmarks use `b.Loop()`** (Go 1.24+) and keep setup under one second and 64 MB.
6. **Standard library first.** The only third-party modules allowed are listed in the README's "Running the examples" section.
7. **Every example has a `README.md`** that starts with `# Part NNN: <Title>` and contains these headings in order: `## What this shows`, `## Run`, `## What to look for`, `## Slides`. An optional `## Notes and corrections` section goes between "What to look for" and "Slides".

## Before you open a pull request

```sh
gofmt -l .                 # must print nothing
go vet ./...
go test -race ./...        # -race needs cgo; on Windows without a C compiler, drop it
go mod tidy -diff          # must print nothing
```

CI runs the same checks on Linux, Windows and macOS, plus `golangci-lint`, a run of every example, the Postgres examples against a real database, and the Docker build for Part 059.

## Licensing

By contributing you agree that code contributions are licensed under the [MIT License](LICENSE) and slide contributions under [CC BY 4.0](LICENSE-SLIDES.md).
