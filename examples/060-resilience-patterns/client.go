package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrAITimeout is the cause attached to a per-call deadline, so callers can
// tell "the provider was too slow" apart from "our caller gave up".
var ErrAITimeout = errors.New("AI provider took too long")

// StatusError is a non-2xx response from the provider.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("provider returned %d", e.Code) }

// AIClient calls the provider over HTTP.
type AIClient struct {
	BaseURL string
	HTTP    *http.Client
	Timeout time.Duration // per call
}

// CallAI is the fixed version of slide p03. The deadline lives in the
// context, and http.NewRequestWithContext ties the request to it: when the
// deadline passes, the transport closes the connection and the provider's
// r.Context() is cancelled, so the provider stops working too. No select, no
// extra goroutine.
func (c *AIClient) CallAI(ctx context.Context, prompt string) (string, error) {
	return c.CallAIIdempotent(ctx, "", prompt)
}

// CallAIIdempotent is CallAI with an Idempotency-Key header. Reusing one key
// for every retry of the same logical operation lets the provider recognise
// a repeat and avoid charging twice (slide p06).
func (c *AIClient) CallAIIdempotent(ctx context.Context, key, prompt string) (string, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.Timeout, ErrAITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/generate", strings.NewReader(prompt))
	if err != nil {
		return "", err
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			// Report why the context ended (ErrAITimeout, or the caller's
			// own cause) rather than the transport's wrapped message.
			return "", fmt.Errorf("call AI: %w", context.Cause(ctx))
		}
		return "", fmt.Errorf("call AI: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("call AI: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("call AI: %w", &StatusError{Code: resp.StatusCode})
	}
	return string(body), nil
}

// generateNoCtx stands in for the slide's aiClient.GenerateResponse: it takes
// no context, so once started nothing can cancel it.
func (c *AIClient) generateNoCtx(prompt string) string {
	resp, err := c.HTTP.Post(c.BaseURL+"/v1/generate", "text/plain", strings.NewReader(prompt))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// CallFlakyAI is slide p03's version, kept to show its flaw. It returns on
// time, but the goroutine and the HTTP request it started keep running: the
// provider finishes the work, and charges for it, after the caller has
// already reported a timeout. That is the "ghost load" from slide p02.
func (c *AIClient) CallFlakyAI(ctx context.Context, timeout time.Duration) (string, error) {
	resChan := make(chan string, 1) // buffered, so the goroutine can always send and exit

	go func() {
		resChan <- c.generateNoCtx("quarterly report")
	}()

	select {
	case res := <-resChan:
		return res, nil
	case <-time.After(timeout): // hard deadline
		return "", ErrAITimeout
	case <-ctx.Done(): // user gave up
		return "", ctx.Err()
	}
}
