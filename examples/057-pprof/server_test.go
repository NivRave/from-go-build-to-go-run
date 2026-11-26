package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMuxSeparation checks the point of slide p03 done safely: the app mux
// does not serve pprof, and the pprof mux does.
func TestMuxSeparation(t *testing.T) {
	t.Parallel()
	app, prof := newAppMux(), newPprofMux()

	tests := []struct {
		name     string
		handler  http.Handler
		path     string
		wantCode int
		wantBody string
	}{
		{"app has no pprof index", app, "/debug/pprof/", http.StatusNotFound, ""},
		{"app has no heap profile", app, "/debug/pprof/heap", http.StatusNotFound, ""},
		{"app serves /cpu", app, "/cpu", http.StatusOK, "hashed"},
		{"pprof index", prof, "/debug/pprof/", http.StatusOK, "goroutine"},
		{"pprof named profile", prof, "/debug/pprof/heap?debug=1", http.StatusOK, "heap profile"},
		{"pprof has no app routes", prof, "/cpu", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.wantCode {
				t.Fatalf("GET %s: status %d, want %d", tt.path, rec.Code, tt.wantCode)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("GET %s: body does not contain %q", tt.path, tt.wantBody)
			}
		})
	}
}
