package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStore is a test double: a map plus an optional injected failure. Each
// test builds its own, so tests share nothing and can run in parallel. With
// a package-level db variable (slide p06) they would have to take turns.
type fakeStore struct {
	mu      sync.Mutex
	data    map[string][]byte
	saveErr error
	saves   int
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string][]byte{}} }

func (f *fakeStore) Save(_ context.Context, id string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves++
	if f.saveErr != nil {
		return f.saveErr
	}
	f.data[id] = data
	return nil
}

func (f *fakeStore) Load(_ context.Context, id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.data[id]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

var fixedTime = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

func fixedClock() time.Time { return fixedTime }

func TestNewServiceValidates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		store   Store
		opts    []Option
		wantErr bool
	}{
		{"nil store", nil, nil, true},
		{"typed nil store is not caught", (*fakeStore)(nil), nil, false},
		{"nil logger", newFakeStore(), []Option{WithLogger(nil)}, true},
		{"nil clock", newFakeStore(), []Option{WithClock(nil)}, true},
		{"defaults", newFakeStore(), nil, false},
		{"all options", newFakeStore(), []Option{WithLogger(slog.New(slog.DiscardHandler)), WithClock(fixedClock)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, err := NewService(tt.store, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tt.wantErr)
			}
			if (svc == nil) != tt.wantErr {
				t.Errorf("svc = %v, want nil only on error", svc)
			}
		})
	}
	if _, err := NewService(nil); !errors.Is(err, ErrNilStore) {
		t.Errorf("NewService(nil) err = %v, want ErrNilStore", err)
	}
}

func TestCreateNoteUsesInjectedClock(t *testing.T) {
	t.Parallel()
	svc, err := NewService(newFakeStore(), WithClock(fixedClock))
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateNote(t.Context(), "a", "text")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetNote(t.Context(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if !got.CreatedAt.Equal(fixedTime) || !created.CreatedAt.Equal(fixedTime) {
		t.Errorf("CreatedAt = %v / %v, want %v", created.CreatedAt, got.CreatedAt, fixedTime)
	}
	if got.Text != "text" {
		t.Errorf("Text = %q, want text", got.Text)
	}
}

func TestCreateNoteWrapsStoreError(t *testing.T) {
	t.Parallel()
	errDisk := errors.New("disk full")
	store := newFakeStore()
	store.saveErr = errDisk

	svc, err := NewService(store, WithClock(fixedClock))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateNote(t.Context(), "a", "x"); !errors.Is(err, errDisk) {
		t.Errorf("err = %v, want it to wrap errDisk", err)
	}
	if store.saves != 1 {
		t.Errorf("saves = %d, want 1", store.saves)
	}
}

func TestGetNoteNotFound(t *testing.T) {
	t.Parallel()
	svc, err := NewService(newFakeStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetNote(t.Context(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestLoggerIsInjected captures logs in a buffer owned by this test: no
// global logger is touched, so it can run in parallel with everything else.
func TestLoggerIsInjected(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	svc, err := NewService(newFakeStore(), WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateNote(t.Context(), "n9", "x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `msg="note created" id=n9`) {
		t.Errorf("log = %q, want a note created line for n9", buf.String())
	}
}

func TestLoggingStoreDecorates(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	inner := newFakeStore()
	store := NewLoggingStore(inner, slog.New(slog.NewTextHandler(&buf, nil)))
	if err := store.Save(t.Context(), "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if string(inner.data["k"]) != "v" {
		t.Error("decorator did not forward Save to the wrapped store")
	}
	if !strings.Contains(buf.String(), "msg=store.Save id=k bytes=1") {
		t.Errorf("log = %q", buf.String())
	}
}

func Example_decoratorDemo() {
	decoratorDemo()
	// Output:
	// level=INFO msg=store.Save id=greeting bytes=5 err=<nil>
	// level=INFO msg=store.Load id=greeting bytes=5 err=<nil>
	// Load(greeting) = "hello", err = <nil>
	// level=INFO msg=store.Load id=missing bytes=0 err="not found"
	// Load(missing): errors.Is(err, ErrNotFound) = true
}

func Example_constructorDemo() {
	constructorDemo()
	// Output:
	// NewService(nil): err = service: store is required
	// NewService(store, WithClock(nil)): err = service: WithLogger and WithClock need non-nil values
	// NewService(store): ok = true, err = <nil>
}

func Example_run() {
	if err := run(context.Background()); err != nil {
		panic(err)
	}
	// Output:
	// level=INFO msg=store.Save id=n1 bytes=82 err=<nil>
	// level=INFO msg="note created" id=n1
	// level=INFO msg=store.Load id=n1 bytes=82 err=<nil>
	// GetNote(n1) = "wire dependencies in main" created 2026-11-26T09:30:00Z
	// level=INFO msg=store.Load id=n2 bytes=0 err="not found"
	// GetNote(n2): err = load note n2: not found, errors.Is(err, ErrNotFound) = true
}
