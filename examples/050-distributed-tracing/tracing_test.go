package main

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// runFlow runs the A -> B request with the given global propagator and returns
// every exported span keyed by name. The tests are not parallel because the
// propagator is process-global.
func runFlow(t *testing.T, prop propagation.TextMapPropagator) map[string]tracetest.SpanStub {
	t.Helper()
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(prop)
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	mem := keepSpans{tracetest.NewInMemoryExporter()}
	tpA := newTracerProvider("order-service", mem)
	tpB := newTracerProvider("payment-service", mem)

	srv := httptest.NewServer(newPaymentService(tpB.Tracer("payment-service"), io.Discard, slog.New(slog.DiscardHandler)))
	defer srv.Close()

	if _, err := placeOrder(t.Context(), tpA.Tracer("order-service"), srv.Client(), srv.URL, "ord-1", io.Discard); err == nil {
		t.Error("placeOrder: want an error from the declined charge, got nil")
	}
	if err := shutdownAll(context.Background(), tpA, tpB); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	byName := make(map[string]tracetest.SpanStub)
	for _, s := range mem.GetSpans() {
		byName[s.Name] = s
	}
	for _, name := range []string{"PlaceOrder", "CheckStock", "ChargeCard"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("span %q was not exported; got %d spans", name, len(byName))
		}
	}
	return byName
}

func TestSpansShareOneTrace(t *testing.T) {
	spans := runFlow(t, propagation.TraceContext{})
	want := spans["PlaceOrder"].SpanContext.TraceID()
	for name, s := range spans {
		if got := s.SpanContext.TraceID(); got != want {
			t.Errorf("%s: trace ID %s, want %s", name, got, want)
		}
	}
}

func TestParentLinks(t *testing.T) {
	spans := runFlow(t, propagation.TraceContext{})
	root := spans["PlaceOrder"]
	if root.Parent.IsValid() {
		t.Errorf("PlaceOrder: want a root span, got parent %s", root.Parent.SpanID())
	}
	for _, name := range []string{"CheckStock", "ChargeCard"} {
		s := spans[name]
		if s.Parent.SpanID() != root.SpanContext.SpanID() {
			t.Errorf("%s: parent %s, want PlaceOrder %s", name, s.Parent.SpanID(), root.SpanContext.SpanID())
		}
		if !s.Parent.IsRemote() {
			t.Errorf("%s: parent should be remote (extracted from traceparent)", name)
		}
	}
}

func TestChargeCardErrorStatus(t *testing.T) {
	spans := runFlow(t, propagation.TraceContext{})
	tests := []struct {
		span string
		want codes.Code
	}{
		{"PlaceOrder", codes.Error},
		{"CheckStock", codes.Unset},
		{"ChargeCard", codes.Error},
	}
	for _, tt := range tests {
		t.Run(tt.span, func(t *testing.T) {
			if got := spans[tt.span].Status.Code; got != tt.want {
				t.Errorf("status = %v, want %v", got, tt.want)
			}
		})
	}

	// RecordError adds an "exception" event carrying the error message.
	var recorded bool
	for _, ev := range spans["ChargeCard"].Events {
		if ev.Name == "exception" {
			recorded = true
		}
	}
	if !recorded {
		t.Error("ChargeCard: RecordError did not add an exception event")
	}
}

// TestNoPropagatorBreaksTrace shows the slide p05 gotcha: with a propagator
// that injects nothing (the default), Service B starts a new trace.
func TestNoPropagatorBreaksTrace(t *testing.T) {
	spans := runFlow(t, propagation.NewCompositeTextMapPropagator())
	a := spans["PlaceOrder"].SpanContext.TraceID()
	b := spans["ChargeCard"].SpanContext.TraceID()
	if a == b {
		t.Errorf("without a propagator the services should not share trace %s", a)
	}
	if spans["ChargeCard"].Parent.IsValid() {
		t.Error("without a propagator ChargeCard should be a root span")
	}
}
