# Part 056: Transactions & connection tuning

Day 28 · Week 4 · Ensuring data integrity and performance at scale.

## What this shows

- An `accounts(id bigint primary key, bal bigint not null check (bal >= 0))` table holding money as `int64` cents.
- `TransferFunds`, fixed from slide p03: both rows locked first in id order (`SELECT ... WHERE id = ANY($1) ORDER BY id FOR UPDATE`), `RowsAffected` checks, the balance checked by the `CHECK` constraint, and `pgx.BeginFunc` for commit or rollback.
- A deadlock demo: the naive version runs 1 to 2 and 2 to 1 transfers in 10 goroutines and Postgres aborts several with SQLSTATE `40P01`, caught with Go 1.26's `errors.AsType[*pgconn.PgError]`. The ordered version shows zero deadlocks.
- Write skew ("at least one doctor stays on call"): it breaks the rule under Read Committed, fails with `40001` under Serializable (`pgx.TxOptions{IsoLevel: pgx.Serializable}`), and a retry loop then re-runs the transaction correctly.
- Pool tuning (`MaxConnLifetime`, `MaxConnLifetimeJitter`, `MaxConnIdleTime`, `ConnectTimeout`) with a `pool.Stat()` printout, and slow-query logging through a `pgx.QueryTracer` that logs via `slog` above 200 ms, triggered by `SELECT pg_sleep(0.3)`.

## Run

This example needs Postgres. From the repository root:

```sh
docker compose up -d
export DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'
go run ./examples/056-transactions-and-tuning
go test ./examples/056-transactions-and-tuning
docker compose down
```

PowerShell: `$env:DATABASE_URL='postgres://gobuild:gobuild@localhost:5432/gobuild?sslmode=disable'`

Without `DATABASE_URL`, `go run` prints these instructions and exits with status 1, and the database tests skip. Every table is prefixed `p056_`, created at the start and dropped at the end. CI's `integration` job runs this example against a Postgres 18 service container.

## What to look for

Expected output of `go run` (the deadlock count, the timings and the pool statistics vary from run to run):

```text
== 1. Accounts in int64 cents, CHECK (bal >= 0) (slide p03) ==
account 1: $100.00
account 2: $50.00
account 3: $25.00
account 4: $0.00
total:     $175.00

== 2. TransferFunds: lock in id order, check rows and balance (slide p03) ==
transfer 1 -> 2    $25.50: err = <nil>
transfer 4 -> 1     $1.00: err = transfer 4 -> 1: insufficient funds
transfer 1 -> 99    $1.00: err = account not found
transfer 2 -> 2     $1.00: err = cannot transfer to the same account
transfer 3 -> 1    -$5.00: err = amount must be positive
account 1: $74.50
account 2: $75.50
account 3: $25.00
account 4: $0.00
total:     $175.00

== 3. Deadlocks: opposite transfers, naive vs ordered (slide p07) ==
naive:   10 transfers, 9 aborted with SQLSTATE 40P01 (deadlock detected) after 2.1s
ordered: 10 transfers, 0 deadlocks after 0s
total balance is still $175.00: a deadlock victim is rolled back as a whole

== 4. Isolation: write skew under Read Committed vs Serializable (slide p04) ==
read committed: alice: committed; bob: committed; doctors on call: 0
serializable:   alice: committed; bob: aborted, SQLSTATE 40001; doctors on call: 1
bob retried under serializable: attempt 1: refused: at least one doctor must stay on call

== 5. Pool tuning and slow-query logging (slides p05, p06, p08) ==
config: MaxConns=20 MinConns=2 MaxConnLifetime=30m0s (+ up to 5m0s jitter) MaxConnIdleTime=5m0s ConnectTimeout=5s
stat:   total=10 idle=10 acquired=0 max=20 acquires=39 new-conns=10 waited-for-conn=8
slow-query tracer on, threshold 200ms
level=WARN msg="slow query" duration=301ms sql="SELECT pg_sleep(0.3)" err=<nil>

cleanup: dropped the p056_ tables
```

- The naive section takes a couple of seconds because Postgres checks for deadlocks only after a statement has waited `deadlock_timeout` (1 s by default). The ordered version finishes at once: the second transfer simply waits for the first one's row lock.
- Every failed transfer in section 2 leaves the balances untouched, and the total stays the same throughout: the transaction is all or nothing.
- Under Read Committed both doctors read "2 on call", both leave, and nobody is on call. Serializable detects the read/write dependency and aborts one transaction. The retry re-reads the data, sees that alice already left, and refuses.
- `waited-for-conn` counts acquisitions that found no idle connection, which happened during the burst of concurrent transfers.
- The tracer is switched on only for section 5. Left on, it would also log every statement that waited on a row lock in section 3, and in production you want exactly that: lock waits are slow queries too.

## Notes and corrections

- **Slide p03:** `TransferFunds` itself deadlocks when two transfers run in opposite directions, because each locks the rows in the order it names them. Lock the rows in a consistent order first.
- **Slide p03:** `amount float64` for money is a bug, since binary floating point cannot represent most decimal amounts exactly. Use integer cents (`int64`) or `NUMERIC` with a decimal type.
- **Slide p03:** an `UPDATE` that matches no row succeeds silently, so check `RowsAffected`, and enforce non-negative balances (here with `CHECK (bal >= 0)`). `pgx.BeginFunc` replaces the `Begin`/`defer Rollback`/`Commit` boilerplate.
- **Slide p07:** keeping tables in the same order is not enough: deadlocks also happen between rows of one table, as section 3 shows. Lock rows in a consistent order and retry on `40P01`.
- **Slide p04:** in Postgres, Read Uncommitted behaves as Read Committed (there are no dirty reads), and Repeatable Read already prevents phantom reads. Serializable adds protection against anomalies such as write skew. Its cost is `40001` serialization failures that the application must retry, not extra locks.
- **Slide p05:** the `(cores * 2) + spindles` formula refers to the database server's cores and is the total across all app instances, not per instance. PgBouncer in transaction mode conflicts with pgx's default statement caching unless you switch pgx to the simple protocol or use PgBouncer 1.21+ prepared-statement support.
- **Slide p06:** both errors are ignored (`config, _ :=` and `pool, _ :=`), so a bad DSN panics later. The defaults, for comparison: `MaxConnLifetime` 1 h, `MaxConnIdleTime` 30 min, `HealthCheckPeriod` 1 min. `MaxConnLifetimeJitter` keeps all connections from reconnecting at the same moment.
- **Slide p08:** pgx v5's hook is `ConnConfig.Tracer` (a `pgx.QueryTracer` with `TraceQueryStart` and `TraceQueryEnd`). It observes queries but cannot rewrite the SQL, so trace-ID comments need a wrapper around the query calls. On the server side, `log_min_duration_statement = 200` does the same logging.

## Slides

[056-transactions-and-tuning.pdf](../../slides/056-transactions-and-tuning.pdf)
