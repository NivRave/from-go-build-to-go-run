// Command distributed-tracing demonstrates OpenTelemetry tracing across two
// in-process HTTP services: a ParentBased sampler, W3C traceparent
// propagation done by hand, error status on spans, flushing with Shutdown and
// putting the trace ID on slog lines.
//
// Slides: slides/050-distributed-tracing.pdf
//
// Run:
//
//	go run ./examples/050-distributed-tracing
//	go run ./examples/050-distributed-tracing -stdout
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func main() {
	stdout := flag.Bool("stdout", false, "export spans with the stdouttrace pretty printer instead of printing a table")
	flag.Parse()

	if err := run(*stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(useStdout bool) error {
	ctx := context.Background()

	section("1. A TracerProvider per service (slides p04, p07)")
	mem := keepSpans{tracetest.NewInMemoryExporter()}
	var exp sdktrace.SpanExporter = mem
	if useStdout {
		pretty, err := stdouttrace.New(stdouttrace.WithPrettyPrint(), stdouttrace.WithWriter(os.Stdout))
		if err != nil {
			return err
		}
		exp = pretty
	}
	tpA := newTracerProvider("order-service", exp)
	tpB := newTracerProvider("payment-service", exp)
	describeSampler()

	section("2. The global propagator is a no-op until you set it (slide p05)")
	installPropagator()

	section("3. Service A calls Service B over HTTP (slides p04-p06)")
	// Part 049 correlation: B logs through slog with trace_id attached. The
	// time attribute is dropped to keep the output short.
	out := &lockedWriter{w: os.Stdout}
	logger := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	srv := httptest.NewServer(newPaymentService(tpB.Tracer("payment-service"), out, logger))
	defer srv.Close()

	root, err := placeOrder(ctx, tpA.Tracer("order-service"), srv.Client(), srv.URL, "ord-42", out)
	fmt.Fprintf(out, "A: PlaceOrder returned: %v\n", err)

	section("4. Shutdown flushes the batcher")
	if !useStdout {
		// The batch span processor holds ended spans until its batch fills or
		// its 5 s timer fires, so a program that exits without Shutdown loses
		// them.
		fmt.Printf("exported before Shutdown: %d spans\n", len(mem.GetSpans()))
	}
	if err := shutdownAll(ctx, tpA, tpB); err != nil {
		return err
	}
	if useStdout {
		fmt.Println("(the stdouttrace exporter printed each span as JSON above)")
		return nil
	}
	spans := mem.GetSpans()
	fmt.Printf("exported after Shutdown:  %d spans\n", len(spans))

	section("5. One trace across both services")
	printSpanTable(os.Stdout, spans)
	fmt.Printf("root trace ID: %s\n", root.TraceID())
	return nil
}

// describeSampler prints the sampler's own description, which spells out
// what ParentBased does for each kind of parent.
func describeSampler() {
	s := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))
	fmt.Println("sampler:", s.Description())
	fmt.Println("head sampling decides at the root span; \"100% of errors\" needs tail sampling in the Collector")
}

// installPropagator shows the gotcha from slide p05: the global propagator
// injects nothing until you install one, so traces silently stop at every
// service boundary.
func installPropagator() {
	fmt.Printf("default propagator fields: %v\n", otel.GetTextMapPropagator().Fields())
	otel.SetTextMapPropagator(propagation.TraceContext{})
	fmt.Printf("after SetTextMapPropagator(propagation.TraceContext{}): %v\n", otel.GetTextMapPropagator().Fields())
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
