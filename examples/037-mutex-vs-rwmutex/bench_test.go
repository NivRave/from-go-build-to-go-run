package main

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
)

// Run with several GOMAXPROCS values to see contention appear:
//
//	go test -run='^$' -bench=. -cpu=1,4,8 ./examples/037-mutex-vs-rwmutex
//
// b.RunParallel runs the body on GOMAXPROCS goroutines at once, so with
// -cpu=1 there is no contention and with -cpu=8 there is a lot.

// (a) A tiny read: return one field. RWMutex does more bookkeeping per call
// (reader count, writer-waiting checks), and with a critical section this
// short that bookkeeping is the whole cost (slide p05).

type mutexField struct {
	mu sync.Mutex
	v  int
}

func (f *mutexField) get() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.v
}

type rwField struct {
	mu sync.RWMutex
	v  int
}

func (f *rwField) get() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.v
}

func BenchmarkTinyRead(b *testing.B) {
	b.Run("Mutex", func(b *testing.B) {
		var f mutexField
		b.RunParallel(func(pb *testing.PB) {
			sum := 0
			for pb.Next() {
				sum += f.get()
			}
			_ = sum
		})
	})
	b.Run("RWMutex", func(b *testing.B) {
		var f rwField
		b.RunParallel(func(pb *testing.PB) {
			sum := 0
			for pb.Next() {
				sum += f.get()
			}
			_ = sum
		})
	})
}

// (b) A slow read (sum 1,000 ints) with 99% reads and 1% writes. Now readers
// hold the lock long enough that letting them overlap pays off.

const sliceLen = 1_000

type mutexSlice struct {
	mu   sync.Mutex
	data []int
}

type rwSlice struct {
	mu   sync.RWMutex
	data []int
}

func sumAll(data []int) int {
	s := 0
	for _, v := range data {
		s += v
	}
	return s
}

func BenchmarkSlowRead99(b *testing.B) {
	b.Run("Mutex", func(b *testing.B) {
		s := &mutexSlice{data: make([]int, sliceLen)}
		b.RunParallel(func(pb *testing.PB) {
			i, sum := 0, 0
			for pb.Next() {
				s.mu.Lock()
				if i%100 == 0 {
					s.data[i%sliceLen]++ // 1% writes
				} else {
					sum += sumAll(s.data) // 99% reads
				}
				s.mu.Unlock()
				i++
			}
			_ = sum
		})
	})
	b.Run("RWMutex", func(b *testing.B) {
		s := &rwSlice{data: make([]int, sliceLen)}
		b.RunParallel(func(pb *testing.PB) {
			i, sum := 0, 0
			for pb.Next() {
				if i%100 == 0 {
					s.mu.Lock()
					s.data[i%sliceLen]++
					s.mu.Unlock()
				} else {
					s.mu.RLock()
					sum += sumAll(s.data)
					s.mu.RUnlock()
				}
				i++
			}
			_ = sum
		})
	})
}

// (c) One map behind one Mutex (the "big lock") vs the 32-shard map vs
// sync.Map, which Go 1.24 rebuilt on a concurrent hash-trie. 90% reads,
// 10% writes over 1,024 keys.

type lockedMap struct {
	mu sync.Mutex
	m  map[string]int
}

func (l *lockedMap) Get(k string) (int, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.m[k]
	return v, ok
}

func (l *lockedMap) Set(k string, v int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.m[k] = v
}

type syncMap struct{ m sync.Map }

func (s *syncMap) Get(k string) (int, bool) {
	v, ok := s.m.Load(k)
	if !ok {
		return 0, false
	}
	return v.(int), true
}

func (s *syncMap) Set(k string, v int) { s.m.Store(k, v) }

type kv interface {
	Get(string) (int, bool)
	Set(string, int)
}

func BenchmarkMap(b *testing.B) {
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%d", i)
	}
	impls := []struct {
		name string
		m    kv
	}{
		{"SingleLock", &lockedMap{m: make(map[string]int)}},
		{"Sharded32", NewShardedMap()},
		{"SyncMap", &syncMap{}},
	}
	for _, impl := range impls {
		for i, k := range keys {
			impl.m.Set(k, i)
		}
		b.Run(impl.name, func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				i := rand.IntN(len(keys)) // each goroutine starts somewhere else
				for pb.Next() {
					k := keys[i%len(keys)]
					if i%10 == 0 {
						impl.m.Set(k, i)
					} else {
						impl.m.Get(k)
					}
					i++
				}
			})
		})
	}
}
