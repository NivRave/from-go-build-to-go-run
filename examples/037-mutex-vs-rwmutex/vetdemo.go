//go:build vetdemo

// This file only builds with -tags vetdemo. It holds slide p07's broken
// method so that `go vet -tags vetdemo` can show the copylocks report:
//
//	go vet -tags vetdemo ./examples/037-mutex-vs-rwmutex

package main

// WRONG: Passed by value.
// Every call creates a NEW copy of the mutex. No actual protection!
func (c Counter) BadValue() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}
