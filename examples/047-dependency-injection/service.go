package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Store is defined here, by its consumer, and asks only for what Service
// uses (slide p03). MemStore, LoggingStore and the test fakes satisfy it
// implicitly; none of them imports or mentions this interface.
type Store interface {
	Save(ctx context.Context, id string, data []byte) error
	Load(ctx context.Context, id string) ([]byte, error)
}

// Note is what Service stores.
type Note struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// Service has one required dependency (store) and two optional ones (logger
// and clock). Its fields are unexported, so its dependencies can only be set
// through NewService.
type Service struct {
	store  Store
	logger *slog.Logger
	now    func() time.Time
}

// ErrNilStore is returned by NewService when the required store is missing.
var ErrNilStore = errors.New("service: store is required")

// Option configures optional dependencies (slide p05 uses Option without
// showing its definition).
//
// Functional options suit constructors that must validate or that will grow
// optional settings over time; gRPC uses them (grpc.NewServer(opts...)).
// http.Server, which slide p05 also cites, does NOT: it is configured
// through exported struct fields (&http.Server{Addr: ..., ReadTimeout: ...})
// and has no constructor at all. Both are idiomatic; plain struct fields are
// simpler when every zero value is a sensible default.
type Option func(*Service)

// WithLogger sets the logger. The default discards everything.
func WithLogger(l *slog.Logger) Option {
	return func(s *Service) { s.logger = l }
}

// WithClock sets the time source. The default is time.Now; tests pass a
// fixed clock so timestamps are predictable.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService validates its inputs (slide p04). Go can't force callers to use
// a constructor: Service{} and &Service{} are legal in this package. What a
// constructor can do is refuse to build an invalid value, so it returns an
// error instead of a Service that panics later on a nil store.
func NewService(store Store, opts ...Option) (*Service, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	s := &Service{
		store:  store,
		logger: slog.New(slog.DiscardHandler),
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.logger == nil || s.now == nil {
		return nil, errors.New("service: WithLogger and WithClock need non-nil values")
	}
	return s, nil
}

// CreateNote stores a note stamped with the injected clock.
func (s *Service) CreateNote(ctx context.Context, id, text string) (Note, error) {
	n := Note{ID: id, Text: text, CreatedAt: s.now().UTC()}
	data, err := json.Marshal(n)
	if err != nil {
		return Note{}, fmt.Errorf("encode note %s: %w", id, err)
	}
	if err := s.store.Save(ctx, id, data); err != nil {
		return Note{}, fmt.Errorf("save note %s: %w", id, err)
	}
	s.logger.LogAttrs(ctx, slog.LevelInfo, "note created", slog.String("id", id))
	return n, nil
}

// GetNote loads a note. Store errors are wrapped with %w, so callers can
// still test for ErrNotFound with errors.Is.
func (s *Service) GetNote(ctx context.Context, id string) (Note, error) {
	data, err := s.store.Load(ctx, id)
	if err != nil {
		return Note{}, fmt.Errorf("load note %s: %w", id, err)
	}
	var n Note
	if err := json.Unmarshal(data, &n); err != nil {
		return Note{}, fmt.Errorf("decode note %s: %w", id, err)
	}
	return n, nil
}
