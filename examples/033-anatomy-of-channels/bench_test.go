package main

import (
	"sync"
	"testing"
)

// Move one million ints from a producer goroutine to a consumer three ways
// (slide p08). Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/033-anatomy-of-channels
//
// Sending one value at a time pays the channel's synchronisation cost a
// million times; batches of 1,000 pay it a thousand times.
const (
	total     = 1_000_000
	batchSize = 1_000
	chanBuf   = 64
)

func BenchmarkChannelOneByOne(b *testing.B) {
	for b.Loop() {
		ch := make(chan int, chanBuf)
		go func() {
			defer close(ch)
			for i := range total {
				ch <- i
			}
		}()
		sum := 0
		for v := range ch {
			sum += v
		}
		_ = sum
	}
}

func BenchmarkChannelBatched(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		ch := make(chan []int, chanBuf)
		go func() {
			defer close(ch)
			for start := 0; start < total; start += batchSize {
				batch := make([]int, batchSize) // ownership passes to the receiver
				for j := range batch {
					batch[j] = start + j
				}
				ch <- batch
			}
		}()
		sum := 0
		for batch := range ch {
			for _, v := range batch {
				sum += v
			}
		}
		_ = sum
	}
}

func BenchmarkMutexSlice(b *testing.B) {
	b.ReportAllocs()
	shared := make([]int, 0, total)
	for b.Loop() {
		var mu sync.Mutex
		shared = shared[:0]
		var wg sync.WaitGroup
		wg.Go(func() {
			for i := range total {
				mu.Lock()
				shared = append(shared, i)
				mu.Unlock()
			}
		})
		wg.Wait()
		sum := 0
		for _, v := range shared {
			sum += v
		}
		_ = sum
	}
}
