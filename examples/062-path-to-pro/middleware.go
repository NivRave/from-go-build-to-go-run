package main

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// SecurityMiddleware is slide p07's middleware, updated. X-Frame-Options is
// superseded by the CSP frame-ancestors directive; browsers that understand
// CSP use it, older ones fall back to X-Frame-Options, so send both.
func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// tokenBucket is a rate limiter in about 30 lines of standard library, the
// "write the 50 lines yourself" advice from slide p04. The bucket holds up
// to burst tokens and refills at rate tokens per second; each request takes
// one.
type tokenBucket struct {
	mu     sync.Mutex
	tokens float64
	burst  float64
	rate   float64 // tokens per second
	last   time.Time
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{tokens: float64(burst), burst: float64(burst), rate: rate, last: time.Now()}
}

// take reports whether a token was available and, if not, how long until
// one will be.
func (b *tokenBucket) take(now time.Time) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = min(b.burst, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
}

// RateLimit rejects requests beyond the bucket's rate with 429 and a
// Retry-After header. One bucket for the whole server keeps the example
// short; production limiters usually keep one per client or API key
// (golang.org/x/time/rate is the usual library).
func RateLimit(rate float64, burst int) func(http.Handler) http.Handler {
	bucket := newTokenBucket(rate, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := bucket.take(time.Now())
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CrossOriginProtection wraps Go 1.25's http.CrossOriginProtection: it
// rejects state-changing requests (POST, PUT, ...) that a browser marks as
// coming from another origin (Sec-Fetch-Site, or an Origin header that
// doesn't match Host), which is CSRF protection without tokens. Requests
// from non-browser clients, which send neither header, pass.
func CrossOriginProtection(next http.Handler) http.Handler {
	return http.NewCrossOriginProtection().Handler(next)
}
