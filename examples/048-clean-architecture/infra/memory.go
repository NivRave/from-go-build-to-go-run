// Package infra is the outer layer: concrete storage that satisfies the
// use-case interfaces. Swapping it for Postgres changes this package and the
// wiring in main, and nothing else.
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/domain"
)

// dbUser is the storage shape of a user, with its own tags and field names
// (FullName, "full_name"). It never leaves this package: GetUser maps it to
// a domain.User at the boundary (slide p06), so a column rename touches only
// this file.
type dbUser struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type dbOrder struct {
	ID     string `json:"order_id"`
	UserID string `json:"user_id"`
	Cents  int    `json:"amount_cents"`
}

func toDBUser(u domain.User) dbUser    { return dbUser{ID: u.ID, FullName: u.Name} }
func (d dbUser) toDomain() domain.User { return domain.User{ID: d.ID, Name: d.FullName} }

func toDBOrder(o domain.Order) dbOrder {
	return dbOrder{ID: o.ID, UserID: o.UserID, Cents: o.AmountCents}
}

func (d dbOrder) toDomain() domain.Order {
	return domain.Order{ID: d.ID, UserID: d.UserID, AmountCents: d.Cents}
}

// MemoryRepo stores rows as JSON documents in memory, like a tiny document
// database. The name doesn't clash with the use-case interfaces (slide p06
// reuses "UserRepository" for both the interface and the struct).
type MemoryRepo struct {
	mu     sync.RWMutex
	users  map[string][]byte
	orders map[string][]byte
}

func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{users: map[string][]byte{}, orders: map[string][]byte{}}
}

// AddUser inserts or replaces a user.
func (r *MemoryRepo) AddUser(u domain.User) error {
	row, err := json.Marshal(toDBUser(u))
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.ID] = row
	return nil
}

// GetUser satisfies usecase.UserGetter.
//
// With database/sql it would look like this. Note the placeholder: building
// the query from id (or GORM's First(&u, id) with a string id) is an SQL
// injection risk.
//
//	var d dbUser
//	err := db.QueryRowContext(ctx,
//		`SELECT id, full_name FROM users WHERE id = $1`, id).Scan(&d.ID, &d.FullName)
//	if errors.Is(err, sql.ErrNoRows) {
//		return nil, domain.ErrNotFound
//	}
func (r *MemoryRepo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err // a real driver would abort the query on ctx too
	}
	r.mu.RLock()
	row, ok := r.users[id]
	r.mu.RUnlock()
	if !ok {
		// Translate "missing" into the domain's error, so use cases never
		// see a storage-specific one.
		return nil, domain.ErrNotFound
	}
	var d dbUser
	if err := json.Unmarshal(row, &d); err != nil {
		return nil, fmt.Errorf("decode user %s: %w", id, err)
	}
	u := d.toDomain()
	return &u, nil
}

// SaveOrder satisfies usecase.OrderSaver.
func (r *MemoryRepo) SaveOrder(ctx context.Context, o *domain.Order) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	row, err := json.Marshal(toDBOrder(*o))
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = row
	return nil
}

// Orders returns every saved order, sorted by ID.
func (r *MemoryRepo) Orders() ([]domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Order, 0, len(r.orders))
	for _, id := range slices.Sorted(maps.Keys(r.orders)) {
		var d dbOrder
		if err := json.Unmarshal(r.orders[id], &d); err != nil {
			return nil, fmt.Errorf("decode order %s: %w", id, err)
		}
		out = append(out, d.toDomain())
	}
	return out, nil
}

// RawUser returns the stored JSON for a user, to show the storage shape.
func (r *MemoryRepo) RawUser(id string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return string(r.users[id])
}
