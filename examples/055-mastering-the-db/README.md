# Part 055: Mastering the database: Postgres in Go

Day 28 · Week 4 · High-performance relational persistence with pgx and sqlx.

## What this shows

- A `pgxpool` set up properly: `pgxpool.ParseConfig` with its error checked, `MaxConns`/`MinConns`, `NewWithConfig`, `Ping`, and `defer pool.Close()`.
- Struct mapping two ways: pgx alone with `pgx.CollectRows(rows, pgx.RowToStructByName[User])`, and sqlx with `sqlx.ConnectContext(ctx, "pgx", dsn)` over pgx's `stdlib` driver and `SelectContext`.
- NULLs: a nullable `bio` scanned into `*string`, `sql.Null[string]` (Go 1.22), `pgtype.Text` and `COALESCE(bio, '')`, plus the error you get when scanning NULL into a plain `string`.
- SQL injection: a `fmt.Sprintf` query with malicious input drops a throwaway table, while the same input as a `$1` parameter is rejected as data.
- Named inserts with sqlx `NamedExecContext` (`:first_name`, using `db:` tags) and with `pgx.NamedArgs` (`@first_name`).
- Bulk loading 10,000 rows in one `COPY` with `CopyFrom`.

## Run

This example needs Postgres. From the repository root:

```sh
docker compose up -d
export DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'
go run ./examples/055-mastering-the-db
go test ./examples/055-mastering-the-db
docker compose down
```

PowerShell: `$env:DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'`

Without `DATABASE_URL`, `go run` prints these instructions and exits with status 1, and the tests skip. Every table is prefixed `p055_`, created at the start and dropped at the end. CI's `integration` job runs this example against a Postgres 18 service container.

## What to look for

Expected output of `go run` (the `COPY` time varies):

```text
== 1. A checked connection pool (slide p05) ==
pool ready: MaxConns=10 MinConns=2, ping ok

== 2. Struct mapping two ways (slide p04) ==
pgx.CollectRows + RowToStructByName: [{ID:1 Email:ada@example.com} {ID:2 Email:linus@example.com}]
sqlx.SelectContext (pgx stdlib):     [{ID:1 Email:ada@example.com} {ID:2 Email:linus@example.com}]

== 3. NULLs (slide p06) ==
plain string:      err = can't scan into dest[0] (col: bio): cannot scan NULL into *string
*string:           nil = true
sql.Null[string]:  Valid = false, V = "" (Go 1.22 generic)
pgtype.Text:       Valid = false, String = ""
COALESCE(bio, ''): "" (NULL and empty are now indistinguishable)
a non-NULL bio:    *string = "First programmer", sql.Null = {"First programmer" true}

== 4. SQL injection vs parameters (slide p07) ==
Sprintf query: SELECT first_name FROM p055_users WHERE user_id = 1; DROP TABLE p055_injection_victim; --
after Sprintf:  p055_injection_victim exists = false
with $1:        rejected as data, SQLSTATE 22P02 (invalid input syntax for type bigint: "1; DROP TABLE p055_injection_victim; --")
after $1:       p055_injection_victim exists = true

== 5. Named inserts (slide p08) ==
sqlx NamedExecContext: 1 row inserted
pgx.NamedArgs:         1 row inserted

== 6. Bulk insert with COPY (slide p03) ==
CopyFrom: 10000 rows in one COPY (took 60ms)
p055_users now has 10005 rows

cleanup: dropped the p055_ tables
```

- Both mapping styles return the same structs from the same `db:` tags. pgx v5 alone now covers what sqlx was usually added for.
- `COALESCE` is the only strategy that cannot tell NULL from an empty string; the other three keep the difference.
- The injected `DROP TABLE` really runs: with no arguments, pgx sends `Exec` over the simple query protocol, which accepts several statements in one string. With `$1`, the value travels separately from the SQL, and Postgres rejects it as an invalid `bigint`.
- `CopyFrom` sends all rows in a single `COPY` stream instead of 10,000 `INSERT` round trips.

## Notes and corrections

- **Slide p05:** `config, _ := pgxpool.ParseConfig(...)` ignores the error, so a bad DSN leads to a nil-pointer panic. `NewWithConfig` doesn't connect eagerly, so follow it with `Ping`, and `defer pool.Close()`.
- **Slides p04/p08 vs p05/p07:** the slides mix sqlx calls (`db.Select`, `db.NamedExec`) with `pgxpool` calls (`db.Exec(ctx, ...)`), but sqlx works only through `database/sql`, here with pgx's `stdlib` driver, not on a `*pgxpool.Pool`. pgx v5 also covers both features natively with `RowToStructByName` and `NamedArgs`. Prefer the context-taking sqlx methods (`SelectContext`, `NamedExecContext`).
- **Slide p03:** the standard library has no Postgres driver, so there is no "standard driver" to compare with. `database/sql` needs one, such as `lib/pq` (in maintenance mode) or pgx's own `stdlib` adapter.
- **Slide p06:** prefer the generic `sql.Null[T]` (Go 1.22) over `sql.NullString`. pgx also scans NULL into pointers and `pgtype` types.
- **Slide p06:** the `COALESCE(column, ")` literal is two single quotes, `''`.
- **Slide p07:** Postgres uses `$1` placeholders; `?` is MySQL and SQLite syntax (sqlx's `Rebind` converts). The injection works because pgx uses the simple protocol when there are no arguments. pgx also prepares and caches statements automatically, which is the "pre-compile" benefit the slide mentions.
- **Slide p08:** `NamedExec` maps `:first_name` to a field only if the struct has `db:"first_name"` tags; without one, sqlx looks for the lower-cased field name, `firstname`.
- **Slide p05:** size the pool against the server's `max_connections` across *all* instances: 50 instances with 100 connections each would need 5,000.

## Slides

[055-mastering-the-db.pdf](../../slides/055-mastering-the-db.pdf)
