package main

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFanOutDeliversACopyToEverySubscriber(t *testing.T) {
	b := NewBroker()
	a, c := b.Subscribe("orders"), b.Subscribe("orders")
	other := b.Subscribe("payments")

	for i := range 3 {
		if err := b.Publish(Event{ID: fmt.Sprint(i), Topic: "orders", Data: []byte("x")}); err != nil {
			t.Fatal(err)
		}
	}
	b.Close()

	// Subscriber a scribbles on its copy; c must not see it.
	var gotA, gotC, gotOther []Event
	for e := range a {
		e.Data[0] = 'A'
		gotA = append(gotA, e)
	}
	for e := range c {
		gotC = append(gotC, e)
	}
	for e := range other {
		gotOther = append(gotOther, e)
	}
	if len(gotA) != 3 || len(gotC) != 3 || len(gotOther) != 0 {
		t.Fatalf("got %d, %d, %d events; want 3, 3, 0", len(gotA), len(gotC), len(gotOther))
	}
	for i, e := range gotC {
		if e.ID != fmt.Sprint(i) || string(e.Data) != "x" {
			t.Errorf("subscriber c event %d = %q %q, want %q \"x\"", i, e.ID, e.Data, fmt.Sprint(i))
		}
	}
	if err := b.Publish(Event{Topic: "orders"}); !errors.Is(err, ErrBrokerClosed) {
		t.Errorf("Publish after Close = %v, want ErrBrokerClosed", err)
	}
}

// TestDedupConcurrentDuplicates delivers every event many times from many
// goroutines at once. Each must be processed exactly once. Run with -race.
func TestDedupConcurrentDuplicates(t *testing.T) {
	const events, copies = 20, 16
	var d Dedup
	var counts [events]atomic.Int32

	var wg sync.WaitGroup
	for i := range events {
		for range copies {
			wg.Go(func() {
				_, _ = d.Handle(fmt.Sprintf("evt-%d", i), func() error {
					counts[i].Add(1)
					return nil
				})
			})
		}
	}
	wg.Wait()

	for i := range counts {
		if n := counts[i].Load(); n != 1 {
			t.Errorf("evt-%d processed %d times, want 1", i, n)
		}
	}
}

func TestDedupRetriesAfterFailure(t *testing.T) {
	var d Dedup
	errBoom := errors.New("boom")
	if ran, err := d.Handle("e1", func() error { return errBoom }); !ran || !errors.Is(err, errBoom) {
		t.Fatalf("first delivery = (%t, %v), want (true, boom)", ran, err)
	}
	if ran, err := d.Handle("e1", func() error { return nil }); !ran || err != nil {
		t.Fatalf("redelivery after failure = (%t, %v), want (true, nil)", ran, err)
	}
	if ran, _ := d.Handle("e1", func() error { return nil }); ran {
		t.Fatal("third delivery ran again after a success")
	}
}

func TestOrderViewPending(t *testing.T) {
	order := func(id string, total int64) Event {
		return Event{Type: TypeOrderPlaced, Data: mustJSON(OrderPlaced{OrderID: id, Seq: 1, Payload: OrderData{TotalCents: total}})}
	}
	pay := func(id string, seq, amount int64) Event {
		return Event{Type: TypePaymentProcessed, Data: mustJSON(PaymentProcessed{OrderID: id, Seq: seq, AmountCents: amount})}
	}

	tests := []struct {
		name        string
		events      []Event
		wantPaid    int64
		wantKnown   bool
		wantPending int
	}{
		{"in order", []Event{order("o", 500), pay("o", 2, 500)}, 500, true, 0},
		{"payment first is parked then released", []Event{pay("o", 2, 500), order("o", 500)}, 500, true, 0},
		{"two parked payments are both released", []Event{pay("o", 2, 200), pay("o", 3, 300), order("o", 500)}, 500, true, 0},
		{"payment without its order stays parked", []Event{pay("o", 2, 500)}, 0, false, 1},
		{"other orders do not release it", []Event{pay("o", 2, 500), order("p", 100)}, 0, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := NewOrderView(func(string, ...any) {})
			for _, e := range tt.events {
				if err := v.Apply(e); err != nil {
					t.Fatal(err)
				}
			}
			paid, known := v.Paid("o")
			if paid != tt.wantPaid || known != tt.wantKnown || v.Pending() != tt.wantPending {
				t.Errorf("paid=%d known=%t pending=%d, want paid=%d known=%t pending=%d",
					paid, known, v.Pending(), tt.wantPaid, tt.wantKnown, tt.wantPending)
			}
		})
	}
}

func TestDecodeOrderV2(t *testing.T) {
	tests := []struct {
		name, json, wantCurrency string
	}{
		{"v1 event gets the default", `{"v":"1","id":"o","data":{"total_cents":1}}`, "USD"},
		{"v2 event keeps its currency", `{"v":"2","id":"o","data":{"total_cents":1,"currency":"EUR"}}`, "EUR"},
		{"unknown fields are ignored", `{"v":"3","id":"o","region":"eu","data":{"total_cents":1,"currency":"GBP","tax":5}}`, "GBP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o, err := decodeOrderV2([]byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			if o.Payload.Currency != tt.wantCurrency {
				t.Errorf("currency = %q, want %q", o.Payload.Currency, tt.wantCurrency)
			}
		})
	}
}

// Example tests keep the README's expected output honest. Section 1 is not
// here because its interleaving depends on timing.

func Example_versionedEvents() {
	versionedEvents()
	// Output:
	// v1 on the wire: {"v":"1","id":"ord-1","seq":1,"ts":"2026-11-26T09:00:00Z","data":{"customer":"ada","total_cents":4250}}
	// v2 consumer read v1: id=ord-1 total=$42.50 currency=USD (defaulted)
	// v2 on the wire: {"v":"2","id":"ord-2","seq":1,"ts":"2026-11-26T09:00:00Z","data":{"customer":"linus","total_cents":1000,"currency":"EUR"}}
	// v1 consumer read v2: id=ord-2 total=$10.00 (unknown field "currency" ignored)
}

func Example_atLeastOnce() {
	atLeastOnce()
	// Output:
	// evt-ord-1: charged
	// evt-ord-2: charged
	// evt-ord-2: duplicate skipped
	// evt-ord-3: charged
	// evt-ord-3: duplicate skipped
	// 5 deliveries, 3 charges
	// 8 concurrent deliveries of evt-ord-9: charged 1 time(s)
}

func Example_outOfOrder() {
	outOfOrder()
	// Output:
	// ord-7 seq=2: payment arrived before its order, parked (1 pending)
	// ord-7 seq=1: order placed (total $25.00)
	// ord-7 seq=2: releasing parked payment
	// ord-7 seq=2: paid $25.00 of $25.00
	// ord-8 seq=1: order placed (total $9.00)
	// ord-8 seq=2: paid $9.00 of $9.00
	// pending payments left: 0
}

func Example_addSubscriberAndShutdown() {
	addSubscriberAndShutdown()
	// Output:
	// publisher: published 3 orders
	// publisher: stopped: context canceled
	// email:     mailed ada, mailed linus, mailed grace
	// analytics: counted $42.50, counted $19.99, counted $1500.00
	// fraud:     ord-1 ok, ord-2 ok, ord-3 FLAGGED
	// all consumers finished
}
