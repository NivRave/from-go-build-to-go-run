package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres error codes (SQLSTATE) this example handles.
const (
	codeCheckViolation       = "23514"
	codeDeadlockDetected     = "40P01"
	codeSerializationFailure = "40001"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotFound   = errors.New("account not found")
	ErrSameAccount       = errors.New("cannot transfer to the same account")
	ErrInvalidAmount     = errors.New("amount must be positive")
)

// sqlState returns the Postgres error code wrapped in err, or "". It uses Go
// 1.26's errors.AsType, the generic form of errors.As.
func sqlState(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}

// TransferFunds moves amount cents from one account to another. It fixes the
// slide p03 version:
//
//   - Money is int64 cents, not float64, which cannot represent 0.10 exactly.
//   - Both rows are locked first, in id order, so two opposite transfers
//     (A to B and B to A) queue up instead of deadlocking.
//   - A missing account is an error: an UPDATE that matches no rows
//     succeeds silently, so RowsAffected is checked.
//   - The balance is checked by the table's CHECK (bal >= 0) constraint,
//     which no code path can forget.
//   - pgx.BeginFunc commits if the function returns nil and rolls back
//     otherwise, which replaces Begin + defer Rollback + Commit.
func TransferFunds(ctx context.Context, pool *pgxpool.Pool, from, to, amount int64) error {
	switch {
	case amount <= 0:
		return ErrInvalidAmount
	case from == to:
		return ErrSameAccount
	}
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id FROM p056_accounts WHERE id = ANY($1) ORDER BY id FOR UPDATE`,
			[]int64{from, to})
		if err != nil {
			return err
		}
		locked, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil {
			return err
		}
		if len(locked) != 2 {
			return ErrAccountNotFound
		}

		if err := addToBalance(ctx, tx, from, -amount); err != nil {
			return err
		}
		return addToBalance(ctx, tx, to, amount)
	})
	if sqlState(err) == codeCheckViolation {
		return fmt.Errorf("transfer %d -> %d: %w", from, to, ErrInsufficientFunds)
	}
	return err
}

func addToBalance(ctx context.Context, tx pgx.Tx, id, delta int64) error {
	tag, err := tx.Exec(ctx, `UPDATE p056_accounts SET bal = bal + $1 WHERE id = $2`, delta, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("account %d: %w", id, ErrAccountNotFound)
	}
	return nil
}

// transferNaive is slide p03's shape with int64 money: debit the sender, then
// credit the receiver, locking rows in whatever order the transfer names
// them. The pause between the two statements stands in for work done inside
// the transaction (which slide p02 warns against) and makes the deadlock
// window easy to hit.
func transferNaive(ctx context.Context, pool *pgxpool.Pool, from, to, amount int64) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE p056_accounts SET bal = bal - $1 WHERE id = $2`, amount, from); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		_, err := tx.Exec(ctx, `UPDATE p056_accounts SET bal = bal + $1 WHERE id = $2`, amount, to)
		return err
	})
}

// retrySerializable runs fn in a Serializable transaction and retries it when
// Postgres aborts it with a serialization failure (40001) or a deadlock
// (40P01). Retrying is the price of Serializable, and it is only safe because
// fn re-reads everything it depends on.
func retrySerializable(ctx context.Context, pool *pgxpool.Pool, attempts int, fn func(pgx.Tx) error) (int, error) {
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		err = pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
		switch sqlState(err) {
		case codeSerializationFailure, codeDeadlockDetected:
			time.Sleep(time.Duration(attempt) * 10 * time.Millisecond) // small backoff
			continue
		}
		return attempt, err
	}
	return attempts, err
}

// slowQueryTracer logs every query slower than threshold (slide p08). pgx v5
// calls it through ConnConfig.Tracer. A tracer observes queries; it cannot
// rewrite the SQL, so adding trace IDs as SQL comments needs a wrapper around
// the query calls instead.
//
// The demo switches it on only for section 5. Otherwise every statement that
// waited on a row lock in the deadlock demo would be logged too, which is
// exactly what you want in production: lock waits are slow queries.
type slowQueryTracer struct {
	threshold time.Duration
	logger    *slog.Logger
	enabled   atomic.Bool
}

type queryStartKey struct{}

type queryStart struct {
	sql   string
	start time.Time
}

func (t *slowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStartKey{}, queryStart{sql: data.SQL, start: time.Now()})
}

func (t *slowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	qs, ok := ctx.Value(queryStartKey{}).(queryStart)
	if !ok || !t.enabled.Load() {
		return
	}
	if d := time.Since(qs.start); d > t.threshold {
		t.logger.Warn("slow query", "duration", d.Round(time.Millisecond), "sql", qs.sql, "err", data.Err)
	}
}
