// Command event-driven-and-pub-sub demonstrates an in-process pub/sub broker:
// fan-out to subscribers that consume at their own pace, versioned JSON
// events, idempotent handling of at-least-once delivery, out-of-order events,
// and adding a subscriber without touching the publisher.
//
// Slides: slides/054-event-driven-and-pub-sub.pdf
//
// Run:
//
//	go run ./examples/054-event-driven-and-pub-sub
//	go run ./examples/054-event-driven-and-pub-sub -race-demo
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	raceDemo := flag.Bool("race-demo", false, "run slide p07's check-then-act handler with concurrent duplicate deliveries")
	flag.Parse()

	if *raceDemo {
		section("Slide p07's check-then-act handler under concurrent redelivery")
		checkThenActRace()
		return
	}

	section("1. Fan-out: fast and slow subscribers (slide p03)")
	fanOut()

	section("2. Versioned fat events (slide p05)")
	versionedEvents()

	section("3. At-least-once delivery needs idempotent handlers (slide p07)")
	atLeastOnce()

	section("4. Out-of-order events: park until the order arrives (slide p06)")
	outOfOrder()

	section("5. Add a Fraud subscriber, then shut down gracefully (slide p03)")
	addSubscriberAndShutdown()
}

// demoTime keeps the JSON in the examples identical on every run.
var demoTime = time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)

func sampleOrders() []OrderPlaced {
	totals := []int64{4250, 1999, 150000}
	customers := []string{"ada", "linus", "grace"}
	orders := make([]OrderPlaced, len(totals))
	for i := range totals {
		orders[i] = OrderPlaced{
			Version:   "1",
			OrderID:   fmt.Sprintf("ord-%d", i+1),
			Seq:       1,
			Timestamp: demoTime,
			Payload:   OrderData{Customer: customers[i], TotalCents: totals[i]},
		}
	}
	return orders
}

// publishOrders is the order service. It knows the topic, not who listens:
// sections 1 and 5 use it unchanged with different subscribers.
func publishOrders(ctx context.Context, b *Broker, orders []OrderPlaced) (int, error) {
	for i, o := range orders {
		if err := ctx.Err(); err != nil {
			return i, err
		}
		e := Event{ID: "evt-" + o.OrderID, Topic: "orders", Type: TypeOrderPlaced, Data: mustJSON(o)}
		if err := b.Publish(e); err != nil {
			return i, err
		}
	}
	return len(orders), nil
}

func fanOut() {
	b := NewBroker()
	fast, slow := b.Subscribe("orders"), b.Subscribe("orders")

	var fastDone, slowDone sync.WaitGroup
	var fastN, slowN atomic.Int64
	fastDone.Go(func() {
		for range fast {
			fastN.Add(1)
		}
	})
	slowDone.Go(func() {
		for range slow {
			time.Sleep(20 * time.Millisecond) // a slow downstream, e.g. analytics
			slowN.Add(1)
		}
	})

	orders := append(sampleOrders(), sampleOrders()...) // 6 orders
	n, _ := publishOrders(context.Background(), b, orders)
	fmt.Printf("publisher: published %d orders and moved on; slow has processed %d\n", n, slowN.Load())

	// Close lets each subscription drain its own queue, then closes its
	// channel; both consumers then leave their range loops.
	b.Close()
	fastDone.Wait()
	fmt.Printf("fast: processed %d\n", fastN.Load())
	slowDone.Wait()
	fmt.Printf("slow: processed %d (20ms each, from its own queue)\n", slowN.Load())
}

func versionedEvents() {
	v1 := OrderPlaced{Version: "1", OrderID: "ord-1", Seq: 1, Timestamp: demoTime,
		Payload: OrderData{Customer: "ada", TotalCents: 4250}}
	wire := mustJSON(v1)
	fmt.Printf("v1 on the wire: %s\n", wire)

	o, err := decodeOrderV2(wire)
	if err != nil {
		fmt.Println("decode:", err)
		return
	}
	fmt.Printf("v2 consumer read v1: id=%s total=%s currency=%s (defaulted)\n",
		o.OrderID, cents(o.Payload.TotalCents), o.Payload.Currency)

	v2 := OrderPlacedV2{Version: "2", OrderID: "ord-2", Seq: 1, Timestamp: demoTime,
		Payload: OrderDataV2{Customer: "linus", TotalCents: 1000, Currency: "EUR"}}
	wire = mustJSON(v2)
	fmt.Printf("v2 on the wire: %s\n", wire)

	// An old consumer must tolerate fields it does not know. encoding/json
	// ignores them unless you opt into Decoder.DisallowUnknownFields, which
	// would break every consumer the day a producer adds a field.
	var old OrderPlaced
	if err := json.Unmarshal(wire, &old); err != nil {
		fmt.Println("decode:", err)
		return
	}
	fmt.Printf("v1 consumer read v2: id=%s total=%s (unknown field \"currency\" ignored)\n",
		old.OrderID, cents(old.Payload.TotalCents))
}

