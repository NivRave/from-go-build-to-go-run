# Part 054: The event-driven mindset: Pub/Sub patterns

Day 27 · Week 4 · Moving from "telling" to "reacting".

This is the second part of Day 27. Part 053 (message queues) is coming and does not have an example yet.

## What this shows

- An in-process `Broker`: `Subscribe(topic) <-chan Event` gives each subscriber its own queue and goroutine, and `Publish` fans a copy out to each. A fast and a slow subscriber consume the same stream at their own rates without slowing the publisher.
- Versioned fat events: `OrderPlaced{Version, OrderID, Seq, Timestamp, Payload}` as JSON. A v2 consumer reads a v1 event, and a v1 consumer reads a v2 event and ignores the field it does not know.
- At-least-once delivery: the broker redelivers some events, and an idempotent handler built on `sync.Map.LoadOrStore` prints "duplicate skipped". Eight concurrent deliveries of one event charge once. `-race-demo` runs slide p07's check-then-act version instead.
- Out-of-order delivery: `PaymentProcessed` arrives before its `OrderPlaced`, is parked in a pending map, and is released when the order arrives.
- A `fraud` subscriber added with one `Subscribe` call while the publisher stays unchanged, then a graceful shutdown: cancel the context to stop the producer, `Broker.Close` to drain the queues, and `wg.Wait` for the consumers started with `wg.Go`.

## Run

```sh
go run ./examples/054-event-driven-and-pub-sub
go test ./examples/054-event-driven-and-pub-sub
go run ./examples/054-event-driven-and-pub-sub -race-demo
go test -race ./examples/054-event-driven-and-pub-sub
```

## What to look for

Expected output of `go run` (section 1 depends on timing; on a heavily loaded machine "slow has processed" may be above 0):

```text
== 1. Fan-out: fast and slow subscribers (slide p03) ==
publisher: published 6 orders and moved on; slow has processed 0
fast: processed 6
slow: processed 6 (20ms each, from its own queue)

== 2. Versioned fat events (slide p05) ==
v1 on the wire: {"v":"1","id":"ord-1","seq":1,"ts":"2026-11-26T09:00:00Z","data":{"customer":"ada","total_cents":4250}}
v2 consumer read v1: id=ord-1 total=$42.50 currency=USD (defaulted)
v2 on the wire: {"v":"2","id":"ord-2","seq":1,"ts":"2026-11-26T09:00:00Z","data":{"customer":"linus","total_cents":1000,"currency":"EUR"}}
v1 consumer read v2: id=ord-2 total=$10.00 (unknown field "currency" ignored)

== 3. At-least-once delivery needs idempotent handlers (slide p07) ==
evt-ord-1: charged
evt-ord-2: charged
evt-ord-2: duplicate skipped
evt-ord-3: charged
evt-ord-3: duplicate skipped
5 deliveries, 3 charges
8 concurrent deliveries of evt-ord-9: charged 1 time(s)

== 4. Out-of-order events: park until the order arrives (slide p06) ==
ord-7 seq=2: payment arrived before its order, parked (1 pending)
ord-7 seq=1: order placed (total $25.00)
ord-7 seq=2: releasing parked payment
ord-7 seq=2: paid $25.00 of $25.00
ord-8 seq=1: order placed (total $9.00)
ord-8 seq=2: paid $9.00 of $9.00
pending payments left: 0

== 5. Add a Fraud subscriber, then shut down gracefully (slide p03) ==
publisher: published 3 orders
publisher: stopped: context canceled
email:     mailed ada, mailed linus, mailed grace
analytics: counted $42.50, counted $19.99, counted $1500.00
fraud:     ord-1 ok, ord-2 ok, ord-3 FLAGGED
all consumers finished
```

- Section 1: `Publish` only appends to each subscriber's queue, so the publisher finishes before the slow subscriber has handled anything, and the fast one is not held back either.
- Section 3: `LoadOrStore` checks and marks in one atomic step, so concurrent duplicates cannot both win. If the work fails, `Dedup.Handle` deletes the mark so a redelivery can retry.
- `-race-demo` prints `8 concurrent deliveries of evt-ord-9: charged 8 time(s)` (the count can vary, but is almost always 8). Each step of slide p07's handler is locked, so `go run -race` reports nothing. The race detector finds unsynchronised memory access, not check-then-act logic races, which is why `TestDedupConcurrentDuplicates` checks the outcome.
- Section 5: the same `publishOrders` function serves three subscribers. Closing the broker lets every queue drain before the subscriber channels close, so no consumer misses an event that was already published.

## Notes and corrections

- **Slide p07:** `alreadyProcessed` followed by `markAsProcessed` is a check-then-act race: two concurrent deliveries both pass the check and both do the work. The dedup record must be written atomically with the side effect, for example in the same transaction, with `INSERT ... ON CONFLICT DO NOTHING` on a `processed_events` key, or in process with `sync.Map.LoadOrStore`. A crash between the work and the mark re-runs the work, so the work itself must be idempotent too.
- **Slide p05:** consumers must tolerate unknown fields. `encoding/json` ignores them by default; don't enable `DisallowUnknownFields` on event consumers.
- **Slide p06:** idempotency keys remove duplicates but don't restore order. Ordering needs per-key sequence numbers (the `Seq` field here) or partitioning by key, such as a Kafka partition key of the order ID.
- **Slide p03:** copying each message per subscriber is how RabbitMQ-style brokers work. Log-based brokers such as Kafka and NATS JetStream keep one copy with a read offset per consumer. An AMQP exchange routes messages to queues rather than being another name for a topic.
- **Slides p04 and p09:** "scales infinitely" overstates it. Choreography removes a central bottleneck at the cost of implicit workflows that are harder to debug, and duplicates are a consequence of at-least-once delivery that handlers must absorb, not a feature.

## Slides

[054-event-driven-and-pub-sub.pdf](../../slides/054-event-driven-and-pub-sub.pdf)
