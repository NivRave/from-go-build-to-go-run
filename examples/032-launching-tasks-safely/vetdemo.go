//go:build vetdemo

package main

// This file is compiled only with -tags vetdemo. It contains the bug from
// slide p04, which Go 1.25's waitgroup vet analyzer reports:
//
//	go vet -tags vetdemo ./examples/032-launching-tasks-safely
//
// wg.Add(1) runs inside the new goroutine, so the parent can reach
// wg.Wait() while the counter is still 0 and return before any work starts.

import (
	"fmt"
	"sync"
)

func addInsideGoroutine() {
	var wg sync.WaitGroup
	for i := range 3 {
		go func() {
			wg.Add(1) // too late: should run in the parent, before go
			defer wg.Done()
			fmt.Println("processing", i)
		}()
	}
	wg.Wait()
}
