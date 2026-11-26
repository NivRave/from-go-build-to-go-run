// Command no-enums-iota demonstrates Go's enum pattern: a named type, an
// iota const block, a bounds-safe String method, a state-machine switch,
// bitmask flags and the common iota tricks.
//
// Slides: slides/004-no-enums-iota.pdf
//
// Run:
//
//	go run ./examples/004-no-enums-iota
//	go test ./examples/004-no-enums-iota
package main

import "fmt"

func main() {
	section("1. OrderStatus with a bounds-safe String (slides p04, p05, p07)")
	statusValues()

	section("2. The type-safety limit (slide p04)")
	typeSafetyLimit()

	section("3. State machine with a default branch (slide p09)")
	stateMachine()

	section("4. Bitmask roles with 1 << iota (slide p08)")
	bitmask()

	section("5. iota tricks: skip, offset, sizes (slides p02, p06)")
	iotaTricks()
}

func statusValues() {
	for s := Unknown; s <= Delivered; s++ {
		fmt.Printf("%-10v = %d\n", s, s)
	}
	// The slide's version would panic here with "index out of range [42]".
	bad := OrderStatus(42)
	fmt.Printf("%v valid=%t\n", bad, bad.IsValid())
}

func typeSafetyLimit() {
	// An untyped constant converts implicitly, so this compiles even though
	// the slide says HandleOrder "strictly requires an OrderStatus".
	fmt.Println("HandleOrder(2):", HandleOrder(2))

	// A typed int variable does not convert implicitly:
	//
	//	n := 2
	//	HandleOrder(n) // compile error: cannot use n (variable of type int) as OrderStatus value in argument to HandleOrder
	//
	// but an explicit conversion always compiles, whatever the value.
	n := 99
	fmt.Println("HandleOrder(OrderStatus(99)):", HandleOrder(OrderStatus(n)))
}

func stateMachine() {
	for _, s := range []OrderStatus{Unknown, Pending, Processing, Shipped, Delivered, -1} {
		fmt.Println(HandleOrder(s))
	}
}

func bitmask() {
	fmt.Printf("Admin  = %d (%04b)\n", Admin, Admin)
	fmt.Printf("Editor = %d (%04b)\n", Editor, Editor)
	fmt.Printf("Viewer = %d (%04b)\n", Viewer, Viewer)

	perms := Admin | Viewer // combine with |
	fmt.Printf("Admin | Viewer      = %04b\n", perms)
	fmt.Printf("has Admin?  (&)     = %t\n", perms.Has(Admin))
	fmt.Printf("has Editor? (&)     = %t\n", perms.Has(Editor))

	perms &^= Admin // remove with &^ (AND NOT, "bit clear")
	fmt.Printf("after &^= Admin     = %04b, has Admin? %t\n", perms, perms.Has(Admin))
}

func iotaTricks() {
	const (
		_ = iota // Skip 0, so the zero value never means "A"
		A
		B
		C
	)
	fmt.Println("skip with _:      A, B, C =", A, B, C)

	const (
		Small  = iota + 10 // Starts at 10
		Medium             // 11
		Large              // 12
	)
	fmt.Println("offset iota + 10: Small, Medium, Large =", Small, Medium, Large)

	// iota is 1 on the KB line because the blank line before it consumed 0.
	const (
		_  = iota
		KB = 1 << (10 * iota) // 1 << 10
		MB                    // 1 << 20
		GB                    // 1 << 30
	)
	fmt.Println("sizes 1 << (10 * iota): KB, MB, GB =", KB, MB, GB)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
