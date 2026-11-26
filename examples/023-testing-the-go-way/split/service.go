package split

import (
	"context"
	"fmt"
)

// Storer is the one behaviour Service needs. Accepting this interface
// instead of a concrete database client is what makes Service testable
// (slide p07).
type Storer interface {
	Load(ctx context.Context, key string) (string, error)
}

// Service splits stored records into fields.
type Service struct {
	store Storer
	sep   string
}

// NewService injects the dependency through the constructor, so a test can
// pass a stub.
func NewService(store Storer) *Service {
	return &Service{store: store, sep: ","}
}

// Fields loads the record stored under key and splits it on commas.
func (s *Service) Fields(ctx context.Context, key string) ([]string, error) {
	record, err := s.store.Load(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("fields %q: %w", key, err)
	}
	return Split(record, s.sep), nil
}
