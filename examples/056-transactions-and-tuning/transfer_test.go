package main

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These checks happen before TransferFunds touches the database, so they run
// without Postgres (the nil pool is never used).
func TestTransferValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		from, to, amount int64
		want             error
	}{
		{"zero amount", 1, 2, 0, ErrInvalidAmount},
		{"negative amount", 1, 2, -100, ErrInvalidAmount},
		{"same account", 3, 3, 100, ErrSameAccount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := TransferFunds(t.Context(), nil, tt.from, tt.to, tt.amount); !errors.Is(err, tt.want) {
				t.Errorf("TransferFunds = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCents(t *testing.T) {
	t.Parallel()
	for in, want := range map[int64]string{0: "$0.00", 5: "$0.05", 2550: "$25.50", -500: "-$5.00"} {
		if got := cents(in); got != want {
			t.Errorf("cents(%d) = %q, want %q", in, got, want)
		}
	}
}

// setup connects, creates the p056_ tables and drops them when the test
// ends. Tests that use it must not run in parallel: they share the tables.
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; see compose.yaml")
	}
	pool, err := newPool(t.Context(), dsn, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := createSchema(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), dropSQL); err != nil {
			t.Errorf("drop tables: %v", err)
		}
	})
	return pool
}

func TestTransferErrors(t *testing.T) {
	pool := setup(t)
	tests := []struct {
		name             string
		from, to, amount int64
		want             error
	}{
		{"insufficient funds", 4, 1, 1, ErrInsufficientFunds},
		{"unknown receiver", 1, 99, 1, ErrAccountNotFound},
		{"unknown sender", 99, 1, 1, ErrAccountNotFound},
		{"exact balance is allowed", 3, 4, 2500, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := TransferFunds(t.Context(), pool, tt.from, tt.to, tt.amount); !errors.Is(err, tt.want) {
				t.Errorf("TransferFunds = %v, want %v", err, tt.want)
			}
		})
	}
	total, err := totalBalance(t.Context(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if total != 17500 {
		t.Errorf("total = %d, want 17500: a failed transfer changed a balance", total)
	}
}

// TestConcurrentTransfers runs many transfers in both directions between all
// accounts at once. With ordered locking there must be no deadlocks, and the
// total must not change.
func TestConcurrentTransfers(t *testing.T) {
	pool := setup(t)
	const goroutines, perGoroutine = 16, 10

	var deadlocks, done atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*perGoroutine)
	for g := range goroutines {
		wg.Go(func() {
			for i := range perGoroutine {
				from := int64((g+i)%4 + 1)
				to := int64((g+i+1+g%3)%4 + 1)
				if from == to {
					continue
				}
				err := TransferFunds(t.Context(), pool, from, to, int64(1+i))
				switch {
				case err == nil:
					done.Add(1)
				case sqlState(err) == codeDeadlockDetected:
					deadlocks.Add(1)
				case errors.Is(err, ErrInsufficientFunds):
					// Fine: account 4 starts empty.
				default:
					errs <- err
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("unexpected error: %v", err)
	}

	if n := deadlocks.Load(); n != 0 {
		t.Errorf("%d deadlocks, want 0", n)
	}
	if done.Load() == 0 {
		t.Error("no transfer succeeded")
	}
	total, err := totalBalance(t.Context(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if total != 17500 {
		t.Errorf("total = %d, want 17500", total)
	}
}

func TestWriteSkew(t *testing.T) {
	pool := setup(t)
	tests := []struct {
		iso        pgx.TxIsoLevel
		wantBob    string
		wantOnCall int
	}{
		{pgx.ReadCommitted, "", 0},                      // both commit: the rule is broken
		{pgx.Serializable, codeSerializationFailure, 1}, // bob is aborted
	}
	for _, tt := range tests {
		t.Run(string(tt.iso), func(t *testing.T) {
			errAlice, errBob, onCall, err := interleave(t.Context(), pool, tt.iso)
			if err != nil {
				t.Fatal(err)
			}
			if errAlice != nil {
				t.Errorf("alice: %v, want commit", errAlice)
			}
			if got := sqlState(errBob); got != tt.wantBob || (tt.wantBob == "" && errBob != nil) {
				t.Errorf("bob: %v, want SQLSTATE %q", errBob, tt.wantBob)
			}
			if onCall != tt.wantOnCall {
				t.Errorf("on call = %d, want %d", onCall, tt.wantOnCall)
			}
		})
	}
}
