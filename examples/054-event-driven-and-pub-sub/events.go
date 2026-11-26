package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Event types carried in Event.Type.
const (
	TypeOrderPlaced      = "OrderPlaced"
	TypePaymentProcessed = "PaymentProcessed"
)

// OrderPlaced is a "fat" event (slide p05): it carries the order itself, so
// subscribers do not have to call the order service back. Version lets the
// schema evolve; Seq orders the events that belong to one order.
type OrderPlaced struct {
	Version   string    `json:"v"`
	OrderID   string    `json:"id"`
	Seq       int64     `json:"seq"`
	Timestamp time.Time `json:"ts"`
	Payload   OrderData `json:"data"` // the "fat" part
}

// OrderData is the payload of a v1 OrderPlaced event.
type OrderData struct {
	Customer   string `json:"customer"`
	TotalCents int64  `json:"total_cents"`
}

// OrderPlacedV2 is the next version of the event. v2 adds a currency; a v2
// consumer must still accept v1 events, which have none.
type OrderPlacedV2 struct {
	Version   string      `json:"v"`
	OrderID   string      `json:"id"`
	Seq       int64       `json:"seq"`
	Timestamp time.Time   `json:"ts"`
	Payload   OrderDataV2 `json:"data"`
}

// OrderDataV2 is the payload of a v2 OrderPlaced event.
type OrderDataV2 struct {
	Customer   string `json:"customer"`
	TotalCents int64  `json:"total_cents"`
	Currency   string `json:"currency"`
}

// PaymentProcessed says an order was paid. Seq 2 follows the order's Seq 1.
type PaymentProcessed struct {
	Version     string `json:"v"`
	OrderID     string `json:"id"`
	Seq         int64  `json:"seq"`
	AmountCents int64  `json:"amount_cents"`
}

// mustJSON marshals event payloads built by this program, which cannot fail.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// decodeOrderV2 is what a v2 consumer does with any OrderPlaced event.
// encoding/json ignores unknown fields by default, and fields a v1 producer
// did not send stay at their zero value, so the consumer fills in defaults.
func decodeOrderV2(data []byte) (OrderPlacedV2, error) {
	var o OrderPlacedV2
	if err := json.Unmarshal(data, &o); err != nil {
		return o, err
	}
	if o.Payload.Currency == "" {
		o.Payload.Currency = "USD" // v1 events predate multi-currency
	}
	return o, nil
}

// Dedup is an idempotent event handler (slide p07, done safely). The check
// and the mark are one atomic step: LoadOrStore either stores the ID and
// reports loaded=false (we own the event) or reports loaded=true (someone
// already did). Two concurrent deliveries can never both win.
//
// In a real service the dedup record must be written atomically with the
// side effect, for example in the same transaction as the work, or with
// INSERT ... ON CONFLICT DO NOTHING on a processed_events primary key, and
// the work itself should be idempotent too.
type Dedup struct {
	seen sync.Map
}

// Handle runs work once per event ID. It reports whether work ran.
func (d *Dedup) Handle(id string, work func() error) (bool, error) {
	if _, loaded := d.seen.LoadOrStore(id, struct{}{}); loaded {
		return false, nil
	}
	if err := work(); err != nil {
		d.seen.Delete(id) // let a redelivery retry
		return true, err
	}
	return true, nil
}

// naiveDedup is slide p07's version: check, do the work, then mark. Each
// step is locked, so there is no data race and go run -race stays quiet,
// but the sequence is not atomic. Concurrent deliveries all pass the check
// before any of them marks the event.
type naiveDedup struct {
	mu        sync.Mutex
	processed map[string]bool
}

func (n *naiveDedup) alreadyProcessed(id string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.processed[id]
}

func (n *naiveDedup) markAsProcessed(id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.processed[id] = true
	return nil
}

func (n *naiveDedup) HandleEvent(e Event, work func()) error {
	if n.alreadyProcessed(e.ID) {
		return nil // Ignore duplicate
	}
	work() // Do the work...
	return n.markAsProcessed(e.ID)
}

// OrderView is a read model fed by order events (slides p06, p08). Events may
// arrive out of order, so a payment for an order it has not seen yet is
// parked in pending until the order arrives.
type OrderView struct {
	orders  map[string]*orderState
	pending map[string][]PaymentProcessed
	log     func(format string, args ...any)
}

type orderState struct {
	placed OrderPlaced
	paid   int64
}

// NewOrderView returns an empty read model that reports what it does via log.
func NewOrderView(log func(format string, args ...any)) *OrderView {
	return &OrderView{
		orders:  make(map[string]*orderState),
		pending: make(map[string][]PaymentProcessed),
		log:     log,
	}
}

// Apply updates the view with one event.
func (v *OrderView) Apply(e Event) error {
	switch e.Type {
	case TypeOrderPlaced:
		var o OrderPlaced
		if err := json.Unmarshal(e.Data, &o); err != nil {
			return err
		}
		st := &orderState{placed: o}
		v.orders[o.OrderID] = st
		v.log("%s seq=%d: order placed (total %s)", o.OrderID, o.Seq, cents(o.Payload.TotalCents))
		parked := v.pending[o.OrderID]
		delete(v.pending, o.OrderID)
		for _, p := range parked {
			v.log("%s seq=%d: releasing parked payment", p.OrderID, p.Seq)
			v.pay(st, p)
		}
	case TypePaymentProcessed:
		var p PaymentProcessed
		if err := json.Unmarshal(e.Data, &p); err != nil {
			return err
		}
		st, ok := v.orders[p.OrderID]
		if !ok {
			v.pending[p.OrderID] = append(v.pending[p.OrderID], p)
			v.log("%s seq=%d: payment arrived before its order, parked (%d pending)", p.OrderID, p.Seq, v.Pending())
			return nil
		}
		v.pay(st, p)
	default:
		return fmt.Errorf("unknown event type %q", e.Type)
	}
	return nil
}

func (v *OrderView) pay(st *orderState, p PaymentProcessed) {
	st.paid += p.AmountCents
	v.log("%s seq=%d: paid %s of %s", p.OrderID, p.Seq, cents(st.paid), cents(st.placed.Payload.TotalCents))
}

// Pending reports how many payments are parked.
func (v *OrderView) Pending() int {
	n := 0
	for _, ps := range v.pending {
		n += len(ps)
	}
	return n
}

// Paid reports the amount paid for an order the view knows about.
func (v *OrderView) Paid(orderID string) (int64, bool) {
	st, ok := v.orders[orderID]
	if !ok {
		return 0, false
	}
	return st.paid, true
}

func cents(c int64) string { return fmt.Sprintf("$%d.%02d", c/100, c%100) }
