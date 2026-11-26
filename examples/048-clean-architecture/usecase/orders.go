// Package usecase holds the business rules. It depends on domain and on the
// two small interfaces below, which it defines itself; it never imports
// infra.
package usecase

import (
	"context"
	"fmt"

	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/domain"
)

// UserGetter and OrderSaver are consumer-defined and deliberately tiny
// (slide p07): ProcessOrder needs one read and one write, so that is all it
// asks for, and a test mock needs only one function each. A 50-method
// "Database" interface would force every mock to implement all 50.
//
// Every method takes a ctx, so cancellation, deadlines and trace IDs flow
// from the caller down to the storage call.
type UserGetter interface {
	GetUser(ctx context.Context, id string) (*domain.User, error)
}

type OrderSaver interface {
	SaveOrder(ctx context.Context, o *domain.Order) error
}

// ProcessOrder validates o, checks that its user exists, and saves it.
//
// Slide p07's version calls s.SaveOrder(context.Background(), o). That
// detaches the save from the caller: a cancelled request or an expired
// deadline wouldn't stop it, and tracing would lose the span. Taking ctx as
// the first parameter and passing it on fixes that.
func ProcessOrder(ctx context.Context, o *domain.Order, users UserGetter, orders OrderSaver) error {
	if o == nil || o.ID == "" || o.AmountCents <= 0 {
		return fmt.Errorf("process order: %w", domain.ErrInvalidOrder)
	}
	u, err := users.GetUser(ctx, o.UserID)
	if err != nil {
		return fmt.Errorf("process order %s: get user %s: %w", o.ID, o.UserID, err)
	}
	if err := orders.SaveOrder(ctx, o); err != nil {
		return fmt.Errorf("process order %s for %s: %w", o.ID, u.Name, err)
	}
	return nil
}
