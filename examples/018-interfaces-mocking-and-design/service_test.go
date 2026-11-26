package main

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// ---- Hand-written test doubles (slides p04, p05, p07) ------------------------

// MockStore records every call so a test can assert on the interaction. It is
// a plain struct: no mocking library, no code generation. It is not safe for
// concurrent use, which is fine because UserService calls it sequentially.
type MockStore struct {
	Calls   []string // "SaveUser u1", "GetUser u1", ...
	Saved   []User
	SaveErr error // returned by SaveUser when non-nil
}

func (m *MockStore) SaveUser(_ context.Context, u User) error {
	m.Calls = append(m.Calls, "SaveUser "+u.ID)
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.Saved = append(m.Saved, u)
	return nil
}

func (m *MockStore) GetUser(_ context.Context, id string) (User, error) {
	m.Calls = append(m.Calls, "GetUser "+id)
	for _, u := range m.Saved {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (m *MockStore) DeleteUser(_ context.Context, id string) error {
	m.Calls = append(m.Calls, "DeleteUser "+id)
	return nil
}

// ErrConnLost is a sentinel, so tests assert with errors.Is instead of
// comparing error strings (slide p05).
var ErrConnLost = errors.New("database connection lost")

// FailStore simulates the error path that is hard to provoke with a real
// database. Every method fails.
type FailStore struct{}

func (FailStore) SaveUser(context.Context, User) error          { return ErrConnLost }
func (FailStore) GetUser(context.Context, string) (User, error) { return User{}, ErrConnLost }
func (FailStore) DeleteUser(context.Context, string) error      { return ErrConnLost }

// getOnlyStore is a partial mock (slide p07): it embeds the DataStore
// interface, which satisfies the whole interface, and overrides one method.
// The embedded field is nil, so calling any method that is NOT overridden
// dereferences a nil interface and panics.
type getOnlyStore struct {
	DataStore // nil: SaveUser and DeleteUser are "promoted" from nothing
	user      User
}

func (s getOnlyStore) GetUser(context.Context, string) (User, error) { return s.user, nil }

// ---- Tests -------------------------------------------------------------------

// Pure logic needs no mock at all (slide p06): call it with concrete values.
func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{"valid", User{ID: "u1", Name: "Ada", Email: "ada@example.com"}, false},
		{"empty ID", User{Name: "Ada", Email: "ada@example.com"}, true},
		{"blank name", User{ID: "u1", Name: "  ", Email: "ada@example.com"}, true},
		{"email without @", User{ID: "u1", Name: "Ada", Email: "ada.example.com"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tt.user)
			if got := errors.Is(err, ErrInvalidUser); got != tt.wantErr {
				t.Errorf("Validate(%+v) = %v; errors.Is(err, ErrInvalidUser) = %v, want %v",
					tt.user, err, got, tt.wantErr)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()
	valid := User{ID: "u1", Name: "Ada", Email: "ada@example.com"}
	tests := []struct {
		name      string
		store     DataStore
		user      User
		wantErr   error    // matched with errors.Is; nil means success
		wantCalls []string // only checked when store is a *MockStore
	}{
		{"saves a valid user", &MockStore{}, valid, nil, []string{"SaveUser u1"}},
		{"rejects invalid input before touching the store", &MockStore{}, User{ID: "u1"}, ErrInvalidUser, nil},
		{"wraps store errors", FailStore{}, valid, ErrConnLost, nil},
		{"wraps errors from a configured mock", &MockStore{SaveErr: ErrConnLost}, valid, ErrConnLost, []string{"SaveUser u1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewUserService(tt.store).Register(t.Context(), tt.user)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Register() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if m, ok := tt.store.(*MockStore); ok {
				if got := strings.Join(m.Calls, ", "); got != strings.Join(tt.wantCalls, ", ") {
					t.Errorf("store calls = [%s], want [%s]", got, strings.Join(tt.wantCalls, ", "))
				}
			}
		})
	}
}

func TestRegisterWrapsWithContext(t *testing.T) {
	t.Parallel()
	err := NewUserService(FailStore{}).Register(t.Context(), User{ID: "u9", Name: "X", Email: "x@example.com"})
	// errors.Is finds the cause; the message still says where it happened.
	if !errors.Is(err, ErrConnLost) {
		t.Fatalf("err = %v, want ErrConnLost in the chain", err)
	}
	if want := "register u9: database connection lost"; err.Error() != want {
		t.Errorf("err.Error() = %q, want %q", err, want)
	}
}

func TestPartialMockOverriddenMethod(t *testing.T) {
	t.Parallel()
	want := User{ID: "u1", Name: "Ada", Email: "ada@example.com"}
	svc := NewUserService(getOnlyStore{user: want})
	got, err := svc.Lookup(t.Context(), "u1")
	if err != nil || got != want {
		t.Fatalf("Lookup() = %+v, %v; want %+v, nil", got, err, want)
	}
}

func TestPartialMockPanicsOnMissingMethod(t *testing.T) {
	t.Parallel()
	svc := NewUserService(getOnlyStore{})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Unregister did not panic; expected a nil pointer dereference")
		}
		rerr, ok := r.(runtime.Error)
		if !ok || !strings.Contains(rerr.Error(), "nil pointer dereference") {
			t.Fatalf("recovered %v (%T), want a runtime nil pointer dereference", r, r)
		}
		t.Logf("calling a non-overridden method panics: %v", rerr)
	}()

	// DeleteUser is not overridden, so it is the nil embedded DataStore's.
	_ = svc.Unregister(t.Context(), "u1")
}

