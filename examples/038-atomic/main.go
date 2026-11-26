// Command atomic demonstrates sync/atomic: a lock-free counter, a
// CompareAndSwap retry loop, why a CAS flag is not once-initialisation, and
// copy-on-write configuration with atomic.Pointer.
//
// Slides: slides/038-atomic.pdf
//
// Run:
//
//	go run ./examples/038-atomic
//	go run -race ./examples/038-atomic -race-demo
//	go test -run='^$' -bench=. -cpu=1,8 ./examples/038-atomic
package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	raceDemo := flag.Bool("race-demo", false, "count with a plain uint64 instead of atomic.Uint64 (run with go run -race)")
	flag.Parse()

	if *raceDemo {
		section("Plain uint64 counter (data race)")
		racyCounter()
		return
	}

	section("1. atomic.Uint64 request counter, 1,000 goroutines (slide p03)")
	atomicCounter()

	section("2. A CompareAndSwap retry loop (slide p04)")
	casLoop()

	section("3. Once-only initialisation (slide p05)")
	casFlagPitfall()
	onceValueFix()

	section("4. Copy-on-write config with atomic.Pointer (slide p06)")
	copyOnWrite()
}

const goroutines = 1_000

// Metrics is slide p03. atomic.Uint64 is a typed wrapper (Go 1.19): it cannot
// be accessed non-atomically by accident, and it is always 8-byte aligned,
// even inside a struct on 32-bit platforms (slide p08).
type Metrics struct {
	requestCount atomic.Uint64
}

func (m *Metrics) Inc() {
	// No locking, no blocking, just a CPU-level increment
	m.requestCount.Add(1)
}

func (m *Metrics) Get() uint64 {
	return m.requestCount.Load()
}

func atomicCounter() {
	var m Metrics
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(m.Inc)
	}
	wg.Wait()
	fmt.Printf("%d goroutines x Inc() = %d\n", goroutines, m.Get())
}

// racyCounter uses a plain uint64. requestCount++ is load, add, store, and
// concurrent increments overwrite each other.
func racyCounter() {
	var requestCount uint64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			requestCount++ // DATA RACE: unsynchronised read-modify-write
		})
	}
	wg.Wait()
	fmt.Printf("%d goroutines x requestCount++ = %d (may be lower; run with -race to see the report)\n",
		goroutines, requestCount)
}

// AtomicUpdate is slide p04 with the variable renamed from new to next: a
// variable called new shadows the builtin, which Go 1.26 also accepts as
// new(expr). It returns how often the swap failed and had to be retried.
func AtomicUpdate(val *atomic.Int32) (retries int) {
	for {
		old := val.Load()
		next := old + 5 // Your complex logic here
		// Atomically attempt the swap
		if val.CompareAndSwap(old, next) {
			// Success! The value hasn't changed since we read it.
			return retries
		}
		// Failure! Someone else modified val: try again with the updated value.
		retries++
	}
}

func casLoop() {
	const workers, perWorker = 8, 1_000
	var val atomic.Int32
	var retries atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				retries.Add(int64(AtomicUpdate(&val)))
			}
		})
	}
	wg.Wait()
	fmt.Printf("%d workers x %d updates of +5: value = %d (want %d)\n",
		workers, perWorker, val.Load(), workers*perWorker*5)
	fmt.Printf("CAS retries under contention: %d (varies; 0 when nothing overlaps)\n", retries.Load())
}

// casFlagPitfall is slide p05. The CAS guarantees the init body runs once,
// but the nine callers that lose the CAS return immediately, while the
// winner is still initialising. Anything they do next sees no config.
func casFlagPitfall() {
	var initialized atomic.Bool
	var ready atomic.Bool // stands in for "the config is loaded"
	var runs atomic.Int32

	Init := func() {
		if initialized.CompareAndSwap(false, true) {
			runs.Add(1)
			time.Sleep(50 * time.Millisecond) // Performing heavy initialization...
			ready.Store(true)
		}
	}

	var early atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			Init()
			if !ready.Load() {
				early.Add(1) // returned from Init, but init is not finished
			}
		})
	}
	wg.Wait()
	fmt.Printf("CAS flag:       init ran %d time; %d of 10 callers returned before it finished\n",
		runs.Load(), early.Load())
}

// onceValueFix uses sync.OnceValue (Go 1.21). The function still runs once,
// and every other caller blocks until it has returned, then gets its result.
func onceValueFix() {
	var runs atomic.Int32
	loadConfig := sync.OnceValue(func() *Config {
		runs.Add(1)
		time.Sleep(50 * time.Millisecond) // Performing heavy initialization...
		return &Config{Version: 1}
	})

	var gotConfig atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if loadConfig() != nil {
				gotConfig.Add(1)
			}
		})
	}
	wg.Wait()
	fmt.Printf("sync.OnceValue: init ran %d time; %d of 10 callers got the finished config\n",
		runs.Load(), gotConfig.Load())
}

// copyOnWrite runs 8 readers and 2 writers (5,000 updates each) against a
// ConfigStore, once with slide p06's Load-copy-Store and once with the CAS
// loop. Readers also check that every snapshot is internally consistent.
func copyOnWrite() {
	const writers, perWriter = 2, 5_000
	for _, v := range []struct {
		name   string
		update func(*ConfigStore, func(*Config))
	}{
		{"Load-copy-Store", (*ConfigStore).UpdateLossy},
		{"CompareAndSwap loop", func(s *ConfigStore, fn func(*Config)) { s.Update(fn) }},
	} {
		final, torn := runConfigWorkload(v.update, writers, perWriter, 8)
		fmt.Printf("%-20s final version %5d of %d writes; inconsistent snapshots seen: %d\n",
			v.name+":", final.Version, writers*perWriter, torn)
	}
	fmt.Println("(Load-copy-Store loses a varying number of updates; the CAS loop never does)")
}

// runConfigWorkload is shared with config_test.go. Every update bumps
// MaxConns by one, and Update/UpdateLossy bump Version by one, so a
// consistent snapshot always has MaxConns == 100 + Version.
func runConfigWorkload(update func(*ConfigStore, func(*Config)), writers, perWriter, readers int) (final *Config, torn int64) {
	store := NewConfigStore(&Config{MaxConns: 100, Features: map[string]bool{"beta": true}})
	bump := func(c *Config) {
		c.MaxConns++
		c.Features["beta"] = c.MaxConns%2 == 0
	}

	var tornCount atomic.Int64
	start := make(chan struct{}) // release all writers at once, so they overlap
	stop := make(chan struct{})
	var readersWG, writersWG sync.WaitGroup
	for range readers {
		readersWG.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				cfg := store.Load()
				if cfg.MaxConns != 100+cfg.Version || cfg.Features["beta"] != (cfg.MaxConns%2 == 0) {
					tornCount.Add(1)
				}
			}
		})
	}
	for range writers {
		writersWG.Go(func() {
			<-start
			for range perWriter {
				update(store, bump)
			}
		})
	}
	close(start)
	writersWG.Wait()
	close(stop)
	readersWG.Wait()
	return store.Load(), tornCount.Load()
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
