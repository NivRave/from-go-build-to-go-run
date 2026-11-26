package main

import (
	"testing"
)

// TestConfigStoreNoLostUpdates runs concurrent writers and readers against
// the CompareAndSwap loop. It is meant to run under `go test -race`: the
// race detector checks that copy-on-write really means nobody mutates a
// published snapshot, and the assertions check that no update is lost.
func TestConfigStoreNoLostUpdates(t *testing.T) {
	update := func(s *ConfigStore, fn func(*Config)) { s.Update(fn) }
	tests := []struct {
		name               string
		writers, perWriter int
	}{
		{"one writer", 1, 2_000},
		{"two writers", 2, 2_000},
		{"eight writers", 8, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			final, torn := runConfigWorkload(update, tt.writers, tt.perWriter, 4)
			want := tt.writers * tt.perWriter
			if final.Version != want {
				t.Errorf("final Version = %d, want %d (lost updates)", final.Version, want)
			}
			if final.MaxConns != 100+want {
				t.Errorf("final MaxConns = %d, want %d", final.MaxConns, 100+want)
			}
			if torn != 0 {
				t.Errorf("readers saw %d inconsistent snapshots, want 0", torn)
			}
		})
	}
}

// TestConfigStoreLoadIsStable checks the copy-on-write promise directly: a
// snapshot a reader holds does not change when a writer publishes.
func TestConfigStoreLoadIsStable(t *testing.T) {
	store := NewConfigStore(&Config{MaxConns: 10, Features: map[string]bool{"beta": false}})
	before := store.Load()

	retries := store.Update(func(c *Config) {
		c.MaxConns = 20
		c.Features["beta"] = true
	})

	if retries != 0 {
		t.Errorf("retries = %d with no concurrent writers, want 0", retries)
	}
	if before.MaxConns != 10 || before.Features["beta"] || before.Version != 0 {
		t.Errorf("old snapshot changed: %+v", before)
	}
	after := store.Load()
	if after.MaxConns != 20 || !after.Features["beta"] || after.Version != 1 {
		t.Errorf("new snapshot = %+v, want MaxConns 20, beta true, Version 1", after)
	}
}
