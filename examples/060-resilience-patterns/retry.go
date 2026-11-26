package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

// Backoff is a retry policy: exponential backoff capped at Max, with full
// jitter (slide p04).
type Backoff struct {
	Base time.Duration // the cap before the first retry
	Max  time.Duration // the cap never grows beyond this

	// Jitter picks the actual wait given the current cap. The default is
	// full jitter, a uniformly random duration in [0, cap), which spreads a
	// thousand clients retrying at once across the whole window instead of
	// waking them all together (the "thundering herd").
	Jitter func(capDelay time.Duration) time.Duration

	// OnRetry, if set, is called before each wait. It is the hook for
	// logging and for the retries_total metric (slide p08).
	OnRetry func(attempt int, err error, wait time.Duration)
}

// DefaultBackoff is the policy RetryWithBackoff uses.
var DefaultBackoff = Backoff{Base: 100 * time.Millisecond, Max: 2 * time.Second}

// RetryWithBackoff is the fixed version of slide p05:
//
//   - fn receives ctx, so each attempt can be cancelled;
//   - the wait between attempts is exponential with full jitter;
//   - there is no wait after the final attempt;
//   - the last error is wrapped into the result instead of discarded.
func RetryWithBackoff(ctx context.Context, attempts int, fn func(context.Context) error) error {
	return DefaultBackoff.Retry(ctx, attempts, fn)
}

// Retry runs fn up to attempts times, waiting between failures.
func (b Backoff) Retry(ctx context.Context, attempts int, fn func(context.Context) error) error {
	jitter := b.Jitter
	if jitter == nil {
		jitter = fullJitter
	}
	var err error
	for attempt := range attempts {
		if err = fn(ctx); err == nil {
			return nil
		}
		if attempt == attempts-1 {
			break // the slide sleeps here, for nothing
		}
		if ctx.Err() != nil {
			break // the caller gave up during the attempt
		}

		wait := jitter(b.capFor(attempt))
		if b.OnRetry != nil {
			b.OnRetry(attempt+1, err, wait)
		}
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return fmt.Errorf("retry aborted after %d attempts: %w; last error: %w", attempt+1, context.Cause(ctx), err)
		}
	}
	if ctx.Err() != nil {
		return fmt.Errorf("retry aborted: %w; last error: %w", context.Cause(ctx), err)
	}
	return fmt.Errorf("max retries reached (%d attempts): %w", attempts, err)
}

// capFor returns Base * 2^attempt, capped at Max.
func (b Backoff) capFor(attempt int) time.Duration {
	d := b.Base
	for range attempt {
		if d >= b.Max/2 {
			return b.Max
		}
		d *= 2
	}
	return min(d, b.Max)
}

// fullJitter returns a random duration in [0, capDelay). rand.N (Go 1.22)
// works directly with time.Duration.
func fullJitter(capDelay time.Duration) time.Duration {
	if capDelay <= 0 {
		return 0
	}
	return rand.N(capDelay)
}
