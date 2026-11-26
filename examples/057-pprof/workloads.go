package main

import (
	"crypto/sha256"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The three workloads below are deliberately wasteful in three different
// ways, so each one dominates a different profile:
//
//	hashWork  -> CPU profile   (pure computation, slide p04)
//	allocWork -> heap profile  (many short-lived allocations, slide p05)
//	lockWork  -> mutex + block profiles (goroutines waiting on a lock, slide p08)

// hashWork hashes a 4 KiB buffer over and over, feeding each digest back in,
// until d has elapsed. It returns the number of rounds so the compiler can't
// discard the work.
func hashWork(d time.Duration) int {
	buf := make([]byte, 4096)
	rounds := 0
	for deadline := time.Now().Add(d); time.Now().Before(deadline); {
		for range 64 {
			sum := sha256.Sum256(buf)
			copy(buf, sum[:])
			rounds++
		}
	}
	return rounds
}

// report is the payload allocWork marshals. Slices of structs with strings
// are typical of what real services push through encoding/json.
type report struct {
	ID    int      `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	Body  string   `json:"body"`
}

// allocWork builds reports with naive string concatenation and marshals them
// to JSON until d has elapsed. Every iteration allocates and throws away
// several objects, which is "allocation pressure" on slide p05: nothing
// leaks, but the GC has to clean up after every request.
func allocWork(d time.Duration) int {
	total := 0
	for deadline := time.Now().Add(d); time.Now().Before(deadline); {
		total += len(buildReport(total))
	}
	return total
}

func buildReport(id int) []byte {
	body := ""
	for i := range 50 {
		body += "line " + strconv.Itoa(i) + "; " // quadratic copying, on purpose
	}
	r := report{
		ID:    id,
		Title: strings.Repeat("report-", 4) + strconv.Itoa(id),
		Tags:  []string{"pprof", "heap", strconv.Itoa(id % 7)},
		Body:  body,
	}
	b, err := json.Marshal(r)
	if err != nil {
		panic(err) // a struct of strings always marshals
	}
	return b
}

// lockWork starts workers goroutines that all fight over one sync.Mutex and
// hold it while doing a little work, until d has elapsed. The time each
// goroutine spends waiting shows up in the mutex profile (who held the lock)
// and the block profile (who waited for it).
func lockWork(d time.Duration, workers int) int {
	var (
		mu      sync.Mutex
		counter int
		wg      sync.WaitGroup
	)
	deadline := time.Now().Add(d)
	for range workers {
		wg.Go(func() {
			for time.Now().Before(deadline) {
				mu.Lock()
				// A critical section that is far too long: the fix on slide
				// p08 is to shrink it, use an RWMutex, or shard the data.
				for range 2000 {
					counter++
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return counter
}
