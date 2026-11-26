package main

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// RecoveryMiddleware turns a panicking handler into a 500 response and one
// structured log entry with the stack (slides p05-p06).
//
// net/http already recovers handler panics on its own: it logs the stack to
// Server.ErrorLog and closes the connection, and the server keeps running.
// What this middleware adds is a proper response for the client instead of
// a dropped connection, and the stack in your structured logs.
//
// It cannot help with a panic in a goroutine the handler starts: that
// goroutine has its own stack, and an unrecovered panic there still kills
// the process (see safeGo in main.go).
func RecoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover() // `rec`, not `err`: the panic value can be anything
			if rec == nil {
				return
			}
			// http.ErrAbortHandler is how a handler deliberately aborts a
			// response. Re-panic so net/http handles it as designed, quietly.
			if rec == http.ErrAbortHandler { //nolint:errorlint // the recovered value is compared by identity, exactly as net/http does
				panic(rec)
			}
			// debug.Stack runs before the stack unwinds, so it still shows
			// the panicking frames.
			logger.Error("panic in handler",
				"method", r.Method,
				"path", r.URL.Path,
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			// If the handler had already written a status line, this would
			// log "superfluous WriteHeader" and append to a partial body.
			// Handlers that stream need a ResponseWriter wrapper that tracks
			// whether headers were sent.
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
