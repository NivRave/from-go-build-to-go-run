package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/domain"
	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/usecase"
)

// errUnexpectedCall is what a mock returns when a test didn't expect the
// method to be called and so left its function field nil. Slide p05's mock
// calls m.OnGet unconditionally and panics in that case.
var errUnexpectedCall = errors.New("mock: unexpected call")

// mockUsers and mockOrders are hand-rolled function-field mocks (slide p05).
// Each test sets only the behaviour it needs.
type mockUsers struct {
	OnGetUser func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUsers) GetUser(ctx context.Context, id string) (*domain.User, error) {
	if m.OnGetUser == nil {
		return nil, errUnexpectedCall
	}
	return m.OnGetUser(ctx, id)
}

type mockOrders struct {
	OnSaveOrder func(ctx context.Context, o *domain.Order) error
	saved       []string
}

func (m *mockOrders) SaveOrder(ctx context.Context, o *domain.Order) error {
	if m.OnSaveOrder == nil {
		return errUnexpectedCall
	}
	m.saved = append(m.saved, o.ID)
	return m.OnSaveOrder(ctx, o)
}

func userFound(_ context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id, Name: "Test User"}, nil
}

func saveOK(context.Context, *domain.Order) error { return nil }

func TestProcessOrder(t *testing.T) {
	t.Parallel()
	valid := domain.Order{ID: "o1", UserID: "u1", AmountCents: 500}

	tests := []struct {
		name      string
		order     *domain.Order
		getUser   func(context.Context, string) (*domain.User, error)
		saveOrder func(context.Context, *domain.Order) error
		wantErr   error // checked with errors.Is; nil means success
		wantSaved int
	}{
		{
			name: "success", order: &valid,
			getUser: userFound, saveOrder: saveOK,
			wantSaved: 1,
		},
		{
			name: "user not found", order: &valid,
			getUser: func(context.Context, string) (*domain.User, error) { return nil, domain.ErrNotFound },
			// saveOrder left nil: saving must not happen.
			wantErr: domain.ErrNotFound,
		},
		{
			name: "user lookup times out", order: &valid,
			getUser: func(context.Context, string) (*domain.User, error) { return nil, context.DeadlineExceeded },
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "save times out", order: &valid,
			getUser:   userFound,
			saveOrder: func(context.Context, *domain.Order) error { return context.DeadlineExceeded },
			wantErr:   context.DeadlineExceeded, wantSaved: 1,
		},
		{
			name: "zero amount", order: &domain.Order{ID: "o2", UserID: "u1"},
			// Neither mock is set: validation must fail before any call.
			wantErr: domain.ErrInvalidOrder,
		},
		{
			name: "nil order", order: nil,
			wantErr: domain.ErrInvalidOrder,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users := &mockUsers{OnGetUser: tt.getUser}
			orders := &mockOrders{OnSaveOrder: tt.saveOrder}

			err := usecase.ProcessOrder(t.Context(), tt.order, users, orders)

			if errors.Is(err, errUnexpectedCall) {
				t.Fatalf("a mock was called unexpectedly: %v", err)
			}
			if tt.wantErr == nil && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if len(orders.saved) != tt.wantSaved {
				t.Errorf("SaveOrder called %d times, want %d", len(orders.saved), tt.wantSaved)
			}
		})
	}
}

// TestProcessOrderPassesContext checks that the caller's ctx is the one the
// dependencies receive, which slide p07's context.Background() would break.
func TestProcessOrderPassesContext(t *testing.T) {
	t.Parallel()
	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "request-123")

	check := func(got context.Context) error {
		if got.Value(key{}) != "request-123" {
			return errors.New("ctx was not propagated")
		}
		return nil
	}
	users := &mockUsers{OnGetUser: func(c context.Context, id string) (*domain.User, error) {
		if err := check(c); err != nil {
			return nil, err
		}
		return userFound(c, id)
	}}
	orders := &mockOrders{OnSaveOrder: func(c context.Context, _ *domain.Order) error { return check(c) }}

	if err := usecase.ProcessOrder(ctx, &domain.Order{ID: "o1", UserID: "u1", AmountCents: 1}, users, orders); err != nil {
		t.Fatal(err)
	}
}
