//go:build vetdemo

// This file is excluded from normal builds. It exists to show what go vet's
// copylocks check reports when a struct containing a sync.Mutex is copied:
//
//	go vet -tags vetdemo ./examples/013-structs-and-methods
//
// Each copy gets its own mutex, so locking the copy does not protect the
// original, which is why Config is only ever used through a pointer.

package main

import "fmt"

// pathCount takes Config by value, so every call copies the lock.
func pathCount(c Config) int {
	return len(c.Paths)
}

func copyConfig() {
	var c Config
	c.AddPath("/etc/app")

	snapshot := c // copies c.mu
	fmt.Println(pathCount(snapshot), len(snapshot.Paths))
}
