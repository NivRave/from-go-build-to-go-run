# From Go Build To Go Run

A 31-day deep dive into Go internals, performance and production patterns, written for engineers who already ship Go and want to understand what happens underneath.

**Status: complete.** 31 days, 62 parts, each part a short standalone PDF.

Written by [Niv Rave](https://linkedin.com/in/niv-rave), Software Engineer. Originally published on LinkedIn as #FromGoBuildToGoRun.

## How to read it

- Every day has two parts. Part numbers match the file names in this repository, so Day 08 is files `015` and `016`.
- Read it in order if you are moving to Go from another language. If you already work in Go, jump to the week you need.
- Weeks 1 and 2 cover the language and its memory behaviour. Week 3 covers concurrency. Weeks 4 and 5 cover building and running services in production.

## Project philosophy

- **Mechanical sympathy:** how Go code interacts with the CPU and memory, and what that means for the code you write.
- **Decoupling versus complexity:** when an abstraction pays for itself and when it does not.
- **Production resilience:** the patterns that keep a service running when things go wrong.

## Roadmap

### Week 1: The Foundations

| Day | Topic | Parts |
| --- | --- | --- |
| 01 | Why Go? Philosophy & Structure | 001, 002 |
| 02 | Variables, Constants & `iota` (Type-safe Enums) | 003, 004 |
| 03 | Memory: Strings, Runes, and Bytes (Iteration Tactics) | 005, 006 |
| 04 | Arrays vs. Slices & Internal Mechanics (Len/Cap) | 007, 008 |
| 05 | Maps: O(1) Theory, Hashing & Random Iteration | 009, 010 |
| 06 | Functions: Multiple Returns & Interface Passing | 011, 012 |
| 07 | Struct Design & Receiver Mechanics (Value vs. Pointer) | 013, 014 |

### Week 2: Core Mechanics & Deep Dives

| Day | Topic | Parts |
| --- | --- | --- |
| 08 | Pointers: Escape Analysis & Heap vs. Stack | 015, 016 |
| 09 | Interfaces: Behavior-based Design & Mocking | 017, 018 |
| 10 | Iteration: Advanced Stream & Generator Patterns | 019, 020 |
| 11 | Error Philosophy: Wrapping & Custom Types | 021, 022 |
| 12 | Testing: Table-Driven, Benchmarking & Fuzzing | 023, 024 |
| 13 | Defer LIFO & Panic/Recovery Middleware | 025, 026 |
| 14 | Packages & `go mod`: Versioning & Replace | 027, 028 |

### Week 3: Data & Concurrency

| Day | Topic | Parts |
| --- | --- | --- |
| 15 | JSON: Custom Marshaling & omitempty | 029, 030 |
| 16 | Goroutines: The M:P:G Scheduler Model | 031, 032 |
| 17 | Channels: Buffering & Producer/Consumer Patterns | 033, 034 |
| 18 | The Select Statement & Timeout Patterns | 035, 036 |
| 19 | Mutex vs. RWMutex & Atomic Operations | 037, 038 |
| 20 | Context: Cancellation Propagation & Values | 039, 040 |
| 21 | Time: Timers, Tickers, and Worker Pools | 041, 042 |

### Week 4: Networking & Architecture

| Day | Topic | Parts |
| --- | --- | --- |
| 22 | HTTP Server: Routing & Middleware Chains | 043, 044 |
| 23 | HTTP Client: Connection Pooling & Retries | 045, 046 |
| 24 | Dependency Injection & Clean Architecture | 047, 048 |
| 25 | Observability: Structured Logging (`slog`) & Tracing | 049, 050 |
| 26 | Graceful Shutdown & Signal Handling | 051, 052 |
| 27 | Message Queues & Event-driven Patterns | 053, 054 |
| 28 | SQL: Postgres Tuning & Transactions | 055, 056 |

### Week 5: Advanced Engineering

| Day | Topic | Parts |
| --- | --- | --- |
| 29 | Profiling: `pprof`, Flame Graphs, and GC Traces | 057, 058 |
| 30 | Go-centric Docker & Resilience Patterns | 059, 060 |
| 31 | Go Memory Model & The Path to Pro Go Engineer | 061, 062 |

## Feedback

Found a mistake or want a topic covered in more depth? Open an issue, or message me on [LinkedIn](https://linkedin.com/in/niv-rave).
