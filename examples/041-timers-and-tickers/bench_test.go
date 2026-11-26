package main

import (
	"testing"
	"time"
)

// A select with a timeout case in a hot loop, two ways. Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/041-timers-and-tickers
//
// Since Go 1.23 neither version leaks: the unfired time.After timers are
// garbage the moment the select returns. What is left is an allocation per
// iteration for time.After versus none for a reused timer.

func BenchmarkTimeAfterInLoop(b *testing.B) {
	b.ReportAllocs()
	ready := make(chan struct{})
	close(ready) // the work case always wins; the timeout never fires
	for b.Loop() {
		select {
		case <-ready:
		case <-time.After(time.Hour):
		}
	}
}

func BenchmarkReusedTimer(b *testing.B) {
	b.ReportAllocs()
	ready := make(chan struct{})
	close(ready)
	timer := time.NewTimer(time.Hour)
	for b.Loop() {
		timer.Reset(time.Hour) // no Stop, no drain (Go 1.23+)
		select {
		case <-ready:
		case <-timer.C:
		}
	}
}
