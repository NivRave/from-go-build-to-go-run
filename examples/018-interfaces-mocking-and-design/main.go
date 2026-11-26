// Command interfaces-mocking-and-design demonstrates a consumer-defined
// interface: UserService depends on a DataStore interface, main wires it to an
// in-memory store, and service_test.go swaps in hand-written mocks.
//
// Slides: slides/018-interfaces-mocking-and-design.pdf
//
// Run:
//
//	go run ./examples/018-interfaces-mocking-and-design
//	go test -v ./examples/018-interfaces-mocking-and-design
package main

import (
	"context"
	"errors"
	"fmt"
)

func main() {
	ctx := context.Background()

	section("1. UserService wired to an in-memory store (slides p02-p03)")
	store := NewMemStore()
	svc := NewUserService(store) // the only place that knows the concrete type
	registerUsers(ctx, svc, store)

	section("2. Validation errors are values the caller can inspect (slide p05)")
	showValidationError(ctx, svc)

	fmt.Println("\nThe interesting part is in service_test.go: go test -v ./examples/018-interfaces-mocking-and-design")
}

func registerUsers(ctx context.Context, svc *UserService, store *MemStore) {
	for _, u := range []User{
		{ID: "u1", Name: "Ada", Email: "ada@example.com"},
		{ID: "u2", Name: "Linus", Email: "linus@example.com"},
	} {
		fmt.Printf("register %s: err = %v\n", u.ID, svc.Register(ctx, u))
	}
	fmt.Println("users stored:", store.Len())

	u, err := svc.Lookup(ctx, "u2")
	fmt.Printf("lookup u2: %s <%s>, err = %v\n", u.Name, u.Email, err)
}

func showValidationError(ctx context.Context, svc *UserService) {
	err := svc.Register(ctx, User{ID: "u3", Name: "Grace", Email: "grace.example.com"})
	fmt.Println("register u3:", err)
	fmt.Println("errors.Is(err, ErrInvalidUser):", errors.Is(err, ErrInvalidUser))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
