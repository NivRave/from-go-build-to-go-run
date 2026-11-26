package main

import (
	"context"
	"log/slog"
)

// ctxKey is unexported, so only this package can set or read the value.
type ctxKey struct{}

// WithRequestID returns a copy of ctx carrying id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestIDFrom returns the request ID stored by WithRequestID.
func RequestIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok
}

// ContextHandler wraps any slog.Handler and adds a request_id attribute when
// the context passed to InfoContext, LogAttrs and friends carries one
// (slide p05, which describes this wrapper without showing it). Calls
// without a context (plain Info) get context.Background() and no ID.
type ContextHandler struct {
	slog.Handler
}

// Handle adds the request ID, then delegates. Enabled is inherited from the
// embedded handler, so level filtering still happens before any work here.
func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := RequestIDFrom(ctx); ok {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs and WithGroup must be overridden. The embedded versions return
// the inner handler, so logger.With(...) would silently drop the wrapper and
// every request_id after it.
func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{h.Handler.WithAttrs(attrs)}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{h.Handler.WithGroup(name)}
}
