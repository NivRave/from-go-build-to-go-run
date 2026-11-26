package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// ---- 4. handle or return ----------------------------------------------------------

var errConnReset = errors.New("connection reset by peer")

// Three layers, each adding what it knows. None of the lower layers logs:
// they return, and only the top layer handles (logs) the error, once.

func queryOrder(id int) error { // repository layer
	return fmt.Errorf("query orders where id=%d: %w", id, errConnReset)
}

func loadOrder(id int) error { // service layer
	if err := queryOrder(id); err != nil {
		return fmt.Errorf("load order: %w", err)
	}
	return nil
}

func checkout(id int) error { // use-case layer
	if err := loadOrder(id); err != nil {
		return fmt.Errorf("checkout %d: %w", id, err)
	}
	return nil
}

// newDemoLogger writes slog's text format to stdout without the time
// attribute, so the demo output is the same on every run.
func newDemoLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	}))
}

func handleOrReturn() {
	logger := newDemoLogger()

	// The top layer (an HTTP handler, a job runner, main) handles it: one
	// failure, one log entry, with the whole logical stack in the message.
	if err := checkout(42); err != nil {
		logger.Error("checkout failed", "request_id", "req-0042", "err", err)
	}
}

// ---- 5. sanitising at the API edge --------------------------------------------------

var (
	ErrOrderNotFound = errors.New("order not found")
	errDBUnreachable = errors.New("dial tcp 10.0.0.5:5432: connect: connection refused")
)

// PublicError is the only thing a client ever sees.
type PublicError struct {
	Message   string `json:"error"`
	RequestID string `json:"request_id"`
}

// Sanitize maps an internal error to a safe public message. Known cases get a
// specific message; everything else gets a generic one. The request ID lets
// support find the full internal error in the logs.
func Sanitize(err error, requestID string) PublicError {
	switch {
	case errors.Is(err, ErrOrderNotFound):
		return PublicError{Message: "Order not found.", RequestID: requestID}
	default:
		return PublicError{Message: "Update failed. Please try again.", RequestID: requestID}
	}
}

func updateUser(id int) error {
	return fmt.Errorf("db update failed for user %d: %w", id, errDBUnreachable)
}

func findOrder(id int) error {
	return fmt.Errorf("find order %d: %w", id, ErrOrderNotFound)
}

func sanitize() {
	for i, err := range []error{updateUser(7), findOrder(99)} {
		reqID := fmt.Sprintf("req-%04d", i+1)
		public, _ := json.Marshal(Sanitize(err, reqID))
		fmt.Println("internal (logs only):", err)
		fmt.Println("public (to client):  ", string(public))
	}
}
