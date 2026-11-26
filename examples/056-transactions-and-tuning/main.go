// Command transactions-and-tuning demonstrates Postgres transactions with
// pgx v5: a safe TransferFunds, a deadlock and its fix, write skew under Read
// Committed versus Serializable, pool tuning and slow-query logging.
//
// It needs a Postgres server (see the root compose.yaml) and DATABASE_URL.
// All tables are prefixed p056_, created at start and dropped at the end.
//
// Slides: slides/056-transactions-and-tuning.pdf
//
// Run:
//
//	docker compose up -d
//	export DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'
//	go run ./examples/056-transactions-and-tuning
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaSQL = `
CREATE TABLE p056_accounts (
	id  bigint PRIMARY KEY,
	bal bigint NOT NULL CHECK (bal >= 0)   -- cents
);
INSERT INTO p056_accounts (id, bal) VALUES (1, 10000), (2, 5000), (3, 2500), (4, 0);
CREATE TABLE p056_oncall (
	doctor  text PRIMARY KEY,
	on_call boolean NOT NULL
);`

const dropSQL = `DROP TABLE IF EXISTS p056_accounts, p056_oncall`

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		printInstructions(os.Stderr)
		os.Exit(1)
	}
	if err := run(context.Background(), dsn, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printInstructions(w io.Writer) {
	fmt.Fprint(w, `DATABASE_URL is not set. This example needs a Postgres server.
From the repository root:

  docker compose up -d
  export DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'
  # PowerShell: $env:DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'
  go run ./examples/056-transactions-and-tuning

Stop the database afterwards with: docker compose down
`)
}

// run owns every resource and defer, so main's os.Exit never skips them.
func run(ctx context.Context, dsn string, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	pool, err := newPool(ctx, dsn, out)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := createSchema(ctx, pool); err != nil {
		return err
	}
	defer func() {
		if _, err := pool.Exec(context.WithoutCancel(ctx), dropSQL); err != nil {
			fmt.Fprintln(out, "cleanup:", err)
			return
		}
		fmt.Fprintln(out, "\ncleanup: dropped the p056_ tables")
	}()

	section(out, "1. Accounts in int64 cents, CHECK (bal >= 0) (slide p03)")
	if err := printBalances(ctx, pool, out); err != nil {
		return err
	}

	section(out, "2. TransferFunds: lock in id order, check rows and balance (slide p03)")
	if err := transfers(ctx, pool, out); err != nil {
		return err
	}

	section(out, "3. Deadlocks: opposite transfers, naive vs ordered (slide p07)")
	if err := deadlocks(ctx, pool, out); err != nil {
		return err
	}

	section(out, "4. Isolation: write skew under Read Committed vs Serializable (slide p04)")
	if err := writeSkew(ctx, pool, out); err != nil {
		return err
	}

	section(out, "5. Pool tuning and slow-query logging (slides p05, p06, p08)")
	return poolAndSlowQueries(ctx, pool, out)
}

// newPool is slide p06 with its errors checked. The values are examples; the
// right ones depend on your database and the number of app instances.
func newPool(ctx context.Context, dsn string, out io.Writer) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute          // recycle connections (default 1h)
	config.MaxConnLifetimeJitter = 5 * time.Minute     // ...but not all at the same moment
	config.MaxConnIdleTime = 5 * time.Minute           // close idle ones (default 30m)
	config.ConnConfig.ConnectTimeout = 5 * time.Second // limit for dialling a new one

	logger := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{} // keep the demo output short
			}
			return a
		},
	}))
	config.ConnConfig.Tracer = &slowQueryTracer{threshold: 200 * time.Millisecond, logger: logger}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func createSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("drop old tables: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

// resetAccounts puts the balances back to the seed values between demos.
func resetAccounts(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `UPDATE p056_accounts SET bal = CASE id WHEN 1 THEN 10000 WHEN 2 THEN 5000 WHEN 3 THEN 2500 ELSE 0 END`)
	return err
}

func totalBalance(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var total int64
	err := pool.QueryRow(ctx, `SELECT sum(bal) FROM p056_accounts`).Scan(&total)
	return total, err
}