func atLeastOnce() {
	b := NewBroker()
	sub := b.Subscribe("orders")

	// The broker redelivers evt-ord-2 and evt-ord-3, as a real one does when
	// an acknowledgement is lost.
	orders := sampleOrders()
	for _, i := range []int{0, 1, 1, 2, 2} {
		o := orders[i]
		_ = b.Publish(Event{ID: "evt-" + o.OrderID, Topic: "orders", Type: TypeOrderPlaced, Data: mustJSON(o)})
	}
	b.Close()

	var d Dedup
	charged, deliveries := 0, 0
	for e := range sub {
		deliveries++
		ran, err := d.Handle(e.ID, func() error { charged++; return nil })
		switch {
		case err != nil:
			fmt.Printf("%s: failed: %v\n", e.ID, err)
		case ran:
			fmt.Printf("%s: charged\n", e.ID)
		default:
			fmt.Printf("%s: duplicate skipped\n", e.ID)
		}
	}
	fmt.Printf("%d deliveries, %d charges\n", deliveries, charged)

	// Redeliveries can also arrive at the same time on different consumers.
	// LoadOrStore makes the check and the mark one atomic step.
	var wg sync.WaitGroup
	var concurrentCharges atomic.Int64
	for range 8 {
		wg.Go(func() {
			_, _ = d.Handle("evt-ord-9", func() error { concurrentCharges.Add(1); return nil })
		})
	}
	wg.Wait()
	fmt.Printf("8 concurrent deliveries of evt-ord-9: charged %d time(s)\n", concurrentCharges.Load())
}

// checkThenActRace runs slide p07's HandleEvent with concurrent deliveries of
// one event. Every delivery passes alreadyProcessed before any of them
// reaches markAsProcessed, so the customer is charged several times. Each
// step is locked, so go run -race reports nothing: the race detector finds
// unsynchronised memory access, not check-then-act logic races.
func checkThenActRace() {
	n := &naiveDedup{processed: make(map[string]bool)}
	e := Event{ID: "evt-ord-9", Topic: "orders", Type: TypeOrderPlaced}

	var wg sync.WaitGroup
	var charges atomic.Int64
	for range 8 {
		wg.Go(func() {
			_ = n.HandleEvent(e, func() {
				time.Sleep(10 * time.Millisecond) // the work: call the payment API
				charges.Add(1)
			})
		})
	}
	wg.Wait()
	fmt.Printf("8 concurrent deliveries of %s: charged %d time(s)\n", e.ID, charges.Load())
	fmt.Println("the fix is Dedup.Handle: sync.Map.LoadOrStore checks and marks in one step")
}

func outOfOrder() {
	b := NewBroker()
	sub := b.Subscribe("orders")

	// PaymentProcessed (seq 2) for ord-7 overtakes its OrderPlaced (seq 1),
	// for example because they went through different partitions.
	pay := func(id string, amount int64) Event {
		return Event{ID: "pay-" + id, Topic: "orders", Type: TypePaymentProcessed,
			Data: mustJSON(PaymentProcessed{Version: "1", OrderID: id, Seq: 2, AmountCents: amount})}
	}
	order := func(id string, total int64) Event {
		return Event{ID: "evt-" + id, Topic: "orders", Type: TypeOrderPlaced,
			Data: mustJSON(OrderPlaced{Version: "1", OrderID: id, Seq: 1, Timestamp: demoTime,
				Payload: OrderData{Customer: "ada", TotalCents: total}})}
	}
	for _, e := range []Event{pay("ord-7", 2500), order("ord-7", 2500), order("ord-8", 900), pay("ord-8", 900)} {
		_ = b.Publish(e)
	}
	b.Close()

	view := NewOrderView(func(format string, args ...any) { fmt.Printf(format+"\n", args...) })
	for e := range sub {
		if err := view.Apply(e); err != nil {
			fmt.Println("apply:", err)
		}
	}
	fmt.Printf("pending payments left: %d\n", view.Pending())
}

func addSubscriberAndShutdown() {
	ctx, cancel := context.WithCancel(context.Background())
	b := NewBroker()

	results := make(map[string][]string) // written by each consumer under mu
	var mu sync.Mutex
	record := func(who, what string) {
		mu.Lock()
		defer mu.Unlock()
		results[who] = append(results[who], what)
	}

	var consumers sync.WaitGroup
	subscribe := func(name string, handle func(OrderPlaced) string) {
		ch := b.Subscribe("orders")
		consumers.Go(func() {
			for e := range ch {
				var o OrderPlaced
				if err := json.Unmarshal(e.Data, &o); err != nil {
					record(name, "bad event: "+err.Error())
					continue
				}
				record(name, handle(o))
			}
		})
	}
	subscribe("email", func(o OrderPlaced) string { return "mailed " + o.Payload.Customer })
	subscribe("analytics", func(o OrderPlaced) string { return "counted " + cents(o.Payload.TotalCents) })
	// The new service: one Subscribe call. publishOrders is unchanged.
	subscribe("fraud", func(o OrderPlaced) string {
		if o.Payload.TotalCents > 100000 {
			return o.OrderID + " FLAGGED"
		}
		return o.OrderID + " ok"
	})

	n, _ := publishOrders(ctx, b, sampleOrders())
	fmt.Printf("publisher: published %d orders\n", n)

	// Shutdown, in order: stop the producers (cancel ctx), drain and close
	// the broker, then wait for every consumer to finish its queue.
	cancel()
	if _, err := publishOrders(ctx, b, sampleOrders()); err != nil {
		fmt.Printf("publisher: stopped: %v\n", err)
	}
	b.Close()
	consumers.Wait()

	for _, name := range []string{"email", "analytics", "fraud"} {
		fmt.Printf("%-10s %s\n", name+":", strings.Join(results[name], ", "))
	}
	fmt.Println("all consumers finished")
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
