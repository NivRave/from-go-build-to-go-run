package main

func Example_statusValues() {
	statusValues()
	// Output:
	// Unknown    = 0
	// Pending    = 1
	// Processing = 2
	// Shipped    = 3
	// Delivered  = 4
	// OrderStatus(42) valid=false
}

func Example_typeSafetyLimit() {
	typeSafetyLimit()
	// Output:
	// HandleOrder(2): processing: pack the box
	// HandleOrder(OrderStatus(99)): rejected: invalid status OrderStatus(99)
}

func Example_stateMachine() {
	stateMachine()
	// Output:
	// unknown status: something is wrong upstream, alert
	// pending: charge the card
	// processing: pack the box
	// shipped: send the tracking email
	// delivered: ask for a review
	// rejected: invalid status OrderStatus(-1)
}

func Example_bitmask() {
	bitmask()
	// Output:
	// Admin  = 1 (0001)
	// Editor = 2 (0010)
	// Viewer = 4 (0100)
	// Admin | Viewer      = 0101
	// has Admin?  (&)     = true
	// has Editor? (&)     = false
	// after &^= Admin     = 0100, has Admin? false
}

func Example_iotaTricks() {
	iotaTricks()
	// Output:
	// skip with _:      A, B, C = 1 2 3
	// offset iota + 10: Small, Medium, Large = 10 11 12
	// sizes 1 << (10 * iota): KB, MB, GB = 1024 1048576 1073741824
}
