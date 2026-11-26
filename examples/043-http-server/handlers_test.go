package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeStore is injected into UserHandler; no database, no network.
type fakeStore map[string]string

func (f fakeStore) UserName(id string) (string, bool) {
	name, ok := f[id]
	return name, ok
}

func TestUserHandler(t *testing.T) {
	t.Parallel()
	h := &UserHandler{Store: fakeStore{"7": "Linus"}}

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantBody   string
	}{
		{"found", "/users?id=7", http.StatusOK, "user 7: Linus\n"},
		{"missing", "/users?id=8", http.StatusNotFound, "user not found\n"},
		{"no id", "/users", http.StatusNotFound, "user not found\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

// TestMuxRoutes goes through the local mux, so it also checks the routing.
func TestMuxRoutes(t *testing.T) {
	t.Parallel()
	mux := newMux(fakeStore{"1": "Ada"})

	tests := []struct {
		target     string
		wantStatus int
		wantBody   string
	}{
		{"/", http.StatusOK, "Welcome to the API\n"},
		{"/anything/else", http.StatusOK, "Welcome to the API\n"}, // "/" is a catch-all
		{"/health", http.StatusOK, "ok\n"},
		{"/users?id=1", http.StatusOK, "user 1: Ada\n"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))
			if rec.Code != tt.wantStatus || rec.Body.String() != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", rec.Code, rec.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}

// TestHomeHandlerImplicit200 shows that a handler that never calls
// WriteHeader still answers 200 (slide p04).
func TestHomeHandlerImplicit200(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	homeHandler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestNewServerSetsTimeouts(t *testing.T) {
	t.Parallel()
	srv := newServer(http.NewServeMux(), 1)
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 {
		t.Errorf("all four timeouts must be non-zero: %+v", srv)
	}
}

func Example_localMux() {
	localMux()
	// Output:
	// GET /             -> 200 Welcome to the API
	// GET /health       -> 200 ok
	// GET /users?id=1   -> 200 user 1: Ada
	// GET /users?id=99  -> 404 user not found
}

func Example_configuredServer() {
	if err := configuredServer(); err != nil {
		panic(err)
	}
	// Output:
	// GET /health     -> 200 OK ok
	// GET /users?id=2 -> 200 OK user 2: Grace
	// Serve returned "http: Server closed"; errors.Is(err, http.ErrServerClosed) = true
}
