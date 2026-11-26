// Command mastering-the-db demonstrates Postgres from Go with pgx v5 and sqlx:
// a checked pgxpool, struct mapping two ways, NULL handling, SQL injection
// versus parameters, named inserts, and bulk loading with COPY.
//
// It needs a Postgres server (see the root compose.yaml) and DATABASE_URL.
// All tables are prefixed p055_, created at start and dropped at the end.
//
// Slides: slides/055-mastering-the-db.pdf
//
// Run:
//
//	docker compose up -d
//	export DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'
//	go run ./examples/055-mastering-the-db
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql (and so sqlx)
	"github.com/jmoiron/sqlx"
)

// User mirrors slide p04. The db tags name the columns; both
// pgx.RowToStructByName and sqlx honour them.
type User struct {
	ID    int64  `db:"user_id"`
	Email string `db:"email"`
}

// NewUser is the input for the named inserts (slide p08). NamedExec needs the
// db tags to map :first_name and friends to fields.
type NewUser struct {
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
	Email     string `db:"email"`
}

const schemaSQL = `
CREATE TABLE p055_users (
	user_id    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	first_name text NOT NULL,
	last_name  text NOT NULL,
	email      text NOT NULL UNIQUE,
	bio        text,                      -- nullable on purpose (slide p06)
	active     boolean NOT NULL DEFAULT true
);
INSERT INTO p055_users (first_name, last_name, email, bio, active) VALUES
	('Ada',   'Lovelace', 'ada@example.com',   'First programmer', true),
	('Linus', 'Torvalds', 'linus@example.com', NULL,               true),
	('Grace', 'Hopper',   'grace@example.com', NULL,               false);
CREATE TABLE p055_injection_victim (id int);`

const dropSQL = `DROP TABLE IF EXISTS p055_users, p055_injection_victim`

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
  go run ./examples/055-mastering-the-db

