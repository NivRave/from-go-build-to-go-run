package domain

import "errors"

// Domain errors. Infrastructure translates its own failures into these
// (sql.ErrNoRows, a missing map key, an HTTP 404) so the use cases can
// test for them with errors.Is without importing any storage package.
var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidOrder = errors.New("invalid order")
)
