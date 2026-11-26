# From Go Build To Go Run

[![CI](https://github.com/NivRave/from-go-build-to-go-run/actions/workflows/ci.yml/badge.svg)](https://github.com/NivRave/from-go-build-to-go-run/actions/workflows/ci.yml)

A 31-day deep dive into Go internals, performance and production patterns, written for engineers who already ship Go and want to understand what happens underneath.

**Status: complete.** 31 days, 62 parts. Each part is a short standalone PDF with a runnable, tested Go example. Part 053 is coming soon.

Written by [Niv Rave](https://linkedin.com/in/niv-rave), Software Engineer. Originally published on LinkedIn as #FromGoBuildToGoRun.

## Download

Get the whole series as one searchable PDF from the [latest release](https://github.com/NivRave/from-go-build-to-go-run/releases/latest), or browse the individual parts below.

## How to read it

- Every day has two parts. Part numbers match the file names in [`slides/`](slides/) and the folders in [`examples/`](examples/), so Day 08 is parts `015` and `016`.
- Read it in order if you are moving to Go from another language. If you already work in Go, jump to the week you need.
- Weeks 1 and 2 cover the language and its memory behaviour. Week 3 covers concurrency. Weeks 4 and 5 cover building and running services in production.
- Each example's README lists **notes and corrections**: places where Go has changed since the slides were published, or where a slide simplifies too far.

## Running the examples

You need **Go 1.26 or later**.

```sh
git clone https://github.com/NivRave/from-go-build-to-go-run.git
cd from-go-build-to-go-run
go run ./examples/020-generators-and-streams     # any example
go test ./...                                    # every test in the repository
```

Every example runs on its own in a few seconds. A few need extra tools:

| Examples | Needs |
| --- | --- |
| 055, 056 | Postgres: `docker compose up -d`, then set `DATABASE_URL` as shown in their READMEs |
| 059 | Docker, to build the images |
| 028 | Nothing extra, but it is a multi-module workspace; see its README |

Almost everything uses only the standard library. The exceptions are `golang.org/x/sync` (022, 032), `github.com/jackc/pgx/v5` (055, 056), `github.com/jmoiron/sqlx` (055) and OpenTelemetry (050).

## Project philosophy

- **Mechanical sympathy:** how Go code interacts with the CPU and memory, and what that means for the code you write.
- **Decoupling versus complexity:** when an abstraction pays for itself and when it does not.
- **Production resilience:** the patterns that keep a service running when things go wrong.

## Roadmap

### Week 1: The Foundations

| Day | Topic | Slides | Code |
| --- | --- | --- | --- |
| 01 | Why Go? Philosophy & Structure | [001](slides/001-why-go.pdf) · [002](slides/002-anatomy-of-a-binary.pdf) | [001](examples/001-why-go/) · [002](examples/002-anatomy-of-a-binary/) |
| 02 | Variables, Constants & `iota` (Type-safe Enums) | [003](slides/003-variables-constants.pdf) · [004](slides/004-no-enums-iota.pdf) | [003](examples/003-variables-constants/) · [004](examples/004-no-enums-iota/) |
| 03 | Memory: Strings, Runes, and Bytes (Iteration Tactics) | [005](slides/005-string-rune-and-byte.pdf) · [006](slides/006-advanced-string-tactics.pdf) | [005](examples/005-string-rune-and-byte/) · [006](examples/006-advanced-string-tactics/) |
| 04 | Arrays vs. Slices & Internal Mechanics (Len/Cap) | [007](slides/007-arrays-vs-slices.pdf) · [008](slides/008-slice-performance-and-memory-safety.pdf) | [007](examples/007-arrays-vs-slices/) · [008](examples/008-slice-performance-and-memory-safety/) |
| 05 | Maps: O(1) Theory, Hashing & Random Iteration | [009](slides/009-maps.pdf) · [010](slides/010-inside-the-go-map.pdf) | [009](examples/009-maps/) · [010](examples/010-inside-the-go-map/) |
| 06 | Functions: Multiple Returns & Interface Passing | [011](slides/011-functions-and-multiple-returns.pdf) · [012](slides/012-interfaces-and-decoupling.pdf) | [011](examples/011-functions-and-multiple-returns/) · [012](examples/012-interfaces-and-decoupling/) |
| 07 | Struct Design & Receiver Mechanics (Value vs. Pointer) | [013](slides/013-structs-and-methods.pdf) · [014](slides/014-pointers-vs-value-receivers.pdf) | [013](examples/013-structs-and-methods/) · [014](examples/014-pointers-vs-value-receivers/) |

### Week 2: Core Mechanics & Deep Dives

| Day | Topic | Slides | Code |
| --- | --- | --- | --- |
| 08 | Pointers: Escape Analysis & Heap vs. Stack | [015](slides/015-pointers.pdf) · [016](slides/016-escape-analysis.pdf) | [015](examples/015-pointers/) · [016](examples/016-escape-analysis/) |
| 09 | Interfaces: Behavior-based Design & Mocking | [017](slides/017-interfaces-behavior-over-inheritance.pdf) · [018](slides/018-interfaces-mocking-and-design.pdf) | [017](examples/017-interfaces-behavior-over-inheritance/) · [018](examples/018-interfaces-mocking-and-design/) |
| 10 | Iteration: Advanced Stream & Generator Patterns | [019](slides/019-iteration.pdf) · [020](slides/020-generators-and-streams.pdf) | [019](examples/019-iteration/) · [020](examples/020-generators-and-streams/) |
| 11 | Error Philosophy: Wrapping & Custom Types | [021](slides/021-error-philosophy.pdf) · [022](slides/022-error-handling.pdf) | [021](examples/021-error-philosophy/) · [022](examples/022-error-handling/) |
| 12 | Testing: Table-Driven, Benchmarking & Fuzzing | [023](slides/023-testing-the-go-way.pdf) · [024](slides/024-advanced-testing.pdf) | [023](examples/023-testing-the-go-way/) · [024](examples/024-advanced-testing/) |
| 13 | Defer LIFO & Panic/Recovery Middleware | [025](slides/025-mastering-defer.pdf) · [026](slides/026-recovery-strategies.pdf) | [025](examples/025-mastering-defer/) · [026](examples/026-recovery-strategies/) |
| 14 | Packages & `go mod`: Versioning & Replace | [027](slides/027-packages-and-go-mod.pdf) · [028](slides/028-modules-replace-and-versioning.pdf) | [027](examples/027-packages-and-go-mod/) · [028](examples/028-modules-replace-and-versioning/) |

### Week 3: Data & Concurrency

| Day | Topic | Slides | Code |
| --- | --- | --- | --- |
| 15 | JSON: Custom Marshaling & omitempty | [029](slides/029-json.pdf) · [030](slides/030-custom-marshaling-and-omitempty.pdf) | [029](examples/029-json/) · [030](examples/030-custom-marshaling-and-omitempty/) |
| 16 | Goroutines: The M:P:G Scheduler Model | [031](slides/031-goroutines.pdf) · [032](slides/032-launching-tasks-safely.pdf) | [031](examples/031-goroutines/) · [032](examples/032-launching-tasks-safely/) |
| 17 | Channels: Buffering & Producer/Consumer Patterns | [033](slides/033-anatomy-of-channels.pdf) · [034](slides/034-producer-consumer.pdf) | [033](examples/033-anatomy-of-channels/) · [034](examples/034-producer-consumer/) |
| 18 | The Select Statement & Timeout Patterns | [035](slides/035-select.pdf) · [036](slides/036-timeout-pattern.pdf) | [035](examples/035-select/) · [036](examples/036-timeout-pattern/) |
| 19 | Mutex vs. RWMutex & Atomic Operations | [037](slides/037-mutex-vs-rwmutex.pdf) · [038](slides/038-atomic.pdf) | [037](examples/037-mutex-vs-rwmutex/) · [038](examples/038-atomic/) |
| 20 | Context: Cancellation Propagation & Values | [039](slides/039-context.pdf) · [040](slides/040-context-propagation.pdf) | [039](examples/039-context/) · [040](examples/040-context-propagation/) |
| 21 | Time: Timers, Tickers, and Worker Pools | [041](slides/041-timers-and-tickers.pdf) · [042](slides/042-worker-pools.pdf) | [041](examples/041-timers-and-tickers/) · [042](examples/042-worker-pools/) |

### Week 4: Networking & Architecture

| Day | Topic | Slides | Code |
| --- | --- | --- | --- |
| 22 | HTTP Server: Routing & Middleware Chains | [043](slides/043-http-server.pdf) · [044](slides/044-middleware-and-routing.pdf) | [043](examples/043-http-server/) · [044](examples/044-middleware-and-routing/) |
| 23 | HTTP Client: Connection Pooling & Retries | [045](slides/045-http-clients.pdf) · [046](slides/046-connection-pooling.pdf) | [045](examples/045-http-clients/) · [046](examples/046-connection-pooling/) |
| 24 | Dependency Injection & Clean Architecture | [047](slides/047-dependency-injection.pdf) · [048](slides/048-clean-architecture.pdf) | [047](examples/047-dependency-injection/) · [048](examples/048-clean-architecture/) |
| 25 | Observability: Structured Logging (`slog`) & Tracing | [049](slides/049-slog.pdf) · [050](slides/050-distributed-tracing.pdf) | [049](examples/049-slog/) · [050](examples/050-distributed-tracing/) |
| 26 | Graceful Shutdown & Signal Handling | [051](slides/051-graceful-shutdown.pdf) · [052](slides/052-signals-and-cleanup.pdf) | [051](examples/051-graceful-shutdown/) · [052](examples/052-signals-and-cleanup/) |
| 27 | Message Queues & Event-driven Patterns | 053 *(coming soon)* · [054](slides/054-event-driven-and-pub-sub.pdf) | — · [054](examples/054-event-driven-and-pub-sub/) |
| 28 | SQL: Postgres Tuning & Transactions | [055](slides/055-mastering-the-db.pdf) · [056](slides/056-transactions-and-tuning.pdf) | [055](examples/055-mastering-the-db/) · [056](examples/056-transactions-and-tuning/) |

### Week 5: Advanced Engineering

| Day | Topic | Slides | Code |
| --- | --- | --- | --- |
| 29 | Profiling: `pprof`, Flame Graphs, and GC Traces | [057](slides/057-pprof.pdf) · [058](slides/058-flame-graphs-and-gc-traces.pdf) | [057](examples/057-pprof/) · [058](examples/058-flame-graphs-and-gc-traces/) |
| 30 | Go-centric Docker & Resilience Patterns | [059](slides/059-docker.pdf) · [060](slides/060-resilience-patterns.pdf) | [059](examples/059-docker/) · [060](examples/060-resilience-patterns/) |
| 31 | Go Memory Model & The Path to Pro Go Engineer | [061](slides/061-the-go-memory-model.pdf) · [062](slides/062-path-to-pro.pdf) | [061](examples/061-the-go-memory-model/) · [062](examples/062-path-to-pro/) |

## Repository layout

| Path | Contents |
| --- | --- |
| [`slides/`](slides/) | The 61 slide decks, one PDF per part |
| [`examples/`](examples/) | One runnable example per part, same name as its PDF |
| [`compose.yaml`](compose.yaml) | Postgres for the database examples |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Conventions for examples and how to propose changes |

## License

The slides are licensed under [CC BY 4.0](LICENSE-SLIDES.md): share and adapt them freely with attribution. The code is licensed under the [MIT License](LICENSE).

## Feedback

Found a mistake or want a topic covered in more depth? Open an issue, or message me on [LinkedIn](https://linkedin.com/in/niv-rave).
