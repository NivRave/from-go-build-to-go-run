// Command context-propagation demonstrates carrying a context across HTTP
// hops: a RoundTripper that injects the trace ID and the remaining budget,
// middleware that rebuilds the context on the other side, deadline
// budgeting through three in-process services, cancellation that travels by
// aborting connections, and slog lines tagged with the trace ID.
//
// Slides: slides/040-context-propagation.pdf
//
// Run:
//
//	go run ./examples/040-context-propagation
//	go test ./examples/040-context-propagation
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"
)

func main() {
	section("1. Client: a RoundTripper that clones and injects (slide p04)")
	transportDemo()

	section("2. Server: middleware rehydrates the trace ID and the budget (slide p05)")
	middlewareDemo()

	sys := startServices(newLogger(os.Stdout))
	defer sys.Close()

	section("3. Budgeting: gateway 1s -> auth 300ms -> business (slide p03)")
	sys.budgeting()

	section("4. Cancellation travels by aborting connections (slides p07, p08)")
	sys.cancellation()

	section("5. slog lines carry trace_id from the context")
	slogDemo()
}

// roundTripFunc lets a plain function act as an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// transportDemo sends one request through Transport to a base RoundTripper
// that only records what it was given.
func transportDemo() {
	var sent *http.Request
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		sent = r
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: r}, nil
	})
	client := &http.Client{Transport: &Transport{Base: base}}

	ctx, cancel := context.WithTimeout(WithTraceID(context.Background(), "abc-123"), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://auth.internal/verify", nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	_ = resp.Body.Close()

	ms, _ := time.ParseDuration(sent.Header.Get(HeaderTimeoutMs) + "ms")
	fmt.Printf("sent:     %s=%s, %s=~%v\n", HeaderTraceID, sent.Header.Get(HeaderTraceID),
		HeaderTimeoutMs, ms.Round(100*time.Millisecond))
	fmt.Printf("original: %d headers (the caller's request was not modified)\n", len(req.Header))
}

// middlewareDemo feeds Middleware a request with and without the headers,
// using httptest.NewRecorder, and reports what the handler sees.
func middlewareDemo() {
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := TraceIDFrom(r.Context())
		if r.Header.Get(HeaderTraceID) == "" {
			id = fmt.Sprintf("<new, %d chars>", len(id))
		}
		budget := "none"
		if dl, ok := r.Context().Deadline(); ok {
			budget = "~" + time.Until(dl).Round(100*time.Millisecond).String()
		}
		fmt.Printf("handler sees trace_id=%s, budget %s\n", id, budget)
	}))

	withHeaders := httptest.NewRequest(http.MethodGet, "/orders/42", nil)
	withHeaders.Header.Set(HeaderTraceID, "abc-123")
	withHeaders.Header.Set(HeaderTimeoutMs, "700")
	h.ServeHTTP(httptest.NewRecorder(), withHeaders)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders/42", nil))
}

// system is three services, each its own httptest.Server, wired
// gateway -> auth -> business. Every service uses Middleware on the way in
// and Transport on the way out.
type system struct {
	gateway, auth, business *httptest.Server
	client                  *http.Client // what callers of the gateway use
	finished                chan struct{}
}

// reserve leaves each hop 10ms to report a downstream failure before its own
// deadline fires (see Transport.Reserve).
const reserve = 10 * time.Millisecond