Stop the database afterwards with: docker compose down
`)
}

// run owns every resource and defer, so main's os.Exit never skips them.
func run(ctx context.Context, dsn string, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	section(out, "1. A checked connection pool (slide p05)")
	pool, err := newPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	fmt.Fprintf(out, "pool ready: MaxConns=%d MinConns=%d, ping ok\n", pool.Config().MaxConns, pool.Config().MinConns)

	if err := createSchema(ctx, pool); err != nil {
		return err
	}
	// Drop the tables even if a demo fails; WithoutCancel keeps this working
	// after ctx has been cancelled.
	defer func() {
		if _, err := pool.Exec(context.WithoutCancel(ctx), dropSQL); err != nil {
			fmt.Fprintln(out, "cleanup:", err)
			return
		}
		fmt.Fprintln(out, "\ncleanup: dropped the p055_ tables")
	}()

	section(out, "2. Struct mapping two ways (slide p04)")
	if err := mapWithPgx(ctx, pool, out); err != nil {
		return err
	}
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return fmt.Errorf("sqlx connect: %w", err)
	}
	defer db.Close()
	if err := mapWithSqlx(ctx, db, out); err != nil {
		return err
	}

	section(out, "3. NULLs (slide p06)")
	if err := nulls(ctx, pool, out); err != nil {
		return err
	}

	section(out, "4. SQL injection vs parameters (slide p07)")
	if err := injection(ctx, pool, out); err != nil {
		return err
	}

	section(out, "5. Named inserts (slide p08)")
	if err := namedInserts(ctx, pool, db, out); err != nil {
		return err
	}

	section(out, "6. Bulk insert with COPY (slide p03)")
	return bulkCopy(ctx, pool, out)
}

// newPool is slide p05 with the missing pieces: the ParseConfig error is
// checked (ignoring it means a nil config and a panic on a bad DSN),
// NewWithConfig is followed by Ping because it does not connect eagerly, and
// the caller defers pool.Close.
func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	// Size against the server's max_connections across *all* instances:
	// 50 instances x 20 connections is already 1,000.
	config.MaxConns = 10
	config.MinConns = 2

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
	// Start from a clean slate in case a previous run was interrupted.
	if _, err := pool.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("drop old tables: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

const activeUsersSQL = `SELECT user_id, email FROM p055_users WHERE active ORDER BY user_id`

// activeUsersPgx maps rows to structs with pgx alone: no database/sql, no
// sqlx. RowToStructByName matches columns to fields by db tag or name.
func activeUsersPgx(ctx context.Context, pool *pgxpool.Pool) ([]User, error) {
	rows, err := pool.Query(ctx, activeUsersSQL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[User]) // closes rows
}

// activeUsersSqlx is slide p04's db.Select with a context. sqlx cannot use a
// *pgxpool.Pool: it wraps database/sql, here with pgx's stdlib driver.
func activeUsersSqlx(ctx context.Context, db *sqlx.DB) ([]User, error) {
	var users []User
	err := db.SelectContext(ctx, &users, activeUsersSQL)
	return users, err
}

func mapWithPgx(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	users, err := activeUsersPgx(ctx, pool)
	if err != nil {
		return fmt.Errorf("pgx.CollectRows: %w", err)
	}
	fmt.Fprintf(out, "pgx.CollectRows + RowToStructByName: %+v\n", users)
	return nil
}

func mapWithSqlx(ctx context.Context, db *sqlx.DB, out io.Writer) error {
	users, err := activeUsersSqlx(ctx, db)
	if err != nil {
		return fmt.Errorf("sqlx.SelectContext: %w", err)
	}
	fmt.Fprintf(out, "sqlx.SelectContext (pgx stdlib):     %+v\n", users)
	return nil
}

// bioStrategies scans Linus's NULL bio four ways (slide p06) and returns
// what each produced, plus the error from scanning into a plain string.
type bioResult struct {
	plainErr error
	pointer  *string
	null     sql.Null[string]
	text     pgtype.Text
	coalesce string
}

func scanBio(ctx context.Context, pool *pgxpool.Pool, email string) (bioResult, error) {
	const q = `SELECT bio FROM p055_users WHERE email = $1`
	var r bioResult

	var plain string
	r.plainErr = pool.QueryRow(ctx, q, email).Scan(&plain)

	if err := pool.QueryRow(ctx, q, email).Scan(&r.pointer); err != nil {
		return r, err
	}
	if err := pool.QueryRow(ctx, q, email).Scan(&r.null); err != nil {
		return r, err
	}
	if err := pool.QueryRow(ctx, q, email).Scan(&r.text); err != nil {
		return r, err
	}
	// Slide p06 renders this literal as `")`; it is two single quotes.
	err := pool.QueryRow(ctx, `SELECT COALESCE(bio, '') FROM p055_users WHERE email = $1`, email).Scan(&r.coalesce)
	return r, err
}

func nulls(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	r, err := scanBio(ctx, pool, "linus@example.com")
	if err != nil {
		return fmt.Errorf("scan bio: %w", err)
	}
	fmt.Fprintf(out, "plain string:      err = %v\n", r.plainErr)
	fmt.Fprintf(out, "*string:           nil = %t\n", r.pointer == nil)
	fmt.Fprintf(out, "sql.Null[string]:  Valid = %t, V = %q (Go 1.22 generic)\n", r.null.Valid, r.null.V)
	fmt.Fprintf(out, "pgtype.Text:       Valid = %t, String = %q\n", r.text.Valid, r.text.String)
	fmt.Fprintf(out, "COALESCE(bio, ''): %q (NULL and empty are now indistinguishable)\n", r.coalesce)

	ada, err := scanBio(ctx, pool, "ada@example.com")
	if err != nil {
		return fmt.Errorf("scan bio: %w", err)
	}
	fmt.Fprintf(out, "a non-NULL bio:    *string = %q, sql.Null = {%q %t}\n", *ada.pointer, ada.null.V, ada.null.Valid)
	return nil
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists)
	return exists, err
}

