package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The middleware is tested with httptest.NewRecorder: no server, no
// network, just a ResponseWriter that records what the handler wrote.

func TestRecoveryMiddleware(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantBody   string
		wantLog    string // substring of the log output; "" means nothing logged
	}{
		{
			name: "passes through when nothing panics",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, "hello")
			},
			wantStatus: http.StatusOK,
			wantBody:   "hello",
		},
		{
			name: "panic becomes a 500 and one log entry",
			handler: func(http.ResponseWriter, *http.Request) {
				panic("nil user record")
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
			wantLog:    `panic="nil user record"`,
		},
		{
			name: "runtime error panic",
			handler: func(http.ResponseWriter, *http.Request) {
				var m map[string]*int
				_ = *m["missing"] // nil pointer dereference
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
			wantLog:    "nil pointer dereference",
		},
		{
			name: "panic(nil) is caught too (Go 1.21+)",
			handler: func(http.ResponseWriter, *http.Request) {
				panic(nil)
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
			wantLog:    "panic called with nil argument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
			RecoveryMiddleware(logger, tt.handler).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}

			out := logs.String()
			if tt.wantLog == "" {
				if out != "" {
					t.Errorf("unexpected log output: %s", out)
				}
				return
			}
			for _, want := range []string{tt.wantLog, "path=/users/42", "stack="} {
				if !strings.Contains(out, want) {
					t.Errorf("log output missing %q:\n%s", want, out)
				}
			}
			if n := strings.Count(out, "\n"); n != 1 {
				t.Errorf("got %d log lines, want exactly 1", n)
			}
		})
	}
}

// http.ErrAbortHandler must go through untouched: net/http treats it as a
// deliberate abort and suppresses its usual stack trace.
func TestRecoveryMiddlewareRepanicsErrAbortHandler(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	h := RecoveryMiddleware(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	rec := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != http.ErrAbortHandler { //nolint:errorlint // the recovered value is compared by identity, exactly as net/http does
				t.Errorf("recovered %v, want http.ErrAbortHandler to be re-panicked", r)
			}
		}()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	if rec.Body.Len() != 0 {
		t.Errorf("middleware wrote a response %q for an aborted handler", rec.Body.String())
	}
	if logs.Len() != 0 {
		t.Errorf("middleware logged an intentional abort: %s", logs.String())
	}
}
