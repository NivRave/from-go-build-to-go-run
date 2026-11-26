package main

import "sync"

// Counter is the mutex-protected counter from slides p02, p06 and p07. The
// zero value is ready to use. Always pass it by pointer: copying a Counter
// copies its mutex, and the copy protects nothing (see vetdemo.go).
type Counter struct {
	mu    sync.Mutex
	value int
}

// Inc adds one (slide p02).
func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock() // Always defer immediately
	c.value++
}

// Increment adds one (slide p06). Like Add, it takes the lock once and then
// calls the unexported helper.
func (c *Counter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.increment()
}

// Add adds n. It must not call Increment: Go mutexes are not re-entrant, so
// locking c.mu a second time from the same goroutine blocks forever.
func (c *Counter) Add(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for range n {
		c.increment()
	}
}

// increment does the work and assumes the caller holds c.mu. This is the
// usual Go fix for "I need re-entrant locking": public methods lock, private
// helpers don't.
func (c *Counter) increment() {
	c.value++
}

// GoodPointer reads the value under the lock (slide p07). The pointer
// receiver means every call shares the same mutex.
func (c *Counter) GoodPointer() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// ReentrantCounter is slide p06 as written: Add holds the lock and calls
// Increment, which tries to take it again. Only reachable with -deadlock.
type ReentrantCounter struct {
	mu    sync.Mutex
	value int
}

func (c *ReentrantCounter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *ReentrantCounter) Add(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for range n {
		c.Increment() // DEADLOCK! Increment tries to Lock mu again.
	}
}

// SessionCache is slide p04 plus the constructor it needs: the slide never
// initialises data, so the first Set would panic on a nil map.
type SessionCache struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewSessionCache() *SessionCache {
	return &SessionCache{data: make(map[string]string)}
}

// Get takes the read lock, so any number of readers can run at once. Do not
// call another RLock-ing method from inside it: if a writer is waiting
// between the two RLocks, the second one blocks behind the writer, and the
// writer waits for the first. That is a deadlock too.
func (s *SessionCache) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// Set takes the write lock, which waits for all readers and blocks new ones.
func (s *SessionCache) Set(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}
