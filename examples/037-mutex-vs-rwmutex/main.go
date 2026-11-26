// Command mutex-vs-rwmutex demonstrates sync.Mutex and sync.RWMutex: a
// counter hammered by 1,000 goroutines, the re-entrancy deadlock and its fix,
// a read-heavy session cache, and a sharded map that splits one big lock
// into 32.
//
// Slides: slides/037-mutex-vs-rwmutex.pdf
//
// Run:
//
//	go run ./examples/037-mutex-vs-rwmutex
//	go run -race ./examples/037-mutex-vs-rwmutex -race-demo
//	go run ./examples/037-mutex-vs-rwmutex -deadlock
//	go vet -tags vetdemo ./examples/037-mutex-vs-rwmutex
//	go test -run='^$' -bench=. -cpu=1,4,8 ./examples/037-mutex-vs-rwmutex
package main

import (
	"flag"
	"fmt"
	"slices"
	"sync"
)

func main() {
	raceDemo := flag.Bool("race-demo", false, "increment a counter without a mutex (run with go run -race)")
	deadlock := flag.Bool("deadlock", false, "lock a mutex twice from the same goroutine (ends in a fatal error)")
	flag.Parse()

	switch {
	case *raceDemo:
		section("Counter without a mutex (data race)")
		racyCounter()
		return
	case *deadlock:
		section("Re-entrant locking (slide p06, as written)")
		reentrantDeadlock()
		return
	}

	section("1. sync.Mutex counter, 1,000 goroutines (slide p02)")
	mutexCounter()

	section("2. No re-entrancy: lock once, call an unexported helper (slide p06)")
	noReentrancy()

	section("3. RWMutex for read-heavy state (slides p03-p04)")
	sessionCache()

	section("4. A sharded map: 32 locks instead of one (slide p08)")
	shardedMap()
}

const goroutines = 1_000

// mutexCounter starts 1,000 goroutines with sync.WaitGroup.Go (Go 1.25),
// each incrementing once. The mutex makes the total exact.
func mutexCounter() {
	var c Counter
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(c.Inc)
	}
	wg.Wait()
	fmt.Printf("%d goroutines x Inc() = %d\n", goroutines, c.GoodPointer())
}

// racyCounter is the same loop without a lock. value++ is a read, an add
// and a write; two goroutines can read the same old value and one update is
// lost. The total is often below 1,000, and `go run -race` reports the race.
func racyCounter() {
	var value int
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			value++ // DATA RACE: unsynchronised read-modify-write
		})
	}
	wg.Wait()
	fmt.Printf("%d goroutines x value++ = %d (may be lower; run with -race to see the report)\n", goroutines, value)
}

// noReentrancy uses the fixed Counter: Add locks once and calls increment().
func noReentrancy() {
	var c Counter
	c.Add(5)
	c.Increment()
	fmt.Println("Add(5) then Increment() =", c.GoodPointer())
}

// reentrantDeadlock calls slide p06's Add. The second Lock waits for the
// first, which is held by the same goroutine, so it waits forever. Because
// nothing else can run, the runtime aborts the program.
func reentrantDeadlock() {
	var c ReentrantCounter
	fmt.Println("calling Add(5), which calls Increment() while holding the lock...")
	c.Add(5)
	fmt.Println("unreachable")
}

// sessionCache runs eight readers and one writer against the RWMutex cache.
// Readers share the lock; the writer waits for them and then has it alone.
func sessionCache() {
	cache := NewSessionCache()
	cache.Set("session-1", "alice")

	var wg sync.WaitGroup
	hits := make([]int, 8) // one slot per reader: no shared counter to race on
	for r := range hits {
		wg.Go(func() {
			for range 1_000 {
				if cache.Get("session-1") != "" {
					hits[r]++
				}
			}
		})
	}
	wg.Go(func() {
		for i := range 10 {
			cache.Set(fmt.Sprintf("session-%d", i+2), "bob")
		}
	})
	wg.Wait()

	total := 0
	for _, h := range hits {
		total += h
	}
	fmt.Printf("8 readers x 1000 Get = %d hits; after 10 Sets, session-11 = %q\n", total, cache.Get("session-11"))
}

// shardedMap writes 8,000 keys from eight goroutines while eight others
// read. Each key lives in one of 32 shards chosen by hash.
func shardedMap() {
	m := NewShardedMap()
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Go(func() {
			for k := range 1_000 {
				m.Set(fmt.Sprintf("w%d-k%d", w, k), k)
			}
		})
		wg.Go(func() {
			for k := range 1_000 {
				m.Get(fmt.Sprintf("w%d-k%d", (w+1)%8, k))
			}
		})
	}
	wg.Wait()

	sizes := m.ShardSizes()
	total := 0
	for _, n := range sizes {
		total += n
	}
	fmt.Printf("%d keys across %d shards; every shard used: %t\n", total, len(sizes), slices.Min(sizes) > 0)
	fmt.Printf("smallest shard %d keys, largest %d (varies: the hash seed is random)\n", slices.Min(sizes), slices.Max(sizes))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
