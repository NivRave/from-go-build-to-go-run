package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// User is the domain type the service works with.
type User struct {
	ID    string
	Name  string
	Email string
}

var (
	// ErrInvalidUser is returned (wrapped) when validation fails.
	ErrInvalidUser = errors.New("invalid user")
	// ErrNotFound is returned by a DataStore that has no such user.
	ErrNotFound = errors.New("user not found")
)

// DataStore is defined here, by the consumer (slide p03), and lists only the
// behaviour UserService needs. A Postgres type in another package can satisfy
// it without importing this one. It has three methods so that the partial
// mock in service_test.go (slide p07) has something to leave out.
type DataStore interface {
	SaveUser(ctx context.Context, u User) error
	GetUser(ctx context.Context, id string) (User, error)
	DeleteUser(ctx context.Context, id string) error
}

// UserService holds business logic. Its dependency is the interface.
type UserService struct {
	store DataStore
}

// NewUserService injects the store, so tests can pass a mock.
func NewUserService(store DataStore) *UserService {
	return &UserService{store: store}
}

// Validate is pure logic with no I/O. It needs no interface and no mock:
// tests call it directly (slide p06, "don't mock your own logic").
func Validate(u User) error {
	switch {
	case u.ID == "":
		return fmt.Errorf("%w: empty ID", ErrInvalidUser)
	case strings.TrimSpace(u.Name) == "":
		return fmt.Errorf("%w: empty name", ErrInvalidUser)
	case !strings.Contains(u.Email, "@"):
		return fmt.Errorf("%w: email %q has no @", ErrInvalidUser, u.Email)
	}
	return nil
}

// Register validates u and saves it. Store errors are wrapped with %w, so a
// caller (or a test) can still match the cause with errors.Is.
func (s *UserService) Register(ctx context.Context, u User) error {
	if err := Validate(u); err != nil {
		return err
	}
	if err := s.store.SaveUser(ctx, u); err != nil {
		return fmt.Errorf("register %s: %w", u.ID, err)
	}
	return nil
}

// Lookup returns the user with the given ID.
func (s *UserService) Lookup(ctx context.Context, id string) (User, error) {
	u, err := s.store.GetUser(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf("lookup %s: %w", id, err)
	}
	return u, nil
}

// Unregister deletes the user with the given ID.
func (s *UserService) Unregister(ctx context.Context, id string) error {
	if err := s.store.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("unregister %s: %w", id, err)
	}
	return nil
}

const (
	maxAttempts  = 3
	retryBackoff = 100 * time.Millisecond
)

// RetryRegister calls Register up to maxAttempts times, sleeping retryBackoff
// between attempts. Validation errors are not retried: trying again won't
// fix them. It sleeps with time.Sleep on purpose: since Go 1.25,
// testing/synctest runs this on a fake clock, so the test needs no Clock
// interface and finishes instantly (see TestRetryRegister).
func (s *UserService) RetryRegister(ctx context.Context, u User) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(retryBackoff)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(err, ctxErr)
		}
		err = s.Register(ctx, u)
		if err == nil || errors.Is(err, ErrInvalidUser) {
			return err
		}
	}
	return fmt.Errorf("after %d attempts: %w", maxAttempts, err)
}

// MemStore is a real, concurrency-safe in-memory DataStore. main uses it; a
// production build would use a database-backed type with the same methods.
type MemStore struct {
	mu    sync.Mutex
	users map[string]User
}

func NewMemStore() *MemStore {
	return &MemStore{users: make(map[string]User)}
}

func (m *MemStore) SaveUser(_ context.Context, u User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}

func (m *MemStore) GetUser(_ context.Context, id string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *MemStore) DeleteUser(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[id]; !ok {
		return ErrNotFound
	}
	delete(m.users, id)
	return nil
}

// Len reports how many users are stored.
func (m *MemStore) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.users)
}
