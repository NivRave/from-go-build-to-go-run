package main

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
)

// ErrNotFound is returned by stores when an id has no record.
var ErrNotFound = errors.New("not found")

// MemStore is an in-memory Store. It knows nothing about Service: it just
// happens to have the methods Service's Store interface asks for.
type MemStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemStore returns an empty store. A constructor is needed here because
// the zero MemStore has a nil map, and writing to it would panic.
func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string][]byte)}
}

func (m *MemStore) Save(_ context.Context, id string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = slices.Clone(data) // don't keep the caller's buffer
	return nil
}

func (m *MemStore) Load(_ context.Context, id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.data[id]
	if !ok {
		return nil, ErrNotFound
	}
	return slices.Clone(data), nil
}

// LoggingStore is a decorator: it satisfies Store by wrapping another Store
// and logging every call. Because Service depends on the interface, adding
// logging needs no change to Service or MemStore, only to the wiring.
type LoggingStore struct {
	next   Store
	logger *slog.Logger
}

func NewLoggingStore(next Store, logger *slog.Logger) *LoggingStore {
	return &LoggingStore{next: next, logger: logger}
}

func (s *LoggingStore) Save(ctx context.Context, id string, data []byte) error {
	err := s.next.Save(ctx, id, data)
	s.logger.LogAttrs(ctx, slog.LevelInfo, "store.Save",
		slog.String("id", id), slog.Int("bytes", len(data)), slog.Any("err", err))
	return err
}

func (s *LoggingStore) Load(ctx context.Context, id string) ([]byte, error) {
	data, err := s.next.Load(ctx, id)
	s.logger.LogAttrs(ctx, slog.LevelInfo, "store.Load",
		slog.String("id", id), slog.Int("bytes", len(data)), slog.Any("err", err))
	return data, err
}
