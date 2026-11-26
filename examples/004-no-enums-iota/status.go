package main

import "strconv"

// OrderStatus is the slide's enum: a named integer type plus a const block.
// The type gives you a name to hang methods on and stops an int *variable*
// from being passed by accident, but it does not make the set closed: any
// untyped constant or explicit conversion produces an OrderStatus.
type OrderStatus int

const (
	Unknown    OrderStatus = iota // 0, the zero value: catches uninitialised data (slide p05)
	Pending                       // 1: an omitted expression repeats "OrderStatus = iota"
	Processing                    // 2
	Shipped                       // 3
	Delivered                     // 4
)

var orderStatusNames = [...]string{"Unknown", "Pending", "Processing", "Shipped", "Delivered"}

// String makes logs say "Shipped" instead of "3" (slide p07). Unlike the
// slide's [...]string{...}[s], it bounds-checks first, so an out-of-range
// value (a corrupted DB row, OrderStatus(42)) prints as "OrderStatus(42)"
// instead of panicking. This is the same fallback `stringer` generates.
func (s OrderStatus) String() string {
	if s < 0 || int(s) >= len(orderStatusNames) {
		return "OrderStatus(" + strconv.Itoa(int(s)) + ")"
	}
	return orderStatusNames[s]
}

// IsValid reports whether s is one of the declared constants. Go enums are
// open sets, so validate values that come from outside the program.
func (s OrderStatus) IsValid() bool {
	return s >= Unknown && s <= Delivered
}

// HandleOrder is the slide p09 state machine, completed: every declared
// state has a case, and default rejects anything else instead of silently
// doing nothing.
func HandleOrder(status OrderStatus) string {
	switch status {
	case Unknown:
		return "unknown status: something is wrong upstream, alert"
	case Pending:
		return "pending: charge the card"
	case Processing:
		return "processing: pack the box"
	case Shipped:
		return "shipped: send the tracking email"
	case Delivered:
		return "delivered: ask for a review"
	default:
		return "rejected: invalid status " + status.String()
	}
}