func printBalances(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	rows, err := pool.Query(ctx, `SELECT id, bal FROM p056_accounts ORDER BY id`)
	if err != nil {
		return err
	}
	type account struct {
		ID  int64 `db:"id"`
		Bal int64 `db:"bal"`
	}
	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByName[account])
	if err != nil {
		return err
	}
	var total int64
	for _, a := range accounts {
		fmt.Fprintf(out, "account %d: %s\n", a.ID, cents(a.Bal))
		total += a.Bal
	}
	fmt.Fprintf(out, "total:     %s\n", cents(total))
	return nil
}

func transfers(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	cases := []struct {
		from, to, amount int64
	}{
		{1, 2, 2550}, // fine
		{4, 1, 100},  // account 4 has $0.00: the CHECK constraint fires
		{1, 99, 100}, // no account 99: only one row locked
		{2, 2, 100},  // rejected before touching the database
		{3, 1, -500}, // a negative amount would reverse the transfer
	}
	for _, c := range cases {
		err := TransferFunds(ctx, pool, c.from, c.to, c.amount)
		fmt.Fprintf(out, "transfer %d -> %-2d %8s: err = %v\n", c.from, c.to, cents(c.amount), err)
	}
	return printBalances(ctx, pool, out)
}

// runConcurrently runs n transfers 1 -> 2 and n transfers 2 -> 1 at once and
// counts the ones Postgres aborted as deadlock victims.
func runConcurrently(ctx context.Context, n int, transfer func(ctx context.Context, from, to int64) error) (deadlocks int64, err error) {
	var wg sync.WaitGroup
	var dl atomic.Int64
	errs := make(chan error, 2*n)
	for i := range 2 * n {
		from, to := int64(1), int64(2)
		if i%2 == 1 {
			from, to = to, from
		}
		wg.Go(func() {
			err := transfer(ctx, from, to)
			switch {
			case sqlState(err) == codeDeadlockDetected:
				dl.Add(1)
			case err != nil:
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	return dl.Load(), <-errs // nil if no other error happened
}

func deadlocks(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	if err := resetAccounts(ctx, pool); err != nil {
		return err
	}
	const n = 5 // goroutines per direction; Postgres needs deadlock_timeout (1s) to spot each cycle

	start := time.Now()
	dl, err := runConcurrently(ctx, n, func(ctx context.Context, from, to int64) error {
		return transferNaive(ctx, pool, from, to, 100)
	})
	if err != nil {
		return fmt.Errorf("naive transfers: %w", err)
	}
	fmt.Fprintf(out, "naive:   %d transfers, %d aborted with SQLSTATE 40P01 (deadlock detected) after %v\n",
		2*n, dl, time.Since(start).Round(100*time.Millisecond))

	start = time.Now()
	dl, err = runConcurrently(ctx, n, func(ctx context.Context, from, to int64) error {
		return TransferFunds(ctx, pool, from, to, 100)
	})
	if err != nil {
		return fmt.Errorf("ordered transfers: %w", err)
	}
	fmt.Fprintf(out, "ordered: %d transfers, %d deadlocks after %v\n", 2*n, dl, time.Since(start).Round(100*time.Millisecond))

	total, err := totalBalance(ctx, pool)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "total balance is still %s: a deadlock victim is rolled back as a whole\n", cents(total))
	return nil
}

var errLastOnCall = errors.New("refused: at least one doctor must stay on call")

// goOffCall is the business rule "at least one doctor stays on call": read
// how many are on call, then take this doctor off if another one remains.
func goOffCall(ctx context.Context, tx pgx.Tx, doctor string) error {
	var onCall int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM p056_oncall WHERE on_call`).Scan(&onCall); err != nil {
		return err
	}
	if onCall < 2 {
		return errLastOnCall
	}
	_, err := tx.Exec(ctx, `UPDATE p056_oncall SET on_call = false WHERE doctor = $1`, doctor)
	return err
}

// interleave runs goOffCall for alice and bob in two transactions whose
// reads both happen before either writes: the classic write-skew schedule.
// It returns each transaction's outcome and how many doctors are on call
// afterwards.
func interleave(ctx context.Context, pool *pgxpool.Pool, iso pgx.TxIsoLevel) (errAlice, errBob error, onCall int, err error) {
	if _, err := pool.Exec(ctx, `DELETE FROM p056_oncall; INSERT INTO p056_oncall VALUES ('alice', true), ('bob', true)`); err != nil {
		return nil, nil, 0, err
	}
	txA, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: iso})
	if err != nil {
		return nil, nil, 0, err
	}
	defer txA.Rollback(ctx) //nolint:errcheck // intentional: no-op after Commit
	txB, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: iso})
	if err != nil {
		return nil, nil, 0, err
	}
	defer txB.Rollback(ctx) //nolint:errcheck // intentional: no-op after Commit

	// Both read "2 on call" before either has written.
	var countA, countB int
	if err := txA.QueryRow(ctx, `SELECT count(*) FROM p056_oncall WHERE on_call`).Scan(&countA); err != nil {
		return nil, nil, 0, err
	}
	if err := txB.QueryRow(ctx, `SELECT count(*) FROM p056_oncall WHERE on_call`).Scan(&countB); err != nil {
		return nil, nil, 0, err
	}
	errAlice = updateAndCommit(ctx, txA, "alice", countA)
	errBob = updateAndCommit(ctx, txB, "bob", countB)

	err = pool.QueryRow(ctx, `SELECT count(*) FROM p056_oncall WHERE on_call`).Scan(&onCall)
	return errAlice, errBob, onCall, err
}

func updateAndCommit(ctx context.Context, tx pgx.Tx, doctor string, onCall int) error {
	if onCall < 2 {
		return errLastOnCall
	}
	if _, err := tx.Exec(ctx, `UPDATE p056_oncall SET on_call = false WHERE doctor = $1`, doctor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func describe(err error) string {
	if err == nil {
		return "committed"
	}
	if code := sqlState(err); code != "" {
		return "aborted, SQLSTATE " + code
	}
	return err.Error()
}

func writeSkew(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	for _, iso := range []pgx.TxIsoLevel{pgx.ReadCommitted, pgx.Serializable} {
		a, b, onCall, err := interleave(ctx, pool, iso)
		if err != nil {
			return fmt.Errorf("%s: %w", iso, err)
		}
		fmt.Fprintf(out, "%-15s alice: %s; bob: %s; doctors on call: %d\n", string(iso)+":", describe(a), describe(b), onCall)
	}

	// The aborted transaction must be retried from the start, re-reading
	// the data. Now it sees that alice already left, and the rule holds.
	attempts, err := retrySerializable(ctx, pool, 5, func(tx pgx.Tx) error { return goOffCall(ctx, tx, "bob") })
	fmt.Fprintf(out, "bob retried under serializable: attempt %d: %v\n", attempts, err)
	return nil
}

func poolAndSlowQueries(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	cfg := pool.Config()
	fmt.Fprintf(out, "config: MaxConns=%d MinConns=%d MaxConnLifetime=%v (+ up to %v jitter) MaxConnIdleTime=%v ConnectTimeout=%v\n",
		cfg.MaxConns, cfg.MinConns, cfg.MaxConnLifetime, cfg.MaxConnLifetimeJitter, cfg.MaxConnIdleTime, cfg.ConnConfig.ConnectTimeout)

	s := pool.Stat()
	fmt.Fprintf(out, "stat:   total=%d idle=%d acquired=%d max=%d acquires=%d new-conns=%d waited-for-conn=%d\n",
		s.TotalConns(), s.IdleConns(), s.AcquiredConns(), s.MaxConns(), s.AcquireCount(), s.NewConnsCount(), s.EmptyAcquireCount())

	// The tracer is installed on every connection through ConnConfig.Tracer.
	if tracer, ok := cfg.ConnConfig.Tracer.(*slowQueryTracer); ok {
		tracer.enabled.Store(true)
		defer tracer.enabled.Store(false)
		fmt.Fprintf(out, "slow-query tracer on, threshold %v\n", tracer.threshold)
	}
	// The tracer logs this one: it takes longer than the 200 ms threshold.
	if _, err := pool.Exec(ctx, `SELECT pg_sleep(0.3)`); err != nil {
		return err
	}
	// This one is fast, so nothing is logged.
	if _, err := pool.Exec(ctx, `SELECT 1`); err != nil {
		return err
	}
	return nil
}

func cents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s$%d.%02d", sign, c/100, c%100)
}

func section(w io.Writer, title string) {
	fmt.Fprintf(w, "\n== %s ==\n", title)
}