func startServices(logger *slog.Logger) *system {
	s := &system{
		client:   &http.Client{Transport: &Transport{Reserve: reserve}},
		finished: make(chan struct{}, 16),
	}

	// Business: does the actual work and honours r.Context().
	blog := logger.With("service", "business")
	bmux := http.NewServeMux()
	bmux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		defer func() { s.finished <- struct{}{} }()
		ctx := r.Context()
		budget := "none"
		if dl, ok := ctx.Deadline(); ok {
			budget = time.Until(dl).Round(100 * time.Millisecond).String()
		}
		blog.InfoContext(ctx, "handling order", "id", r.PathValue("id"), "budget", budget)

		work, _ := time.ParseDuration(r.URL.Query().Get("work"))
		select {
		case <-time.After(work):
			fmt.Fprintf(w, "order %s: shipped\n", r.PathValue("id"))
		case <-ctx.Done():
			blog.InfoContext(ctx, "stopped early", "err", ctx.Err())
			http.Error(w, ctx.Err().Error(), http.StatusGatewayTimeout)
		}
	})
	// The wrong way (slide p07): a fresh context.Background() detaches the
	// work from the request, so it carries on after everyone has given up.
	bmux.HandleFunc("GET /detached/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		defer func() { s.finished <- struct{}{} }()
		ctx := context.Background() // WRONG: ignores the caller's cancellation
		work, _ := time.ParseDuration(r.URL.Query().Get("work"))
		select {
		case <-time.After(work):
			blog.InfoContext(r.Context(), "detached handler finished work nobody is waiting for", "id", r.PathValue("id"))
		case <-ctx.Done(): // never closes
		}
	})
	s.business = newServer(Middleware(bmux))

	// Auth: takes 300ms to verify the caller, then forwards to business.
	alog := logger.With("service", "auth")
	amux := http.NewServeMux()
	amux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(300 * time.Millisecond):
		case <-r.Context().Done():
			http.Error(w, r.Context().Err().Error(), http.StatusGatewayTimeout)
			return
		}
		alog.InfoContext(r.Context(), "user verified")
		forward(w, r, s.client, s.business.URL)
	})
	s.auth = newServer(Middleware(amux))

	// Gateway: the edge. It owns the 1s budget for the whole request.
	gmux := http.NewServeMux()
	gmux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		forward(w, r.WithContext(ctx), s.client, s.auth.URL)
	})
	s.gateway = newServer(Middleware(gmux))
	return s
}

// newServer starts an httptest.Server whose own error log is silenced:
// aborted connections are part of the demo.
func newServer(h http.Handler) *httptest.Server {
	srv := httptest.NewUnstartedServer(h)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	return srv
}

// forward sends the same path and query to the next service with the
// incoming request's context, so trace ID, budget and cancellation all
// travel with it.
func forward(w http.ResponseWriter, r *http.Request, client *http.Client, baseURL string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, baseURL+r.URL.RequestURI(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "upstream: "+err.Error(), http.StatusGatewayTimeout)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *system) Close() {
	s.gateway.Close()
	s.auth.Close()
	s.business.Close()
}

// call sends GET path to the gateway and waits until the business handler
// has finished, so its log lines always come before the result line.
func (s *system) call(ctx context.Context, path string) {
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.gateway.URL+path, nil)
	resp, err := s.client.Do(req)
	elapsed := time.Since(start).Round(100 * time.Millisecond)
	select {
	case <-s.finished:
	case <-time.After(3 * time.Second): // on a very slow machine business may never be reached
	}
	if err != nil {
		fmt.Printf("client: GET %s -> error after ~%v: %v\n", path, elapsed, errors.Unwrap(err))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("client: GET %s -> %s after ~%v: %s\n", path, resp.Status, elapsed, strings.TrimSpace(string(body)))
}

func (s *system) budgeting() {
	s.call(WithTraceID(context.Background(), "trace-fast"), "/orders/42?work=100ms")
	s.call(WithTraceID(context.Background(), "trace-slow"), "/orders/43?work=2s")
}

// cancellation: the client gives up at 400ms, while business is working.
// Nothing is sent downstream to say so. The client closes its connection,
// the gateway's r.Context() is cancelled, which aborts its connection to
// auth, and so on down the chain.
func (s *system) cancellation() {
	for _, path := range []string{"/orders/44?work=2s", "/detached/orders/45?work=300ms"} {
		ctx, cancel := context.WithCancel(WithTraceID(context.Background(), "trace-cancel"))
		time.AfterFunc(400*time.Millisecond, cancel)
		s.call(ctx, path)
		cancel()
	}
}

// newLogger writes text logs without timestamps, so the output is readable,
// and wraps the handler so every *Context call adds trace_id.
func newLogger(w io.Writer) *slog.Logger {
	h := slog.NewTextHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	})
	return slog.New(TraceHandler{h})
}

func slogDemo() {
	logger := newLogger(os.Stdout).With("service", "payments")
	ctx := WithTraceID(context.Background(), "abc-123")
	logger.InfoContext(ctx, "payment captured", "amount", 42)
	logger.InfoContext(context.Background(), "cache warmed") // no trace in ctx: no trace_id
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
