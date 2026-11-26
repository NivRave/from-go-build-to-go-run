# Part 048: Clean architecture & the art of mockability

Day 24 · Week 4 · Structuring your Go code for a decade of change.

## What this shows

- `domain`: the entities (`User`, `Order`) and the domain errors `ErrNotFound` and `ErrInvalidOrder`. It imports nothing from the other layers.
- `usecase`: `ProcessOrder(ctx, order, users, orders)`, written against two one-method interfaces it defines itself, `UserGetter` and `OrderSaver`. Every call takes a `ctx`.
- `infra`: an in-memory `MemoryRepo` that stores a separate `dbUser`/`dbOrder` model with its own JSON tags, maps it to and from the domain at the boundary, and turns "missing" into `domain.ErrNotFound`.
- `main` wires `infra` into `usecase` and runs four orders: one succeeds, one has an unknown user, one is invalid, and one arrives with an already-cancelled context.
- `usecase/orders_test.go`: hand-rolled function-field mocks with a nil guard, simulating not-found and `context.DeadlineExceeded` in a table-driven test asserted with `errors.Is`.

## Run

```sh
go run ./examples/048-clean-architecture
go test ./examples/048-clean-architecture/...
```

## What to look for

Expected output of `go run`:

```text
== 1. Domain entities and the storage model (slides p03, p06) ==
domain {ID:u1 Name:Ada Lovelace}  ->  stored {"id":"u1","full_name":"Ada Lovelace"}
domain {ID:u2 Name:Grace Hopper}  ->  stored {"id":"u2","full_name":"Grace Hopper"}

== 2. The use case runs against the wired infrastructure (slides p02, p07-p08) ==
o1: err = <nil>
o2: err = process order o2: get user u9: not found
    errors.Is(err, domain.ErrNotFound): reply 404
o3: err = process order: invalid order
    errors.Is(err, domain.ErrInvalidOrder): reply 400
o4: err = process order o4: get user u2: context canceled
    errors.Is(err, context.Canceled): the caller's ctx reached the repository
saved orders: [{ID:o1 UserID:u1 AmountCents:4200}]
```

- The domain's `Name` is stored as `full_name`. Only `infra` knows that; renaming the column would touch one file.
- `o2` fails with the domain's `ErrNotFound`, not a storage error, so a handler can map it to a 404 without importing `infra`.
- `o4` shows why `ProcessOrder` takes a `ctx`: the cancelled context reaches the repository and the order isn't saved. With the slide's `context.Background()`, the save would have gone ahead.
- Dependencies point inward: `main` imports all three packages, `infra` and `usecase` import `domain`, and `domain` imports nothing from the other layers. `go list -f '{{.Imports}}' ./examples/048-clean-architecture/usecase` (or `domain`, `infra`) confirms it.

## Notes and corrections

- **Slide p07:** `ProcessOrder` calls `SaveOrder(context.Background(), o)`, which detaches the save from the caller's cancellation, deadline and trace. Take `ctx context.Context` as the first parameter and pass it on.
- **Slide p06:** with a **string** id, GORM's `First(&u, id)` is documented as an SQL injection risk; use `First(&u, "id = ?", id)`, or a placeholder with `database/sql` as shown in a comment in `infra/memory.go`.
- **Slide p06:** map "not found" to a domain error (for example `errors.Is(err, gorm.ErrRecordNotFound)` → `domain.ErrNotFound`) so the core stays storage-agnostic. The method also needs a `ctx`.
- **Slide p06:** the concrete `*UserRepository` has the same name as the `UserRepository` interface from p05, which can't coexist in one package. Here the concrete type is `MemoryRepo`.
- **Slide p05:** `Get` panics with a nil function call if `OnGet` was never set. The mocks here check for nil and return an "unexpected call" error instead.
- **Slide p03:** the diagram mixes Onion, Hexagonal and Clean Architecture terms. That's a reasonable simplification; the shared idea is that dependencies point inward.

## Slides

[048-clean-architecture.pdf](../../slides/048-clean-architecture.pdf)
