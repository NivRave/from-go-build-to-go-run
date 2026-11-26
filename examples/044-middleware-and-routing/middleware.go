package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Middleware is a handler that wraps a handler (slide p02).
type Middleware func(http.Handler) http.Handler

// Chain applies middleware in the order it is listed, with the first one
// outermost: Chain(h, A, B) is A(B(h)), so a request passes A, then B, then
// h. It iterates in reverse because the last wrapper applied ends up on the
// outside.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// SlideChain is the version from slide p04. Ranging forward wraps A first and
// B around it, so Chain(h, A, B) becomes B(A(h)) and B runs first: the
// opposite of the order the slide describes.
func SlideChain(h http.Handler, middlewares ...Middleware) http.Handler {
	for _, m := range middlewares {
		h = m(h)
	}
	return h
}

// statusRecorder captures the status code a handler writes, which the
// slide's logger cannot see (slide p03).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK // the first Write sends an implicit 200
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer, so
// Flush, SetWriteDeadline and friends keep working through the wrapper
// (Go 1.20+). Without it, wrapping hides those capabilities.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Logging records method, path, status and duration with slog.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			next.ServeHTTP(rec, r)

			if rec.status == 0 {
				rec.status = http.StatusOK // handler wrote nothing at all
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// Recovery turns a handler panic into a 500 response and a log line
// (slide p08).
//
// net/http already recovers panics per connection, so without this the
// process survives anyway; the client just sees an aborted connection. What
// this adds is a proper 500 and a structured log entry. It cannot catch
// panics in goroutines the handler starts: those still crash the process.
//
// http.ErrAbortHandler is re-panicked: it is the documented way for a
// handler to abort the response, and net/http handles it silently.
func Recovery(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				// A production version would also log debug.Stack(). If the
				// handler already wrote headers, the 500 below can't replace
				// them; net/http logs "superfluous WriteHeader" in that case.
				logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
					slog.Any("value", v),
					slog.String("path", r.URL.Path),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ctxKey is unexported, so no other package can create a colliding key or
// read the value without going through UserFrom (slide p07).
type ctxKey struct{}

// UserFrom returns the user ID that Auth stored in ctx.
func UserFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok
}

// Auth accepts "Authorization: Bearer <token>" for a known token and stores
// the matching user ID in the request context. Anything else gets a 401.
//
// The context is a request-scoped carrier, not a security boundary: any code
// holding the context can read the value.
func Auth(tokens map[string]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			userID, known := tokens[token]
			if !ok || !known {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
