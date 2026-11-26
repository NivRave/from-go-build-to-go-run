package main

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// tracer appends name-before / name-after around next.
func tracer(name string, trace *[]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*trace = append(*trace, name+"-before")
			next.ServeHTTP(w, r)
			*trace = append(*trace, name+"-after")
		})
	}
}

func TestChainOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		chain func(http.Handler, ...Middleware) http.Handler
		want  string
	}{
		{"Chain", Chain, "A-before B-before C-before handler C-after B-after A-after"},
		{"SlideChain", SlideChain, "C-before B-before A-before handler A-after B-after C-after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var trace []string
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trace = append(trace, "handler")
			})
			wrapped := tt.chain(h, tracer("A", &trace), tracer("B", &trace), tracer("C", &trace))
			wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			if got := strings.Join(trace, " "); got != tt.want {
				t.Errorf("trace = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuth(t *testing.T) {
	t.Parallel()
	var gotUser string
	h := Auth(map[string]string{"tok": "u1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, _ = UserFrom(r.Context())
	}))

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"not bearer", "Basic dTE6cHc=", http.StatusUnauthorized},
		{"unknown token", "Bearer nope", http.StatusUnauthorized},
		{"valid", "Bearer tok", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: the subtests share gotUser.
			gotUser = ""
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK && gotUser != "u1" {
				t.Errorf("UserFrom = %q, want u1", gotUser)
			}
		})
	}
}

func TestUserFromEmptyContext(t *testing.T) {
	t.Parallel()
	if id, ok := UserFrom(t.Context()); ok || id != "" {
		t.Errorf("UserFrom(empty) = %q, %t; want \"\", false", id, ok)
	}
}

func TestRecoveryReturns500(t *testing.T) {
	t.Parallel()
	h := Recovery(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestRecoveryRepanicsErrAbortHandler(t *testing.T) {
	t.Parallel()
	h := Recovery(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		v := recover()
		err, ok := v.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Errorf("recovered %v, want http.ErrAbortHandler re-panicked", v)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Error("ServeHTTP returned normally; want a panic")
}

func TestLoggingRecordsStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"explicit", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }, "status=418"},
		{"implicit via Write", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "hi") }, "status=200"},
		{"nothing written", func(w http.ResponseWriter, r *http.Request) {}, "status=200"},
		{"http.Error", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", http.StatusNotFound) }, "status=404"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			Logging(logger)(tt.handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("log %q does not contain %q", buf.String(), tt.want)
			}
		})
	}
}

func TestStatusRecorderUnwrap(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec}
	if err := http.NewResponseController(sr).Flush(); err != nil {
		t.Errorf("Flush through statusRecorder: %v", err)
	}
	if !rec.Flushed {
		t.Error("underlying recorder was not flushed")
	}
}

// TestAppRoutes drives the whole stack with a recorder: middleware plus mux.
func TestAppRoutes(t *testing.T) {
	t.Parallel()
	app := newApp(discardLogger())
	tests := []struct {
		target, token string
		wantStatus    int
		wantBody      string
	}{
		{"/users/7", "", http.StatusUnauthorized, "Unauthorized\n"},
		{"/users/7", "secret-token", http.StatusOK, "user 7 (requested by user-42)"},
		{"/api/v1/users/9", "secret-token", http.StatusOK, "v1 user 9"},
		{"/panic", "secret-token", http.StatusInternalServerError, "Internal Server Error\n"},
	}
	for _, tt := range tests {
		t.Run(tt.target+"/"+tt.token, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus || rec.Body.String() != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", rec.Code, rec.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}

func Example_chainOrder() {
	chainOrder()
	// Output:
	// Chain(h, A, B):        A-before B-before handler B-after A-after
	// SlideChain(h, A, B):   B-before A-before handler A-after B-after
}

func Example_loggingDemo() {
	loggingDemo()
	// Output:
	// handler: Flush through the wrapper, err = <nil>
	// level=INFO msg=request method=GET path=/coffee status=418
}

func Example_routes() {
	routes()
	// Output:
	// level=INFO msg=request method=GET path=/users/7 status=200
	// -> /users/7: 200 "user 7 (requested by user-42)"
	// level=INFO msg=request method=GET path=/api/v1/users/9 status=200
	// -> /api/v1/users/9: 200 "v1 user 9"
	// level=INFO msg=request method=GET path=/nope status=404
	// -> /nope: 404 "404 page not found"
}

func Example_recoveryAndAuth() {
	recoveryAndAuth()
	// Output:
	// level=INFO msg=request method=GET path=/users/7 status=401
	// -> /users/7: 401 "Unauthorized"
	// level=INFO msg=request method=GET path=/users/7 status=401
	// -> /users/7: 401 "Unauthorized"
	// level=ERROR msg="panic recovered" value="database handle is nil" path=/panic
	// level=INFO msg=request method=GET path=/panic status=500
	// -> /panic: 500 "Internal Server Error"
	// -> /abort: connection aborted, no response
}
