package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Header names used on the wire. X-Trace-ID mirrors the slides; real systems
// use the W3C `traceparent` header, which OpenTelemetry reads and writes
// (Part 050). X-Timeout-Ms carries the *remaining* budget, the way gRPC's
// grpc-timeout header does.
const (
	HeaderTraceID   = "X-Trace-ID"
	HeaderTimeoutMs = "X-Timeout-Ms"
)

// An unexported key type, so no other package can read or overwrite it.
type ctxKey int

const traceKey ctxKey = iota

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey, id)
}

func TraceIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(traceKey).(string)
	return id, ok
}

// Transport is the client side (slide p04): it copies the trace ID and the
// remaining time budget from the request's context into headers, so callers
// never set them by hand.
type Transport struct {
	// Base performs the request. nil means http.DefaultTransport.
	Base http.RoundTripper
	// Reserve is subtracted from the budget before it is sent, leaving the
	// caller time to receive and handle the downstream error before its own
	// deadline fires. Zero sends the full remaining time.
	Reserve time.Duration
}

// RoundTrip must not modify req (that is the http.RoundTripper contract), so
// it sets the headers on a clone. Sending a relative duration instead of the
// slide's absolute RFC3339 deadline avoids depending on synchronised clocks
// and keeps millisecond precision.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	out := req.Clone(ctx) // deep-copies the headers

	if id, ok := TraceIDFrom(ctx); ok {
		out.Header.Set(HeaderTraceID, id)
	}
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl) - t.Reserve
		if remaining < time.Millisecond {
			if req.Body != nil {
				_ = req.Body.Close() // RoundTrip must close the body, even on error
			}
			return nil, fmt.Errorf("no time budget left for %s: %w", req.URL.Path, context.DeadlineExceeded)
		}
		out.Header.Set(HeaderTimeoutMs, strconv.FormatInt(remaining.Milliseconds(), 10))
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(out)
}

// Middleware is the server side (slide p05). It rebuilds the context for
// this service: the trace ID (or a new one, if the caller sent none) and the
// caller's budget, applied with context.WithTimeout on top of r.Context(),
// so the handler is stopped by whichever comes first: the budget running
// out, or the caller going away.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		traceID := r.Header.Get(HeaderTraceID)
		if traceID == "" {
			traceID = rand.Text() // we are the edge: start a new trace
		}
		ctx = WithTraceID(ctx, traceID)

		if ms, err := strconv.ParseInt(r.Header.Get(HeaderTimeoutMs), 10, 64); err == nil && ms > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
			defer cancel()
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceHandler is a slog.Handler that adds trace_id to every record logged
// with a context that carries one (logger.InfoContext(ctx, ...)).
type TraceHandler struct {
	slog.Handler
}

func (h TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := TraceIDFrom(ctx); ok {
		r.AddAttrs(slog.String("trace_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return TraceHandler{h.Handler.WithAttrs(attrs)}
}

func (h TraceHandler) WithGroup(name string) slog.Handler {
	return TraceHandler{h.Handler.WithGroup(name)}
}
