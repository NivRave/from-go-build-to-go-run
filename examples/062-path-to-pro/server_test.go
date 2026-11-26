package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// "Done" includes tests (slide p11). These call the handler directly with
// httptest.NewRecorder: no network, no ports, and they run in parallel.

func testConfig(t *testing.T, provider string) Config {
	t.Helper()
	gen, err := NewGenerator(provider, 0)
	if err != nil {
		t.Fatal(err)
	}
	return Config{Provider: provider, Gen: gen, RateLimit: 1000, Burst: 1000}
}

func request(t *testing.T, h http.Handler, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const testBody = `{"report": "one two three four five six"}`

func TestVersions(t *testing.T) {
	t.Parallel()
	h := NewHandler(testConfig(t, "echo"))

	t.Run("v1", func(t *testing.T) {
		t.Parallel()
		rec := request(t, h, http.MethodPost, "/v1/summarize", testBody, nil)
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("status %d, body %q, err %v", rec.Code, rec.Body, err)
		}
		if got["summary"] != "one two three four five" || len(got) != 1 {
			t.Errorf("v1 body = %v, want only the summary", got)
		}
	})

	t.Run("v2 adds provider", func(t *testing.T) {
		t.Parallel()
		rec := request(t, h, http.MethodPost, "/v2/summarize", testBody, nil)
		var got summaryV2
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("status %d, body %q, err %v", rec.Code, rec.Body, err)
		}
		if got.Summary != "one two three four five" || got.Provider != "echo" {
			t.Errorf("v2 body = %+v", got)
		}
	})

	t.Run("v0 is gone", func(t *testing.T) {
		t.Parallel()
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			rec := request(t, h, method, "/v0/summarize", testBody, nil)
			if rec.Code != http.StatusGone {
				t.Errorf("%s /v0: status %d, want 410", method, rec.Code)
			}
			want := map[string]string{
				"Deprecation": "@1735689600",
				"Sunset":      "Thu, 01 Jan 2026 00:00:00 GMT",
				"Link":        `</v2/summarize>; rel="successor-version"`,
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s /v0: %s = %q, want %q", method, k, got, v)
				}
			}
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		t.Parallel()
		if rec := request(t, h, http.MethodGet, "/v1/summarize", "", nil); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET /v1: status %d, want 405", rec.Code)
		}
	})
}

