# Part 059: Go-centric Docker

Day 30 · Week 5 · From 800MB to 15MB: Mastering the ultra-slim production image.

## What this shows

- A service that prints one JSON object describing what it finds at run time: Go version, `GOMAXPROCS`, `NumCPU`, `GOMEMLIMIT`, whether `time.LoadLocation("Europe/Berlin")` works, and whether the system certificate pool has any roots. CI runs it inside both images.
- `-serve`: `GET /healthz` and `GET /info` on `:8080`, with graceful shutdown on SIGTERM (what `docker stop` sends).
- `-https URL`: one outbound GET that proves the image's CA certificates work.
- `Dockerfile`: a multi-stage build on `golang:1.26-alpine` with BuildKit cache mounts, `CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`, and a `gcr.io/distroless/static-debian12:nonroot` runner.
- `Dockerfile.scratch`: the same binary on `scratch`, with the gaps filled: a copied CA bundle, the time zone database embedded with `-tags timetzdata`, and a numeric `USER 65532:65532`.

## Run

```sh
go run ./examples/059-docker
go test ./examples/059-docker
go run ./examples/059-docker -serve                       # Ctrl+C to stop
go run ./examples/059-docker -https https://go.dev        # makes a real network request
```

Both Dockerfiles build from the **repository root**, so `go.mod` and `go.sum` are in the build context (the root `.dockerignore` keeps `.git/` and the ~170 MB of slides out of it):

```sh
docker build -f examples/059-docker/Dockerfile -t gosvc:distroless .
docker build -f examples/059-docker/Dockerfile.scratch -t gosvc:scratch .
docker run --rm gosvc:distroless
docker run --rm gosvc:scratch
docker run --rm --cpus=2 gosvc:distroless
docker run --rm --cpus=2 -m 256m -e GOMEMLIMIT=230MiB gosvc:scratch
docker run --rm -p 127.0.0.1:8080:8080 gosvc:distroless -serve
docker images gosvc
```

Binary size with and without stripping (a Linux build, as in the image):

```sh
GOOS=linux CGO_ENABLED=0 go build -o bin/full ./examples/059-docker
GOOS=linux CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/stripped ./examples/059-docker
ls -l bin/full bin/stripped
```

PowerShell: `$env:GOOS="linux"; $env:CGO_ENABLED="0"; go build -o bin/full ./examples/059-docker` (then `Remove-Item Env:GOOS, Env:CGO_ENABLED`).

## What to look for

Expected output of `go run` (the values describe your machine; on Windows and macOS the system certificate pool is the OS verifier, so it reports `true`):

```text
{"go_version":"go1.26.4","gomaxprocs":16,"num_cpu":16,"gomemlimit":"unset","tz_europe_berlin":"ok: Europe/Berlin","system_ca_certs":true}
```