// flakyStore fails the first `failures` saves, then succeeds.
type flakyStore struct {
	MockStore
	failures int
	attempts int
}

func (f *flakyStore) SaveUser(ctx context.Context, u User) error {
	f.attempts++
	if f.attempts <= f.failures {
		return ErrConnLost
	}
	return f.MockStore.SaveUser(ctx, u)
}

// RetryRegister sleeps between attempts. Inside synctest.Test the bubble has
// a fake clock: time.Sleep returns as soon as every goroutine in the bubble
// is blocked, and time.Now advances by exactly the slept duration. The test
// takes microseconds of real time and asserts exact virtual durations.
func TestRetryRegister(t *testing.T) {
	t.Parallel()
	valid := User{ID: "u1", Name: "Ada", Email: "ada@example.com"}
	tests := []struct {
		name         string
		failures     int
		user         User
		wantErr      error
		wantAttempts int
		wantElapsed  time.Duration
	}{
		{"first try", 0, valid, nil, 1, 0},
		{"succeeds on third try", 2, valid, nil, 3, 2 * retryBackoff},
		{"gives up after maxAttempts", 5, valid, ErrConnLost, maxAttempts, (maxAttempts - 1) * retryBackoff},
		{"does not retry validation errors", 5, User{ID: "u1"}, ErrInvalidUser, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				store := &flakyStore{failures: tt.failures}
				svc := NewUserService(store)

				start := time.Now()
				err := svc.RetryRegister(t.Context(), tt.user)
				elapsed := time.Since(start)

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want errors.Is(_, %v)", err, tt.wantErr)
				}
				if store.attempts != tt.wantAttempts {
					t.Errorf("attempts = %d, want %d", store.attempts, tt.wantAttempts)
				}
				if elapsed != tt.wantElapsed {
					t.Errorf("virtual elapsed = %v, want exactly %v", elapsed, tt.wantElapsed)
				}
			})
		})
	}
}

func TestRetryRegisterStopsWhenContextIsCancelled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		// Cancel after the first backoff has started: the second attempt
		// never happens.
		ctx, cancel := context.WithTimeout(t.Context(), retryBackoff/2)
		defer cancel()
		store := &flakyStore{failures: 5}

		err := NewUserService(store).RetryRegister(ctx, User{ID: "u1", Name: "Ada", Email: "ada@example.com"})
		if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrConnLost) {
			t.Fatalf("err = %v, want both the last attempt's error and DeadlineExceeded", err)
		}
		if store.attempts != 1 {
			t.Errorf("attempts = %d, want 1", store.attempts)
		}
	})
}
