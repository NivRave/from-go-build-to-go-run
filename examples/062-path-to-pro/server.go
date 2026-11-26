package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Config is everything that varies between deployments, including which AI
// provider to use.
type Config struct {
	Provider  string        // "echo" or "reverse"
	Gen       TextGenerator // built from Provider by NewGenerator
	RateLimit float64       // requests per second
	Burst     int
}

// maxBodyBytes caps request bodies; anything larger gets 413.
const maxBodyBytes = 64 << 10

// The deprecation dates for v0. Deprecation (RFC 9745) is a structured-field
// date, "@" plus Unix seconds; Sunset (RFC 8594) is an HTTP date. Both are in
// the past, because v0 is already gone.
var (
	v0Deprecated = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	v0Sunset     = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// NewHandler builds the routes and wraps them in the middleware chain.
func NewHandler(cfg Config) http.Handler {
	mux := http.NewServeMux()

	// Go 1.22 patterns: method and path in one string, so path versioning
	// is one line per version (slide p06).
	mux.HandleFunc("POST /v1/summarize", func(w http.ResponseWriter, r *http.Request) {
		summary, ok := summarize(w, r, cfg.Gen)
		if ok {
			writeJSON(w, http.StatusOK, summaryV1{Summary: summary})
		}
	})
	mux.HandleFunc("POST /v2/summarize", func(w http.ResponseWriter, r *http.Request) {
		summary, ok := summarize(w, r, cfg.Gen)
		if ok {
			writeJSON(w, http.StatusOK, summaryV2{Summary: summary, Provider: cfg.Provider})
		}
	})
	// No method in the pattern: every method on v0 is gone.
	mux.HandleFunc("/v0/summarize", func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h.Set("Deprecation", "@"+strconv.FormatInt(v0Deprecated.Unix(), 10))
		h.Set("Sunset", v0Sunset.Format(http.TimeFormat))
		h.Set("Link", `</v2/summarize>; rel="successor-version"`)
		http.Error(w, "v0 was removed; use /v2/summarize", http.StatusGone)
	})

	// Outermost first: security headers go on every response, including
	// 403 and 429; then rate limiting; then CSRF protection.
	var h http.Handler = mux
	h = CrossOriginProtection(h)
	h = RateLimit(cfg.RateLimit, cfg.Burst)(h)
	h = SecurityMiddleware(h)
	return h
}

type summarizeRequest struct {
	Report string `json:"report"`
}

type summaryV1 struct {
	Summary string `json:"summary"`
}

// summaryV2 adds a field. Adding fields is backwards-compatible for most
// clients, but renaming or removing one is not, which is what new versions
// are for.
type summaryV2 struct {
	Summary  string `json:"summary"`
	Provider string `json:"provider"`
}

// summarize decodes the request and calls the generator. On failure it has
// already written the error response and returns false.
func summarize(w http.ResponseWriter, r *http.Request, gen TextGenerator) (string, bool) {
	// MaxBytesReader stops reading after maxBodyBytes, so a client can't make
	// the server buffer an arbitrarily large body.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req summarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, "request body larger than "+strconv.FormatInt(maxErr.Limit, 10)+" bytes", http.StatusRequestEntityTooLarge)
			return "", false
		}
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return "", false
	}
	if strings.TrimSpace(req.Report) == "" {
		http.Error(w, `"report" is required`, http.StatusBadRequest)
		return "", false
	}
	summary, err := SummarizeReport(r.Context(), gen, req.Report)
	if err != nil {
		http.Error(w, "generation failed", http.StatusBadGateway)
		return "", false
	}
	return summary, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// harden sets the server timeouts that matter more than any header
// (slide p07). ReadHeaderTimeout is the one that stops Slowloris: a client
// that trickles its headers in gets disconnected.
func harden(srv *http.Server) {
	srv.ReadHeaderTimeout = 2 * time.Second
	srv.ReadTimeout = 10 * time.Second
	srv.WriteTimeout = 30 * time.Second
	srv.IdleTimeout = 60 * time.Second
	srv.MaxHeaderBytes = 16 << 10
}
