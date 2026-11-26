package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
)

// The Transport and Middleware tests run inside synctest bubbles: time does
// not advance while the code under test runs, so a 500ms budget is sent as
// exactly "500", not "499" or "500" depending on the machine.

func TestTransportClonesAndInjects(t *testing.T) {
	tests := []struct {
		name        string
		traceID     string // "" means no trace ID in the context
		timeout     time.Duration
		reserve     time.Duration
		wantTrace   string
		wantTimeout string
	}{
		{"trace and deadline", "abc-123", 500 * time.Millisecond, 0, "abc-123", "500"},
		{"reserve is subtracted", "abc-123", 500 * time.Millisecond, 10 * time.Millisecond, "abc-123", "490"},
		{"no deadline", "abc-123", 0, 0, "abc-123", ""},
		{"no trace ID", "", time.Second, 0, "", "1000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				if tt.traceID != "" {
					ctx = WithTraceID(ctx, tt.traceID)
				}
				if tt.timeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tt.timeout)
					defer cancel()
				}

				var sent *http.Request
				tr := &Transport{Reserve: tt.reserve, Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					sent = r
					return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: r}, nil
				})}

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.invalid/", nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Accept", "text/plain")
				resp, err := tr.RoundTrip(req)
				if err != nil {
					t.Fatal(err)
				}
				_ = resp.Body.Close()

				if sent == req {
					t.Fatal("Transport passed the caller's request through instead of a clone")
				}
				if len(req.Header) != 1 || req.Header.Get(HeaderTraceID) != "" || req.Header.Get(HeaderTimeoutMs) != "" {
					t.Errorf("original request was modified: headers = %v", req.Header)
				}
				if got := sent.Header.Get("Accept"); got != "text/plain" {
					t.Errorf("clone lost the caller's headers: Accept = %q", got)
				}
				if got := sent.Header.Get(HeaderTraceID); got != tt.wantTrace {
					t.Errorf("%s = %q, want %q", HeaderTraceID, got, tt.wantTrace)
				}
				if got := sent.Header.Get(HeaderTimeoutMs); got != tt.wantTimeout {
					t.Errorf("%s = %q, want %q", HeaderTimeoutMs, got, tt.wantTimeout)
				}
			})
		})
	}
}

func TestTransportNoBudgetLeft(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
		defer cancel()
		called := false
		tr := &Transport{Reserve: 10 * time.Millisecond, Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return nil, errors.New("must not be called")
		})}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.invalid/", nil)

		resp, err := tr.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", err)
		}
		if called {
			t.Error("base RoundTripper was called with no budget left")
		}
	})
}

// A nil Base falls back to http.DefaultTransport. This one needs a real
// (loopback) connection, so it runs outside synctest.
func TestTransportNilBaseUsesDefaultTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Echo-Trace", r.Header.Get(HeaderTraceID))
	}))
	defer srv.Close()

	client := &http.Client{Transport: &Transport{}}
	req, _ := http.NewRequestWithContext(WithTraceID(t.Context(), "abc-123"), http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Echo-Trace"); got != "abc-123" {
		t.Errorf("server saw %s = %q, want %q", HeaderTraceID, got, "abc-123")
	}
}

func TestMiddlewareBudget(t *testing.T) {
	tests := []struct {
		name         string
		timeoutMs    string
		traceID      string
		wantDeadline time.Duration // 0: no deadline expected
	}{
		{"budget applied", "700", "abc-123", 700 * time.Millisecond},
		{"no budget header", "", "abc-123", 0},
		{"garbage budget is ignored", "soon", "abc-123", 0},
		{"negative budget is ignored", "-5", "abc-123", 0},
		{"missing trace ID gets a new one", "250", "", 250 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				var gotDeadline time.Duration
				var gotTrace string
				h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if dl, ok := r.Context().Deadline(); ok {
						gotDeadline = time.Until(dl)
					}
					gotTrace, _ = TraceIDFrom(r.Context())
				}))

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if tt.timeoutMs != "" {
					req.Header.Set(HeaderTimeoutMs, tt.timeoutMs)
				}
				if tt.traceID != "" {
					req.Header.Set(HeaderTraceID, tt.traceID)
				}
				h.ServeHTTP(httptest.NewRecorder(), req)

				if gotDeadline != tt.wantDeadline {
					t.Errorf("handler deadline in %v, want %v", gotDeadline, tt.wantDeadline)
				}
				switch {
				case tt.traceID != "" && gotTrace != tt.traceID:
					t.Errorf("trace ID = %q, want %q", gotTrace, tt.traceID)
				case tt.traceID == "" && gotTrace == "":
					t.Error("no trace ID generated for a request without one")
				}
			})
		})
	}
}

// TestMiddlewareStopsSlowHandler checks the whole point of the budget: a
// handler that honours r.Context() gives up when the caller's budget does.
func TestMiddlewareStopsSlowHandler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var err error
		var elapsed time.Duration
		h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			select {
			case <-time.After(2 * time.Second):
			case <-r.Context().Done():
				err = r.Context().Err()
			}
			elapsed = time.Since(start)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(HeaderTimeoutMs, "700")
		h.ServeHTTP(httptest.NewRecorder(), req)

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("handler err = %v, want context.DeadlineExceeded", err)
		}
		if elapsed != 700*time.Millisecond {
			t.Errorf("handler stopped after %v, want 700ms", elapsed)
		}
	})
}
