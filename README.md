# 🚀 From Go Build To Go Run

### A Masterclass for Senior Engineers: Internals, Performance, and Patterns

This repository is the central hub for the **#FromGoBuildToGoRun** series — a high‑signal
resource for engineers who want to move past syntax and master the Go **runtime**,
**memory model**, and **high‑concurrency architecture**.

Each lesson ships as a PDF today. Runnable **code examples** and **exercises** are being
added lesson‑by‑lesson (see [Roadmap for this repo](#-roadmap-for-this-repo)).

---

## 🛠 Project Philosophy

This isn't a "Hello World" repository. Every module targets the complexities of building
production‑grade software in Go.

- **Mechanical Sympathy** — how the code interacts with the CPU and RAM.
- **Decoupling vs. Complexity** — choosing the right abstraction.
- **Production Resilience** — handling failures, leaks, and bottlenecks.

---

## 📦 Repository Structure

Lessons live in numbered, self‑contained folders. Every folder currently holds the lesson
PDF; code examples and exercises are added over time using the layout below.

```
NNN-topic-slug/
├── NNN - topic.pdf     # the lesson (always present)
├── README.md           # lesson notes & how to run   (added with code)
├── main.go             # runnable example            (added with code)
├── examples/           # focused, runnable snippets   (added with code)
└── exercises/          # self-check exercises         (added with code)
    ├── exercise.md
    ├── starter.go
    ├── solution.go
    └── exercise_test.go
```

- **Single Go module** at the repo root (`go.mod`) — everything builds and tests together.
- Update the module path in [`go.mod`](go.mod) to your own repo before pushing.

---

## 🚦 Getting Started

Requires **Go 1.25+**.

```bash
# Clone
git clone <your-repo-url>
cd from-go-build-to-go-run

# Verify the toolchain
go version

# Once code examples exist, from the repo root you can:
go build ./...        # build every lesson
go test ./...         # run every exercise/test
go run ./016-escape-analysis   # run a specific lesson
```

> Reading order follows the numbered folders (`001` → `062`). Two lessons map to each
> day of the roadmap below.

---

## 🗺 The Master Roadmap

> **Note:** Lesson `053` (the first half of *Day 27 — Message Queues*) is not yet in the
> repository, so numbering skips from `052` to `054`.

### Week 1 — The Foundations

| Day | Topic | Lessons |
| --- | --- | --- |
| 01 | Why Go? Philosophy & Structure | [001 · Why Go](001-why-go/001%20-%20why%20go.pdf) · [002 · Anatomy of a Binary](002-anatomy-of-a-binary/002%20-%20anatomy%20of%20a%20binary.pdf) |
| 02 | Variables, Constants & `iota` | [003 · Variables & Constants](003-variables-and-constants/003%20-%20variables%20and%20constants.pdf) · [004 · iota & Enums](004-iota-and-enums/004%20-%20iota%20and%20enums.pdf) |
| 03 | Strings, Runes & Bytes | [005 · Strings, Runes & Bytes](005-strings-runes-and-bytes/005%20-%20strings%20runes%20and%20bytes.pdf) · [006 · Advanced String Tactics](006-advanced-string-tactics/006%20-%20advanced%20string%20tactics.pdf) |
| 04 | Arrays vs. Slices & Internals | [007 · Arrays vs. Slices](007-arrays-vs-slices/007%20-%20arrays%20vs%20slices.pdf) · [008 · Slice Performance & Memory Safety](008-slice-performance-and-memory-safety/008%20-%20slice%20performance%20and%20memory%20safety.pdf) |
| 05 | Maps: Hashing & Iteration | [009 · Maps](009-maps/009%20-%20maps.pdf) · [010 · Inside the Go Map](010-inside-the-go-map/010%20-%20inside%20the%20go%20map.pdf) |
| 06 | Functions & Interface Passing | [011 · Functions & Multiple Returns](011-functions-and-multiple-returns/011%20-%20functions%20and%20multiple%20returns.pdf) · [012 · Interfaces & Decoupling](012-interfaces-and-decoupling/012%20-%20interfaces%20and%20decoupling.pdf) |
| 07 | Struct & Receiver Mechanics | [013 · Structs & Methods](013-structs-and-methods/013%20-%20structs%20and%20methods.pdf) · [014 · Value vs. Pointer Receivers](014-value-vs-pointer-receivers/014%20-%20value%20vs%20pointer%20receivers.pdf) |

### Week 2 — Core Mechanics & Deep Dives

| Day | Topic | Lessons |
| --- | --- | --- |
| 08 | Pointers: Escape Analysis, Heap vs. Stack | [015 · Pointers](015-pointers/015%20-%20pointers.pdf) · [016 · Escape Analysis](016-escape-analysis/016%20-%20escape%20analysis.pdf) |
| 09 | Interfaces: Behavior & Mocking | [017 · Behavior over Inheritance](017-interfaces-behavior-over-inheritance/017%20-%20interfaces%20behavior%20over%20inheritance.pdf) · [018 · Mocking & Design](018-interfaces-mocking-and-design/018%20-%20interfaces%20mocking%20and%20design.pdf) |
| 10 | Iteration: Streams & Generators | [019 · Iteration](019-iteration/019%20-%20iteration.pdf) · [020 · Generators & Streams](020-generators-and-streams/020%20-%20generators%20and%20streams.pdf) |
| 11 | Error Philosophy | [021 · Error Philosophy](021-error-philosophy/021%20-%20error%20philosophy.pdf) · [022 · Error Handling](022-error-handling/022%20-%20error%20handling.pdf) |
| 12 | Testing, Benchmarking & Fuzzing | [023 · Testing the Go Way](023-testing-the-go-way/023%20-%20testing%20the%20go%20way.pdf) · [024 · Advanced Testing](024-advanced-testing/024%20-%20advanced%20testing.pdf) |
| 13 | Defer LIFO & Panic/Recovery | [025 · Mastering Defer](025-mastering-defer/025%20-%20mastering%20defer.pdf) · [026 · Recovery Strategies](026-recovery-strategies/026%20-%20recovery%20strategies.pdf) |
| 14 | Packages & `go mod` | [027 · Packages & go mod](027-packages-and-go-mod/027%20-%20packages%20and%20go%20mod.pdf) · [028 · Modules: Replace & Versioning](028-modules-replace-and-versioning/028%20-%20modules%20replace%20and%20versioning.pdf) |

### Week 3 — Data & Concurrency

| Day | Topic | Lessons |
| --- | --- | --- |
| 15 | JSON: Marshaling & `omitempty` | [029 · JSON](029-json/029%20-%20json.pdf) · [030 · Custom Marshaling & omitempty](030-custom-marshaling-and-omitempty/030%20-%20custom%20marshaling%20and%20omitempty.pdf) |
| 16 | Goroutines & the Scheduler | [031 · Goroutines](031-goroutines/031%20-%20goroutines.pdf) · [032 · Launching Tasks Safely](032-launching-tasks-safely/032%20-%20launching%20tasks%20safely.pdf) |
| 17 | Channels & Producer/Consumer | [033 · Anatomy of Channels](033-anatomy-of-channels/033%20-%20anatomy%20of%20channels.pdf) · [034 · Producer/Consumer](034-producer-consumer/034%20-%20producer%20consumer.pdf) |
| 18 | Select & Timeout Patterns | [035 · Select](035-select/035%20-%20select.pdf) · [036 · Timeout Pattern](036-timeout-pattern/036%20-%20timeout%20pattern.pdf) |
| 19 | Mutex, RWMutex & Atomics | [037 · Mutex vs. RWMutex](037-mutex-vs-rwmutex/037%20-%20mutex%20vs%20rwmutex.pdf) · [038 · Atomic](038-atomic/038%20-%20atomic.pdf) |
| 20 | Context: Cancellation & Values | [039 · Context](039-context/039%20-%20context.pdf) · [040 · Context Propagation](040-context-propagation/040%20-%20context%20propagation.pdf) |
| 21 | Timers, Tickers & Worker Pools | [041 · Timers & Tickers](041-timers-and-tickers/041%20-%20timers%20and%20tickers.pdf) · [042 · Worker Pools](042-worker-pools/042%20-%20worker%20pools.pdf) |

### Week 4 — Networking & Architecture

| Day | Topic | Lessons |
| --- | --- | --- |
| 22 | HTTP Server: Routing & Middleware | [043 · HTTP Server](043-http-server/043%20-%20http%20server.pdf) · [044 · Middleware & Routing](044-middleware-and-routing/044%20-%20middleware%20and%20routing.pdf) |
| 23 | HTTP Client: Pooling & Retries | [045 · HTTP Clients](045-http-clients/045%20-%20http%20clients.pdf) · [046 · Connection Pooling](046-connection-pooling/046%20-%20connection%20pooling.pdf) |
| 24 | Dependency Injection & Clean Architecture | [047 · Dependency Injection](047-dependency-injection/047%20-%20dependency%20injection.pdf) · [048 · Clean Architecture](048-clean-architecture/048%20-%20clean%20architecture.pdf) |
| 25 | Observability: `slog` & Tracing | [049 · slog](049-slog/049%20-%20slog.pdf) · [050 · Distributed Tracing](050-distributed-tracing/050%20-%20distributed%20tracing.pdf) |
| 26 | Graceful Shutdown & Signals | [051 · Graceful Shutdown](051-graceful-shutdown/051%20-%20graceful%20shutdown.pdf) · [052 · Signals & Cleanup](052-signals-and-cleanup/052%20-%20signals%20and%20cleanup.pdf) |
| 27 | Message Queues & Event‑driven | _053 · Message Queues (coming soon)_ · [054 · Event‑driven & Pub/Sub](054-event-driven-and-pub-sub/054%20-%20event%20driven%20and%20pub%20sub.pdf) |
| 28 | SQL: Postgres & Transactions | [055 · Mastering the DB](055-mastering-the-db/055%20-%20mastering%20the%20db.pdf) · [056 · Transactions & Tuning](056-transactions-and-tuning/056%20-%20transactions%20and%20tuning.pdf) |

### Week 5 — Advanced Engineering

| Day | Topic | Lessons |
| --- | --- | --- |
| 29 | Profiling: `pprof` & GC Traces | [057 · pprof](057-pprof/057%20-%20pprof.pdf) · [058 · Flame Graphs & GC Traces](058-flame-graphs-and-gc-traces/058%20-%20flame%20graphs%20and%20gc%20traces.pdf) |
| 30 | Docker & Resilience Patterns | [059 · Docker](059-docker/059%20-%20docker.pdf) · [060 · Resilience Patterns](060-resilience-patterns/060%20-%20resilience%20patterns.pdf) |
| 31 | Memory Model & Path to Pro | [061 · The Go Memory Model](061-the-go-memory-model/061%20-%20the%20go%20memory%20model.pdf) · [062 · Path to Pro](062-path-to-pro/062%20-%20path%20to%20pro.pdf) |

---

## 🧭 Roadmap for this repo

- [x] Reorganize lessons into numbered, self‑contained folders
- [x] Single Go module at the root
- [ ] Add a runnable `main.go` example per lesson
- [ ] Add self‑check exercises (`exercise_test.go`) per lesson
- [ ] CI (build, `vet`, `test`, lint) via GitHub Actions
- [ ] Fill the missing lesson `053`

---

## 🤝 Contributing

Found a bug in an example, or want to sharpen an exercise? Open an issue or PR.
Keep examples idiomatic and runnable, and prefer `go test`‑driven exercises so learners
can self‑verify.
