package main

import "github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/infra"

func Example() {
	repo := infra.NewMemoryRepo()
	if err := seedUsers(repo); err != nil {
		panic(err)
	}
	if err := processOrders(repo); err != nil {
		panic(err)
	}
	// Output:
	// domain {ID:u1 Name:Ada Lovelace}  ->  stored {"id":"u1","full_name":"Ada Lovelace"}
	// domain {ID:u2 Name:Grace Hopper}  ->  stored {"id":"u2","full_name":"Grace Hopper"}
	// o1: err = <nil>
	// o2: err = process order o2: get user u9: not found
	//     errors.Is(err, domain.ErrNotFound): reply 404
	// o3: err = process order: invalid order
	//     errors.Is(err, domain.ErrInvalidOrder): reply 400
	// o4: err = process order o4: get user u2: context canceled
	//     errors.Is(err, context.Canceled): the caller's ctx reached the repository
	// saved orders: [{ID:o1 UserID:u1 AmountCents:4200}]
}
