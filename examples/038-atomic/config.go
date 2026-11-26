package main

import (
	"maps"
	"sync/atomic"
)

// Config is an immutable snapshot. Once a *Config has been published through
// ConfigStore, nobody modifies it again; writers change a copy instead
// (copy-on-write, slide p06). That is what makes lock-free reads safe.
type Config struct {
	Version  int
	MaxConns int
	Features map[string]bool
}

// clone makes a deep copy: copying the struct alone would share the
// Features map between the old and the new snapshot.
func (c *Config) clone() *Config {
	next := *c
	next.Features = maps.Clone(c.Features)
	return &next
}

// ConfigStore holds the current Config in an atomic.Pointer (Go 1.19). It is
// type-safe, unlike atomic.Value, which takes any and panics on a nil Store
// or on a Store of a different concrete type.
type ConfigStore struct {
	p atomic.Pointer[Config]
}

func NewConfigStore(initial *Config) *ConfigStore {
	s := &ConfigStore{}
	s.p.Store(initial)
	return s
}

// Load returns the current snapshot. It never blocks, and the result stays
// valid (and unchanged) even if a writer publishes a new version meanwhile.
// Callers must treat it as read-only.
func (s *ConfigStore) Load() *Config {
	return s.p.Load()
}

// Update applies fn to a copy of the current config and publishes it with a
// CompareAndSwap loop. If another writer published first, the swap fails and
// fn is applied again to the newer version, so no update is lost. It returns
// how many times it had to retry.
func (s *ConfigStore) Update(fn func(*Config)) (retries int) {
	for {
		old := s.p.Load()
		next := old.clone()
		fn(next)
		next.Version = old.Version + 1
		if s.p.CompareAndSwap(old, next) {
			return retries
		}
		retries++
	}
}

// UpdateLossy is slide p06's recipe: Load, copy, Store. There is no data
// race (every access is atomic), but with two concurrent writers both can
// copy version N and the second Store silently discards the first's change.
func (s *ConfigStore) UpdateLossy(fn func(*Config)) {
	old := s.p.Load()
	next := old.clone()
	fn(next)
	next.Version = old.Version + 1
	s.p.Store(next)
}
