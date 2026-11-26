package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"text/tabwriter"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// errCardDeclined is the failure Service B reports from ChargeCard.
var errCardDeclined = errors.New("card declined")

// newTracerProvider builds the SDK TracerProvider one service would create at
// startup. Each service gets its own provider (and its own service.name), just
// as two separate processes would; the only thing linking their spans is the
// traceparent header.
//
// Sampling is head-based: the decision is made when the root span starts,
// before anyone knows whether the request will fail. ParentBased makes every
// downstream service follow the caller's decision (the "sampled" flag in
// traceparent), and TraceIDRatioBased(1.0) samples every new trace. A policy
// like "1% of successes, 100% of errors" (slide p07) needs tail sampling,
// which lives in the OpenTelemetry Collector (tailsamplingprocessor), not in
// the Go SDK.
//
// WithBatcher is what production code uses: spans are queued and exported in
// batches, which is why Shutdown must be called to flush them.
func newTracerProvider(service string, exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service))),
	)
}

// keepSpans wraps the in-memory exporter so that TracerProvider.Shutdown does
// not wipe what it collected: InMemoryExporter.Shutdown clears its spans, and
// we want to print them after the flush. Two providers share one exporter, so
// the first Shutdown must not discard the other service's spans either.
type keepSpans struct {
	*tracetest.InMemoryExporter
}

func (keepSpans) Shutdown(context.Context) error { return nil }

// lockedWriter serialises writes from the client goroutine and the server's
// handler goroutine. The HTTP round trip orders them in practice, but the race
// detector cannot see synchronisation through a TCP socket.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// newPaymentService is Service B. It extracts the caller's span context from
// the headers, so its spans become children of Service A's PlaceOrder span
// in the same trace.
func newPaymentService(tracer trace.Tracer, out io.Writer, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract reads traceparent and returns a context carrying a *remote*
		// span context. Without a configured propagator this is a no-op and
		// every span below would start a brand-new trace.
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		remote := trace.SpanContextFromContext(ctx)
		fmt.Fprintf(out, "B: extracted parent span %s (remote=%t)\n", remote.SpanID(), remote.IsRemote())

		orderID := r.URL.Query().Get("order")
		if err := checkStock(ctx, tracer, orderID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		fmt.Fprintln(out, "B: CheckStock ok")

		if err := chargeCard(ctx, tracer, logger, orderID); err != nil {
			fmt.Fprintf(out, "B: ChargeCard failed: %v\n", err)
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func checkStock(ctx context.Context, tracer trace.Tracer, orderID string) error {
	_, span := tracer.Start(ctx, "CheckStock")
	defer span.End()
	span.SetAttributes(attribute.String("order.id", orderID), attribute.Int("stock.available", 3))
	return nil
}

func chargeCard(ctx context.Context, tracer trace.Tracer, logger *slog.Logger, orderID string) error {
	ctx, span := tracer.Start(ctx, "ChargeCard")
	defer span.End()
	span.SetAttributes(attribute.String("order.id", orderID))

	err := fmt.Errorf("charge order %s: %w", orderID, errCardDeclined)
	// Slide p06: record the error (an "exception" event with the message)
	// and mark the span as failed so the backend draws it red.
	span.RecordError(err)
	span.SetStatus(codes.Error, errCardDeclined.Error())
	// Part 049 correlation: put the trace and span IDs on the log line so a
	// log search leads straight to this span.
	withTrace(ctx, logger).Error("charge failed", "order_id", orderID, "err", err)
	return err
}

// placeOrder is Service A: it starts the root span, injects its context into
// the outgoing request and calls Service B. It returns the span context of
// PlaceOrder so callers can compare it with what B received.
func placeOrder(ctx context.Context, tracer trace.Tracer, client *http.Client, baseURL, orderID string, out io.Writer) (trace.SpanContext, error) {
	ctx, span := tracer.Start(ctx, "PlaceOrder")
	defer span.End() // Always end the span (slide p04).
	span.SetAttributes(attribute.String("order.id", orderID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/charge?order="+orderID, http.NoBody)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "build request")
		return span.SpanContext(), err
	}
	// Inject writes traceparent (and tracestate) into the headers. This is
	// what otelhttp.NewTransport does for you; it is spelled out here.
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	fmt.Fprintf(out, "A: traceparent: %s\n", req.Header.Get("traceparent"))
	fmt.Fprintf(out, "A:   version-traceid-parentid-flags; parentid = PlaceOrder's span ID: %t\n",
		req.Header.Get("traceparent") == traceparent(span.SpanContext()))

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "call payment service")
		return span.SpanContext(), err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	fmt.Fprintf(out, "A: payment service answered %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("payment service: %s", http.StatusText(resp.StatusCode))
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment failed")
		return span.SpanContext(), err
	}
	return span.SpanContext(), nil
}

// traceparent formats a span context the way the W3C TraceContext propagator
// does, for comparison with the injected header.
func traceparent(sc trace.SpanContext) string {
	return fmt.Sprintf("00-%s-%s-%s", sc.TraceID(), sc.SpanID(), sc.TraceFlags())
}

// withTrace returns a logger that tags every record with the current trace
// and span IDs (the Part 049 correlation).
func withTrace(ctx context.Context, logger *slog.Logger) *slog.Logger {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return logger
	}
	return logger.With("trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
}

// printSpanTable prints one line per span, ordered by start time, with the
// parent's name resolved from the same batch of spans.
func printSpanTable(w io.Writer, spans tracetest.SpanStubs) {
	names := make(map[trace.SpanID]string, len(spans))
	for _, s := range spans {
		names[s.SpanContext.SpanID()] = s.Name
	}
	sorted := slices.Clone(spans)
	slices.SortFunc(sorted, func(a, b tracetest.SpanStub) int { return a.StartTime.Compare(b.StartTime) })

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SERVICE\tSPAN\tPARENT\tTRACE\tSTATUS")
	for _, s := range sorted {
		parent := "-"
		if s.Parent.IsValid() {
			parent = names[s.Parent.SpanID()]
			if s.Parent.IsRemote() {
				parent += " (remote)"
			}
		}
		status := s.Status.Code.String()
		if s.Status.Description != "" {
			status += ": " + s.Status.Description
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			serviceName(s), s.Name, parent, s.SpanContext.TraceID().String()[:8], status)
	}
	_ = tw.Flush()
}

func serviceName(s tracetest.SpanStub) string {
	if s.Resource == nil {
		return "?"
	}
	if v, ok := s.Resource.Set().Value("service.name"); ok {
		return v.AsString()
	}
	return "?"
}

// shutdownAll flushes and stops every provider. Shutdown blocks until queued
// spans are exported or ctx expires, so give it a deadline (and in a real
// service, run it during graceful shutdown; see Part 051).
func shutdownAll(ctx context.Context, tps ...*sdktrace.TracerProvider) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var errs []error
	for _, tp := range tps {
		errs = append(errs, tp.Shutdown(ctx))
	}
	return errors.Join(errs...)
}
