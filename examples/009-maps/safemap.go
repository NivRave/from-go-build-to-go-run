package main

import (
	"fmt"
	"sync"
)

// SafeMap is the slide p06 type with two fixes. The mutex has its own field
// name (mu) instead of sharing the receiver's name, so s.mu.RLock() compiles.
// And NewSafeMap makes the inner map, because Set on a nil map panics.
type SafeMap struct {
	mu   sync.RWMutex
	data map[string]int
}

func NewSafeMap() *SafeMap {
	return &SafeMap{data: make(map[string]int)}
}

func (s *SafeMap) Get(key string) int {
	s.mu.RLock() // many readers may hold the read lock at once
	defer s.mu.RUnlock()
	return s.data[key]
}

func (s *SafeMap) Set(key string, val int) {
	s.mu.Lock() // writers get exclusive access
	defer s.mu.Unlock()
	s.data[key] = val
}

// Add is a read-modify-write under one lock. Calling Get and then Set would
// take two separate locks, and two goroutines could both read the same old
// value, losing an update.
func (s *SafeMap) Add(key string, delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] += delta
}

func (s *SafeMap) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// hammer starts the given number of goroutines with sync.WaitGroup.Go (Go
// 1.25). Each writes its own key once and increments the shared "hits" key
// many times, so the final contents are the same on every run.
func hammer(s *SafeMap, goroutines, increments int) {
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			s.Set(fmt.Sprintf("worker-%03d", g), g)
			for range increments {
				s.Add("hits", 1)
				_ = s.Get("hits")
			}
		})
	}
	wg.Wait()
}

// concurrentWritesCrash is the corrected version of the slide p06 crash: the
// goroutines run long enough to overlap, and main waits for them. The runtime
// detects the unsynchronised writes and stops the whole program with
// "fatal error: concurrent map writes". This is not a panic, so recover
// cannot catch it. It only runs with -crash, never from a test.
func concurrentWritesCrash() {
	m := make(map[int]int)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 1_000_000 {
				m[i] = g
			}
		})
	}
	wg.Wait()
	// Detection is best-effort; if it ever gets here, go run -race always
	// reports the race.
	fmt.Println("no crash this time; try: go run -race ./examples/009-maps -crash")
}
