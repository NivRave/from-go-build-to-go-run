package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("GET /healthz = %d %q, want 200 \"ok\"", rec.Code, rec.Body.String())
	}
}

func TestInfo(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/info", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /info = %d, want 200", rec.Code)
	}
	var got info
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("GET /info: invalid JSON: %v", err)
	}
	if got.GoVersion != runtime.Version() || got.GOMAXPROCS < 1 || got.NumCPU < 1 {
		t.Errorf("GET /info = %+v", got)
	}
}