func injection(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	userInput := "1; DROP TABLE p055_injection_victim; --"

	// DANGEROUS: the input becomes part of the SQL text. With no arguments,
	// pgx sends Exec over the simple query protocol, which allows several
	// statements in one string, so the DROP runs.
	badQuery := fmt.Sprintf("SELECT first_name FROM p055_users WHERE user_id = %s", userInput)
	fmt.Fprintf(out, "Sprintf query: %s\n", badQuery)
	if _, err := pool.Exec(ctx, badQuery); err != nil {
		return fmt.Errorf("injected query: %w", err)
	}
	exists, err := tableExists(ctx, pool, "p055_injection_victim")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "after Sprintf:  p055_injection_victim exists = %t\n", exists)

	if _, err := pool.Exec(ctx, `CREATE TABLE p055_injection_victim (id int)`); err != nil {
		return err
	}

	// SECURE: the template and the value travel separately (extended
	// protocol, one statement only). Postgres uses $1; ? is MySQL/SQLite.
	var name string
	err = pool.QueryRow(ctx, `SELECT first_name FROM p055_users WHERE user_id = $1`, userInput).Scan(&name)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr):
		fmt.Fprintf(out, "with $1:        rejected as data, SQLSTATE %s (%s)\n", pgErr.Code, pgErr.Message)
	case err != nil:
		fmt.Fprintf(out, "with $1:        rejected as data: %v\n", err)
	default:
		fmt.Fprintf(out, "with $1:        unexpectedly found %q\n", name)
	}
	exists, err = tableExists(ctx, pool, "p055_injection_victim")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "after $1:       p055_injection_victim exists = %t\n", exists)
	return nil
}

func namedInserts(ctx context.Context, pool *pgxpool.Pool, db *sqlx.DB, out io.Writer) error {
	// sqlx: :name placeholders, filled from the struct's db tags and
	// rewritten to $1, $2, ... for Postgres.
	res, err := db.NamedExecContext(ctx, `
		INSERT INTO p055_users (first_name, last_name, email)
		VALUES (:first_name, :last_name, :email)`,
		NewUser{FirstName: "Barbara", LastName: "Liskov", Email: "barbara@example.com"})
	if err != nil {
		return fmt.Errorf("sqlx NamedExecContext: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "sqlx NamedExecContext: %d row inserted\n", n)

	// pgx: @name placeholders from a NamedArgs map; no extra dependency.
	tag, err := pool.Exec(ctx, `
		INSERT INTO p055_users (first_name, last_name, email)
		VALUES (@first_name, @last_name, @email)`,
		pgx.NamedArgs{"first_name": "Ken", "last_name": "Thompson", "email": "ken@example.com"})
	if err != nil {
		return fmt.Errorf("pgx NamedArgs: %w", err)
	}
	fmt.Fprintf(out, "pgx.NamedArgs:         %d row inserted\n", tag.RowsAffected())
	return nil
}

func bulkCopy(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	const n = 10_000
	start := time.Now()
	copied, err := pool.CopyFrom(ctx,
		pgx.Identifier{"p055_users"},
		[]string{"first_name", "last_name", "email"},
		pgx.CopyFromSlice(n, func(i int) ([]any, error) {
			return []any{"Bulk", fmt.Sprintf("User%05d", i), fmt.Sprintf("bulk%05d@example.com", i)}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("CopyFrom: %w", err)
	}
	elapsed := time.Since(start)

	var total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM p055_users`).Scan(&total); err != nil {
		return err
	}
	fmt.Fprintf(out, "CopyFrom: %d rows in one COPY (took %v)\n", copied, elapsed.Round(time.Millisecond))
	fmt.Fprintf(out, "p055_users now has %d rows\n", total)
	return nil
}

func section(w io.Writer, title string) {
	fmt.Fprintf(w, "\n== %s ==\n", title)
}