The images, from a real run on Docker Desktop (the image's Go is whatever patch release `golang:1.26-alpine` points at):

```text
$ docker run --rm gosvc:distroless
{"go_version":"go1.26.11","gomaxprocs":16,"num_cpu":16,"gomemlimit":"unset","tz_europe_berlin":"ok: Europe/Berlin","system_ca_certs":true}
$ docker run --rm gosvc:scratch
{"go_version":"go1.26.11","gomaxprocs":16,"num_cpu":16,"gomemlimit":"unset","tz_europe_berlin":"ok: Europe/Berlin","system_ca_certs":true}
$ docker run --rm --cpus=2 gosvc:distroless
{"go_version":"go1.26.11","gomaxprocs":2,"num_cpu":16,"gomemlimit":"unset","tz_europe_berlin":"ok: Europe/Berlin","system_ca_certs":true}
$ docker run --rm --cpus=2 -m 256m -e GOMEMLIMIT=230MiB gosvc:scratch
{"go_version":"go1.26.11","gomaxprocs":2,"num_cpu":16,"gomemlimit":"230 MiB","tz_europe_berlin":"ok: Europe/Berlin","system_ca_certs":true}
$ docker images gosvc
IMAGE              ID             DISK USAGE   CONTENT SIZE   EXTRA
gosvc:distroless   e1843b9dc373       15.5MB         3.51MB
gosvc:scratch      98072b0b2d24       10.2MB            3MB
```

```text
$ ls -l bin/full bin/stripped          # linux/amd64, built with Go 1.26.4
-rw-r--r-- 1 nivik 197609 9443582 Nov 26 08:47 bin/full
-rw-r--r-- 1 nivik 197609 6545570 Nov 26 08:47 bin/stripped
$ docker run --rm -m 256m gosvc:distroless
{"go_version":"go1.26.11","gomaxprocs":16,"num_cpu":16,"gomemlimit":"unset","tz_europe_berlin":"ok: Europe/Berlin","system_ca_certs":true}
```

- **`--cpus=2` gives `gomaxprocs: 2`** while `num_cpu` stays 16: since Go 1.25 the runtime reads the cgroup CPU limit and sets `GOMAXPROCS` from it, so `uber-go/automaxprocs` is no longer needed.
- **`GOMEMLIMIT` stays `unset`** even under `-m 256m`. Go does not read the memory limit; set `GOMEMLIMIT` to about 90% of it yourself, as in the last run.
- **Both images are about 10-15 MB** on disk, around 3 MB compressed. The scratch image is smaller by distroless's base layer; its binary is about 400 KB larger because of the embedded time zone database.
- **`-s -w` removes about 30%** of the binary (9.4 MB to 6.5 MB here). `-ldflags="-s"` alone gives the identical size, because since Go 1.22 `-s` implies `-w`.

### What breaks on bare scratch

The same binary on `FROM scratch` with no CA bundle and no `timetzdata` tag (a scratch-pad Dockerfile, not in this repo) prints:

```text
{"go_version":"go1.26.11","gomaxprocs":16,"num_cpu":16,"gomemlimit":"unset","tz_europe_berlin":"error: unknown time zone Europe/Berlin","system_ca_certs":false}
$ docker run --rm gosvc-bare -https https://go.dev
GET failed: Get "https://go.dev": tls: failed to verify certificate: x509: certificate signed by unknown authority
```

`Dockerfile.scratch` fixes both, and the fixed image prints `GET https://go.dev: 200 OK (...); TLS verification succeeded`. Without `/etc/passwd`, `USER nonroot` can't be resolved, so it uses the numeric `USER 65532:65532`.

### Next hardening step: pin base images by digest

Tags move: `golang:1.26-alpine` and `static-debian12:nonroot` are rebuilt for every patch and CVE fix. For reproducible builds, pin both by digest and let a bot (Dependabot, Renovate) propose updates:

```dockerfile
FROM golang:1.26-alpine@sha256:<digest> AS build
FROM gcr.io/distroless/static-debian12:nonroot@sha256:<digest>
```

Find the current digest with `docker buildx imagetools inspect golang:1.26-alpine`.

## Notes and corrections

- **Slide p04:**
  - `golang:1.23-alpine` is out of support (Go 1.23 left support when Go 1.25 was released in August 2025); this example uses `golang:1.26-alpine`, which matches `go.mod`. An older image would have to download a newer toolchain, or fail offline.
  - The Dockerfile adds BuildKit cache mounts for the module and build caches, `-trimpath` (no local paths in the binary), and relies on the repository's `.dockerignore` because of `COPY . .`.
  - `USER nonroot:nonroot` is redundant with the `:nonroot` tag, which already runs as UID 65532. `GOOS=linux` is redundant inside a Linux builder.
- **Since Go 1.25:** `GOMAXPROCS` follows the container's CPU limit, as the `--cpus=2` run shows. `GOMEMLIMIT` is not set automatically, so set it to about 90% of the memory limit.
- **Slide p02:** pure-Go binaries are statically linked by default; cgo is what brings in libc. Since Go 1.20, `CGO_ENABLED` defaults to 0 when no C compiler is found, as in `golang:alpine`. Setting it explicitly is still good practice.
- **Slide p05:** scratch's gaps each have a fix: copy a CA bundle (or import `golang.org/x/crypto/x509roots/fallback`), embed time zones with `-tags timetzdata` or `import _ "time/tzdata"`, and use a numeric `USER`. Distroless remains the simpler default.
- **Slide p06:** `-s` removes the ELF symbol table, not the information stack traces use. Panics still show function names and line numbers, because those come from Go's own pclntab. Since Go 1.22, `-s` implies `-w`.
- **Slide p08:** `kubectl debug` adds an ephemeral container to the running pod (GA since Kubernetes 1.25), not a sidecar. Distroless also publishes `:debug` tags that include a busybox shell.

## Slides

[059-docker.pdf](../../slides/059-docker.pdf)
