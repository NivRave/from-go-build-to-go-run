package main

import (
	"hash/maphash"
	"sync"
)

// shardCount is the number of independent locks (slide p08). Goroutines that
// touch keys in different shards never wait for each other.
const shardCount = 32

// ShardedMap splits one big map into 32 smaller ones, each with its own
// RWMutex, instead of guarding everything with one "big lock".
type ShardedMap struct {
	seed   maphash.Seed
	shards [shardCount]shard
}

type shard struct {
	mu sync.RWMutex
	m  map[string]int
}

func NewShardedMap() *ShardedMap {
	s := &ShardedMap{seed: maphash.MakeSeed()}
	for i := range s.shards {
		s.shards[i].m = make(map[string]int)
	}
	return s
}

// shardFor picks a shard with hash/maphash, the standard library's fast,
// randomly seeded string hash.
func (s *ShardedMap) shardFor(key string) *shard {
	return &s.shards[maphash.String(s.seed, key)%shardCount]
}

func (s *ShardedMap) Get(key string) (int, bool) {
	sh := s.shardFor(key)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	v, ok := sh.m[key]
	return v, ok
}

func (s *ShardedMap) Set(key string, v int) {
	sh := s.shardFor(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.m[key] = v
}

// ShardSizes reports how many keys each shard holds. It locks one shard at
// a time, so under concurrent writes the result is not a single snapshot.
func (s *ShardedMap) ShardSizes() []int {
	sizes := make([]int, shardCount)
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.RLock()
		sizes[i] = len(sh.m)
		sh.mu.RUnlock()
	}
	return sizes
}
