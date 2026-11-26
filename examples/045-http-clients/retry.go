package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// Backoff schedule: attempt n (counting from 0) waits a random duration in
// [0, min(backoffMax, backoffBase<<n)). That is "full jitter": synchronised
// clients spread out instead of retrying in lockstep, which the slide's fixed
// 100ms -> 200ms -> 400ms schedule doesn't do.
const (
	backoffBase = 100 * time.Millisecond
	backoffMax  = 2 * time.Second
)

// Retry calls fn up to attempts times and returns nil on the first success.
// Compared with retryFetch on slide p07, it:
//
//   - returns the last error, wrapped with %w, instead of swallowing it;
//   - doesn't sleep after the final attempt;
//   - waits with full jitter (rand.N from math/rand/v2);
//   - waits on ctx as well, so cancellation interrupts the backoff;
//   - stops at once on an error marked Permanent;
//   - uses the server's delay when the error carries one (RetryAfter).
func Retry(ctx context.Context, attempts int, fn func(context.Context) error) error {
	var lastErr error
	for attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return retryStopped(attempt, err, lastErr)
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if pe, ok := errors.AsType[*permanentError](lastErr); ok {
			return fmt.Errorf("attempt %d: permanent failure: %w", attempt+1, pe.err)
		}
		if attempt == attempts-1 {
			break // no point waiting after the last attempt
		}

		wait := jitter(attempt)
		if ra, ok := errors.AsType[*retryAfterError](lastErr); ok {
			wait = ra.after
		}
		if err := sleep(ctx, wait); err != nil {
			return retryStopped(attempt+1, err, lastErr)
		}
	}
	return fmt.Errorf("giving up after %d attempts: %w", attempts, lastErr)
}

func retryStopped(done int, ctxErr, lastErr error) error {
	if lastErr == nil {
		return fmt.Errorf("retry stopped before the first attempt: %w", ctxErr)
	}
	return fmt.Errorf("retry stopped after %d attempts: %w (last error: %w)", done, ctxErr, lastErr)
}

// jitter returns the full-jitter wait before retry number attempt+1.
func jitter(attempt int) time.Duration {
	ceiling := min(backoffMax, backoffBase<<attempt)
	return rand.N(ceiling) // rand.N panics on 0; ceiling is always >= backoffBase
}

// sleep waits for d or until ctx is done, whichever comes first. Unlike
// time.Sleep it can be interrupted. (Since Go 1.23 an unstopped timer is
// garbage-collected, but stopping it is still tidy.)
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// permanentError marks a failure that retrying can't fix (a 404, a bad URL).
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent tells Retry to stop immediately and return err.
func Permanent(err error) error { return &permanentError{err: err} }

// retryAfterError carries the delay a server asked for in Retry-After.
type retryAfterError struct {
	err   error
	after time.Duration
}

func (e *retryAfterError) Error() string {
	return fmt.Sprintf("%v (Retry-After: %v)", e.err, e.after)
}
func (e *retryAfterError) Unwrap() error { return e.err }

// RetryAfter tells Retry to wait d before the next attempt instead of using
// its own backoff.
func RetryAfter(err error, d time.Duration) error { return &retryAfterError{err: err, after: d} }

// parseRetryAfter understands both forms of the header: delay-seconds and an
// HTTP date.
func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(0, time.Until(t)), true
	}
	return 0, false
}

// drain reads at most 64 KiB of whatever is left of a response body.
// Reading to EOF before Close lets the transport reuse the connection; the
// cap stops a huge unwanted body from stalling the call. A body too big to
// drain costs one connection, which is cheaper than reading megabytes. Only
// the unread remainder is drained; a body you already consumed is at EOF.
func drain(body io.Reader) {
	_, _ = io.CopyN(io.Discard, body, 64<<10)
}

// fetch performs one GET and classifies the outcome for Retry:
//
//   - transport errors (timeouts, refused connections) are retryable, because
//     GET is idempotent;
//   - 429 and 5xx are retryable, honouring Retry-After;
//   - other 4xx are permanent.
func fetch(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", Permanent(err) // a malformed URL won't fix itself
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	// Without Close the connection (a file descriptor) leaks; without the
	// drain it can't go back to the keep-alive pool.
	defer func() {
		drain(resp.Body)
		resp.Body.Close()
	}()

	switch code := resp.StatusCode; {
	case code == http.StatusTooManyRequests || code >= 500:
		err := fmt.Errorf("server returned %s", resp.Status)
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return "", RetryAfter(err, d)
		}
		return "", err
	case code >= 400:
		return "", Permanent(fmt.Errorf("server returned %s", resp.Status))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
