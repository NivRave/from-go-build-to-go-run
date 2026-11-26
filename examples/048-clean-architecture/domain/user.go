// Package domain holds the entities at the centre of the onion. It imports
// nothing from the other layers, and nothing here knows about storage,
// JSON or HTTP.
package domain

// User is the business view of a customer. Its fields are named for the
// business, not for a database column (compare infra's dbUser).
type User struct {
	ID   string
	Name string
}

// Order is a purchase by a user. Amounts are in cents to avoid floating
// point.
type Order struct {
	ID          string
	UserID      string
	AmountCents int
}
