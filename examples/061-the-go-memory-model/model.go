package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// The Go memory model answers one question: when is a goroutine guaranteed
// to see a write made by another goroutine? Only when the write
// "happens before" the read. Within one goroutine, program order gives you
// that for free. Between goroutines, only synchronisation does:
//
//   - a send on a channel happens before the matching receive completes
//     (and closing a channel happens before a receive that sees it closed);
//   - Unlock of a sync.Mutex happens before the next Lock returns;
//   - a sync/atomic store happens before an atomic load that observes it;
//   - sync.Once's function returns before any Do returns.
//
// Each function below has one writer goroutine that sets payload and then
// publishes it, and a reader that waits for the publication and then reads
// payload. Because the publication is a synchronising operation, the write to
// payload happens before the read, and the reader always sees 42.

const answer = 42

// publishChannel publishes through an unbuffered channel. The channel carries
// no data here; the send/receive pair alone orders the memory.
func publishChannel() int {
	var payload int
	done := make(chan struct{})
	go func() {
		payload = answer
		done <- struct{}{} // the send happens before the receive completes
	}()
	<-done
	return payload
}

// publishMutex publishes through a flag guarded by a sync.Mutex.
func publishMutex() int {
	var (
		mu      sync.Mutex
		payload int
		ready   bool
	)
	go func() {
		mu.Lock()
		payload = answer
		ready = true
		mu.Unlock() // happens before the reader's next Lock returns
	}()
	for {
		mu.Lock()
		if ready {
			v := payload
			mu.Unlock()
			return v
		}
		mu.Unlock()
		runtime.Gosched()
	}
}

// publishAtomic publishes through an atomic.Bool (typed atomics, Go 1.19).
// payload itself is a plain int: the atomic flag orders it.
func publishAtomic() int {
	var (
		payload int
		ready   atomic.Bool
	)
	go func() {
		payload = answer
		ready.Store(true) // happens before a Load that returns true
	}()
	for !ready.Load() {
		runtime.Gosched()
	}
	return payload
}

func memoryModel() {
	fmt.Println("channel:", publishChannel())
	fmt.Println("mutex:  ", publishMutex())
	fmt.Println("atomic: ", publishAtomic())
}

// The racy version, run only with -race-demo. payload and ready are plain
// variables with no synchronisation, so there is no happens-before edge:
// the reader may see ready == true and still read a stale payload, may never
// see ready change at all (the compiler is allowed to read it once), and the
// program has a data race, which the race detector reports.
var (
	racyPayload int
	racyReady   bool
)

func publishRacy() {
	go func() {
		racyPayload = answer
		racyReady = true // DATA RACE: an ordinary write another goroutine reads
	}()
	// Bounded, so the demo ends even if the read of racyReady is hoisted.
	for i := 0; !racyReady && i < 1_000_000; i++ {
		runtime.Gosched()
	}
	fmt.Println("racy read:", racyPayload, "(0 or 42: without synchronisation either is allowed; run with -race to see the report)")
}
