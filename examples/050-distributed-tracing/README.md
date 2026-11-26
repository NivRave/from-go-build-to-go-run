# Part 050: Distributed tracing & OpenTelemetry

Day 25 · Week 4 · Connecting the dots across a distributed landscape.

## What this shows

- A `TracerProvider` per service with a head-based `ParentBased(TraceIDRatioBased(1.0))` sampler and a batching exporter (in memory by default, `stdouttrace` with `-stdout`).
- That the global propagator injects nothing until you call `otel.SetTextMapPropagator(propagation.TraceContext{})`.
- Service A (`PlaceOrder`) injecting its span context into the request headers by hand, and the `traceparent` header that results.
- Service B extracting it and starting `CheckStock` and `ChargeCard` as children; `ChargeCard` fails with `span.RecordError` and `span.SetStatus(codes.Error, ...)`, and its `slog` line carries `trace_id` and `span_id` (Part 049).
- `tp.Shutdown(ctx)` flushing the batcher, then a table of the exported spans: one trace ID across both services, with remote parent links.

## Run

```sh
go run ./examples/050-distributed-tracing
go test ./examples/050-distributed-tracing
go run ./examples/050-distributed-tracing -stdout
```

## What to look for

Expected output of `go run` (trace and span IDs are random on every run):

```text
== 1. A TracerProvider per service (slides p04, p07) ==
sampler: ParentBased{root:TraceIDRatioBased{1},remoteParentSampled:AlwaysOnSampler,remoteParentNotSampled:AlwaysOffSampler,localParentSampled:AlwaysOnSampler,localParentNotSampled:AlwaysOffSampler}
head sampling decides at the root span; "100% of errors" needs tail sampling in the Collector

== 2. The global propagator is a no-op until you set it (slide p05) ==
default propagator fields: []
after SetTextMapPropagator(propagation.TraceContext{}): [traceparent tracestate]

== 3. Service A calls Service B over HTTP (slides p04-p06) ==
A: traceparent: 00-2b2a07b44e08b9e88257f59ec698e4e2-7ec072e8772f5111-01
A:   version-traceid-parentid-flags; parentid = PlaceOrder's span ID: true
B: extracted parent span 7ec072e8772f5111 (remote=true)
B: CheckStock ok
level=ERROR msg="charge failed" trace_id=2b2a07b44e08b9e88257f59ec698e4e2 span_id=4ea9e254ad4252a0 order_id=ord-42 err="charge order ord-42: card declined"
B: ChargeCard failed: charge order ord-42: card declined
A: payment service answered 402 Payment Required
A: PlaceOrder returned: payment service: Payment Required

== 4. Shutdown flushes the batcher ==
exported before Shutdown: 0 spans
exported after Shutdown:  3 spans

== 5. One trace across both services ==
SERVICE          SPAN        PARENT               TRACE     STATUS
order-service    PlaceOrder  -                    2b2a07b4  Error: payment failed
payment-service  CheckStock  PlaceOrder (remote)  2b2a07b4  Unset
payment-service  ChargeCard  PlaceOrder (remote)  2b2a07b4  Error: card declined
root trace ID: 2b2a07b44e08b9e88257f59ec698e4e2
```

- The `traceparent` header carries the trace ID **and** the caller's span ID. B's "extracted parent span" is exactly A's `PlaceOrder` span, which is how B's spans get the right parent, not just the right trace.
- The two services have separate `TracerProvider`s and talk only over HTTP, yet every row shows the same trace ID. `TestNoPropagatorBreaksTrace` runs the same flow with an empty propagator and gets two unrelated traces.
- "exported before Shutdown: 0 spans": the batch processor holds ended spans until its batch fills or its timer fires. A program that exits without `Shutdown` loses them, so call it during graceful shutdown (Part 051).
- The `slog` line's `span_id` matches the `ChargeCard` span, so a log search leads straight to the failing span.
- With `-stdout`, the `stdouttrace` exporter prints every span as JSON during `Shutdown` instead of the table, including the `exception` event that `RecordError` added.

## Notes and corrections

- **Slide p07:** "trace 100% of errors" cannot be done with head sampling, which decides when the root span starts, before the outcome is known. The Go SDK only has head samplers (`AlwaysSample`, `TraceIDRatioBased`, `ParentBased`); tail sampling is done in the OpenTelemetry Collector's `tailsamplingprocessor`.
- **Slide p05:** propagation carries the trace ID and the parent span ID (W3C `traceparent` = version-traceid-parentid-flags); the trace ID alone cannot establish parentage.
- **Slide p05:** Go's global propagator is a no-op by default, so you must call `otel.SetTextMapPropagator(propagation.TraceContext{})` or nothing propagates.
- **Slide p05:** in real services, `otelhttp.NewHandler` and `otelhttp.NewTransport` do the inject and extract shown by hand here.
- **Slide p05:** "Day 20" means the context days, Parts 039 and 040 (context and context propagation), not Part 020.
- **Slide p04:** the error returned by `s.repo.Save` is not recorded on the span; combine it with slide p06's `span.RecordError` and `span.SetStatus(codes.Error, ...)`.
- **Slide p08:** the application still serialises and exports spans (batched, usually over OTLP). The Collector offloads retries, fan-out, processing and authentication.
- `runtime/trace` and Go 1.25's `trace.FlightRecorder` are *execution* tracing of one process (goroutines, GC, syscalls). They are unrelated to distributed tracing across services.

## Slides

[050-distributed-tracing.pdf](../../slides/050-distributed-tracing.pdf)