func TestBadRequests(t *testing.T) {
	t.Parallel()
	h := NewHandler(testConfig(t, "echo"))
	tests := []struct {
		name string
		body string
		want int
	}{
		{"invalid JSON", `{"report":`, http.StatusBadRequest},
		{"missing report", `{}`, http.StatusBadRequest},
		{"too large", `{"report": "` + strings.Repeat("x", maxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if rec := request(t, h, http.MethodPost, "/v1/summarize", tt.body, nil); rec.Code != tt.want {
				t.Errorf("status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	h := NewHandler(testConfig(t, "echo"))
	want := map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
	}
	// Every response carries them, errors included.
	for _, path := range []string{"/v1/summarize", "/v0/summarize", "/nope"} {
		rec := request(t, h, http.MethodPost, path, testBody, nil)
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("POST %s: %s = %q, want %q", path, k, got, v)
			}
		}
	}
}

func TestCrossOriginProtection(t *testing.T) {
	t.Parallel()
	h := NewHandler(testConfig(t, "echo"))
	tests := []struct {
		name   string
		header map[string]string
		want   int
	}{
		{"non-browser client", nil, http.StatusOK},
		{"same origin", map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"cross site", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"foreign Origin", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if rec := request(t, h, http.MethodPost, "/v1/summarize", testBody, tt.header); rec.Code != tt.want {
				t.Errorf("status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	// synctest: the bucket refills with fake time, so the test doesn't wait.
	synctest.Test(t, func(t *testing.T) {
		cfg := testConfig(t, "echo")
		cfg.RateLimit, cfg.Burst = 2, 3
		h := NewHandler(cfg)
		post := func() *httptest.ResponseRecorder {
			return request(t, h, http.MethodPost, "/v1/summarize", testBody, nil)
		}

		for i := range 3 {
			if rec := post(); rec.Code != http.StatusOK {
				t.Fatalf("request %d within burst: status %d", i+1, rec.Code)
			}
		}
		rec := post()
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "1" {
			t.Fatalf("over burst: status %d, Retry-After %q; want 429 and 1", rec.Code, rec.Header().Get("Retry-After"))
		}

		time.Sleep(500 * time.Millisecond) // one token at 2/s
		if rec := post(); rec.Code != http.StatusOK {
			t.Errorf("after refill: status %d, want 200", rec.Code)
		}
		if rec := post(); rec.Code != http.StatusTooManyRequests {
			t.Errorf("bucket empty again: status %d, want 429", rec.Code)
		}
	})
}

func TestGenerators(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider, report, want string
	}{
		{"echo", "a b c d e f g", "a b c d e"},
		{"echo", "short one", "short one"},
		{"reverse", "a b c", "c b a"},
	}
	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.report, func(t *testing.T) {
			t.Parallel()
			gen, err := NewGenerator(tt.provider, 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := SummarizeReport(t.Context(), gen, tt.report)
			if err != nil || got != tt.want {
				t.Errorf("SummarizeReport = %q, %v; want %q", got, err, tt.want)
			}
		})
	}

	if _, err := NewGenerator("openai", 0); err == nil {
		t.Error("NewGenerator(openai) succeeded, want an error")
	}
	gen, _ := NewGenerator("echo", 0)
	if _, err := SummarizeReport(t.Context(), gen, "   "); err == nil {
		t.Error("empty report: want an error")
	}
}

// fakeGenerator shows the payoff of the consumer-side interface: tests
// substitute any behaviour without touching a real provider.
type fakeGenerator struct{ err error }

func (f fakeGenerator) Generate(context.Context, string) (string, error) { return "fake", f.err }

func TestFakeGenerator(t *testing.T) {
	t.Parallel()
	ok := NewHandler(Config{Provider: "fake", Gen: fakeGenerator{}, RateLimit: 10, Burst: 10})
	if rec := request(t, ok, http.MethodPost, "/v2/summarize", testBody, nil); !strings.Contains(rec.Body.String(), `"summary":"fake"`) {
		t.Errorf("fake generator: body %q", rec.Body)
	}
	failing := NewHandler(Config{Provider: "fake", Gen: fakeGenerator{err: errors.New("quota exceeded")}, RateLimit: 10, Burst: 10})
	if rec := request(t, failing, http.MethodPost, "/v1/summarize", testBody, nil); rec.Code != http.StatusBadGateway {
		t.Errorf("failing generator: status %d, want 502", rec.Code)
	}
}

func TestGeneratorRespectsCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gen, _ := NewGenerator("reverse", time.Minute)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		start := time.Now()
		_, err := SummarizeReport(ctx, gen, "a b c")
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != time.Second {
			t.Errorf("err = %v after %v, want DeadlineExceeded after 1s", err, time.Since(start))
		}
	})
}

// Examples for the deterministic demos in main.go.

func Example_versioning() {
	gen, _ := NewGenerator("echo", 0)
	versioning(Config{Provider: "echo", Gen: gen, RateLimit: 100, Burst: 100})
	// Output:
	// POST /v1/summarize -> 200 {"summary":"Revenue grew 12 percent while"}
	// POST /v2/summarize -> 200 {"summary":"Revenue grew 12 percent while","provider":"echo"}
	// POST /v0/summarize -> 410 v0 was removed; use /v2/summarize
	//   Deprecation: @1735689600
	//   Sunset: Thu, 01 Jan 2026 00:00:00 GMT
	//   Link: </v2/summarize>; rel="successor-version"
	// GET /v1/summarize -> 405 (the pattern says POST)
}

func Example_providerSwap() {
	providerSwap()
	// Output:
	// -provider echo    -> "Revenue grew 12 percent while", err = <nil>
	// -provider reverse -> "third a by fell tickets support while percent 12 grew Revenue", err = <nil>
	// -provider openai  -> unknown provider "openai" (want echo or reverse)
}

func Example_middleware() {
	gen, _ := NewGenerator("echo", 0)
	middleware(Config{Provider: "echo", Gen: gen, RateLimit: 100, Burst: 100})
	// Output:
	// Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
	// X-Frame-Options: DENY
	// X-Content-Type-Options: nosniff
	// Referrer-Policy: no-referrer
	// cross-site POST -> 403 cross-origin request detected from Sec-Fetch-Site header
	// 5 requests against burst 3 -> [200 200 200 429 429], Retry-After: 1
}
