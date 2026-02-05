# 🚀 From Go Build To Go Run
### A 31-Day Masterclass for Senior Engineers: Internals, Performance, and Patterns.

This repository is the central hub for the **#FromGoBuildToGoRun** series. It is designed as a high-signal resource for Senior Engineers who want to move past syntax and master the Go runtime, memory model, and high-concurrency architecture.



---

## 🛠 Project Philosophy
This isn't a "Hello World" repository. Every daily module is curated to address the complexities of building production-grade software in Go. 

**The "Senior Take" focus:**
* **Mechanical Sympathy:** How the code interacts with the CPU and RAM.
* **Decoupling vs. Complexity:** Choosing the right abstraction.
* **Production Resilience:** Handling failures, leaks, and bottlenecks.

---

## 🗺 The Master Roadmap

### **Week 1: The Foundations**
* **Day 01:** Why Go? Philosophy & Structure
* **Day 02:** Variables, Constants & `iota` (Type-safe Enums)
* **Day 03:** Memory: Strings, Runes, and Bytes (Iteration Tactics)
* **Day 04:** Arrays vs. Slices & Internal Mechanics (Len/Cap)
* **Day 05:** Maps: O(1) Theory, Hashing & Random Iteration
* **Day 06:** Functions: Multiple Returns & Interface Passing
* **Day 07:** Struct Design & Receiver Mechanics (Value vs. Pointer)

### **Week 2: Core Mechanics & Deep Dives**
* **Day 08:** Pointers: Escape Analysis & Heap vs. Stack
* **Day 09:** Interfaces: Behavior-based Design & Mocking
* **Day 10:** Iteration: Advanced Stream & Generator Patterns
* **Day 11:** Error Philosophy: Wrapping & Custom Types
* **Day 12:** Testing: Table-Driven, Benchmarking & Fuzzing
* **Day 13:** CURRENT: Defer LIFO & Panic/Recovery Middleware
* **Day 14:** Packages & `go mod`: Versioning & Replace

### **Week 3: Data & Concurrency**
* **Day 15:** JSON: Custom Marshaling & omitempty
* **Day 16:** Goroutines: The M:P:G Scheduler Model
* **Day 17:** Channels: Buffering & Producer/Consumer Patterns
* **Day 18:** The Select Statement & Timeout Patterns
* **Day 19:** Mutex vs. RWMutex & Atomic Operations
* **Day 20:** Context: Cancellation Propagation & Values
* **Day 21:** Time: Timers, Tickers, and Worker Pools

### **Week 4: Networking & Architecture**
* **Day 22:** HTTP Server: Routing & Middleware Chains
* **Day 23:** HTTP Client: Connection Pooling & Retries
* **Day 24:** Dependency Injection & Clean Architecture
* **Day 25:** Observability: Structured Logging (`slog`) & Tracing
* **Day 26:** Graceful Shutdown & Signal Handling
* **Day 27:** Message Queues & Event-driven Patterns
* **Day 28:** SQL: Postgres Tuning & Transactions

### **Week 5: Advanced Engineering**
* **Day 29:** Profiling: `pprof`, Flame Graphs, and GC Traces
* **Day 30:** Go-centric Docker & Resilience Patterns
* **Day 31:** Go memory model & The Path to Pro Go Engineer
