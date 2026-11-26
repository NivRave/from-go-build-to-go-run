package split_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NivRave/from-go-build-to-go-run/examples/023-testing-the-go-way/split"
)

var errNoRecord = errors.New("no such record")

// stubStore is a stub Storer: canned answers, plus a little bookkeeping so a
// test can check how it was used. Injected through split.NewService.
type stubStore struct {
	records map[string]string
	loads   int
}

func (s *stubStore) Load(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.loads++
	v, ok := s.records[key]
	if !ok {
		return "", errNoRecord
	}
	return v, nil
}

// newStubStore builds a stub for one test and registers its teardown with
// t.Cleanup, which runs when the test (and its subtests) finish, even if
// the test fails. Nothing leaks into the next test (slide p05).
func newStubStore(t *testing.T) *stubStore {
	t.Helper()
	s := &stubStore{records: copyFixtures()}
	t.Cleanup(func() {
		t.Logf("cleanup: stub served %d loads", s.loads)
		clear(s.records)
	})
	return s
}

func TestServiceFields(t *testing.T) {
	t.Parallel()
	store := newStubStore(t)
	svc := split.NewService(store)

	// t.Context() (Go 1.24) is cancelled just before Cleanup functions run,
	// so work started by the test can't outlive it.
	got, err := svc.Fields(t.Context(), "user:2")
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	assertEqual(t, got, []string{"Alan", "Turing", "1912"})
	if store.loads != 1 {
		t.Errorf("store.Load called %d times, want 1", store.loads)
	}
}

func TestServiceFieldsMissingRecord(t *testing.T) {
	t.Parallel()
	svc := split.NewService(newStubStore(t))

	_, err := svc.Fields(t.Context(), "user:404")
	if !errors.Is(err, errNoRecord) {
		t.Fatalf("err = %v, want errors.Is(err, errNoRecord)", err)
	}
}

func TestServiceFieldsCancelledContext(t *testing.T) {
	t.Parallel()
	store := newStubStore(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := split.NewService(store).Fields(ctx, "user:1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if store.loads != 0 {
		t.Errorf("store.Load did work for a cancelled context")
	}
}
