// Command clean-architecture demonstrates a layered Go service: domain
// entities and errors at the centre, a use case that depends only on small
// interfaces it defines, and an infrastructure repository that maps its own
// storage model to the domain at the boundary. main wires them together.
//
// Slides: slides/048-clean-architecture.pdf
//
// Run:
//
//	go run ./examples/048-clean-architecture
//	go test ./examples/048-clean-architecture/...
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/domain"
	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/infra"
	"github.com/NivRave/from-go-build-to-go-run/examples/048-clean-architecture/usecase"
)

func main() {
	repo := infra.NewMemoryRepo()

	section("1. Domain entities and the storage model (slides p03, p06)")
	if err := seedUsers(repo); err != nil {
		log.Fatal(err)
	}

	section("2. The use case runs against the wired infrastructure (slides p02, p07-p08)")
	if err := processOrders(repo); err != nil {
		log.Fatal(err)
	}
}

// seedUsers adds domain users through the repository and shows the shape
// they are stored in, which the domain never sees.
func seedUsers(repo *infra.MemoryRepo) error {
	for _, u := range []domain.User{{ID: "u1", Name: "Ada Lovelace"}, {ID: "u2", Name: "Grace Hopper"}} {
		if err := repo.AddUser(u); err != nil {
			return err
		}
		fmt.Printf("domain %+v  ->  stored %s\n", u, repo.RawUser(u.ID))
	}
	return nil
}

// processOrders is the composition root at work: the same *infra.MemoryRepo
// is passed as both the UserGetter and the OrderSaver. The use case only
// knows the interfaces.
func processOrders(repo *infra.MemoryRepo) error {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel() // a request whose client already went away

	cases := []struct {
		ctx   context.Context
		order *domain.Order
	}{
		{context.Background(), &domain.Order{ID: "o1", UserID: "u1", AmountCents: 4200}},
		{context.Background(), &domain.Order{ID: "o2", UserID: "u9", AmountCents: 100}},
		{context.Background(), &domain.Order{ID: "o3", UserID: "u2", AmountCents: 0}},
		{cancelled, &domain.Order{ID: "o4", UserID: "u2", AmountCents: 999}},
	}
	for _, c := range cases {
		err := usecase.ProcessOrder(c.ctx, c.order, repo, repo)
		fmt.Printf("%s: err = %v\n", c.order.ID, err)
		switch {
		case err == nil:
		case errors.Is(err, domain.ErrNotFound):
			fmt.Println("    errors.Is(err, domain.ErrNotFound): reply 404")
		case errors.Is(err, domain.ErrInvalidOrder):
			fmt.Println("    errors.Is(err, domain.ErrInvalidOrder): reply 400")
		case errors.Is(err, context.Canceled):
			fmt.Println("    errors.Is(err, context.Canceled): the caller's ctx reached the repository")
		}
	}

	saved, err := repo.Orders()
	if err != nil {
		return err
	}
	fmt.Printf("saved orders: %+v\n", saved)
	return nil
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
