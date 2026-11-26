package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// TextGenerator is defined where it is used, by the code that needs it, and
// says only what this service needs from any AI provider (slide p03). No
// vendor SDK type appears in the business logic, so switching providers is a
// configuration change: see NewGenerator and the -provider flag.
type TextGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// SummarizeReport is the slide's business function, completed: it returns
// the summary and an error instead of nothing.
func SummarizeReport(ctx context.Context, gen TextGenerator, data string) (string, error) {
	data = strings.TrimSpace(data)
	if data == "" {
		return "", fmt.Errorf("summarize: empty report")
	}
	summary, err := gen.Generate(ctx, "Summarize in one line: "+data)
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}
	return summary, nil
}

// The two fake providers stand in for real vendors. Each simulates network
// latency and respects cancellation, as a real client would.

// echoProvider returns the first five words of the report.
type echoProvider struct{ latency time.Duration }

func (p echoProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := sleep(ctx, p.latency); err != nil {
		return "", err
	}
	words := strings.Fields(reportText(prompt))
	return strings.Join(words[:min(5, len(words))], " "), nil
}

// reverseProvider returns the report's words in reverse order. It is
// deliberately different from echoProvider so a swap is visible.
type reverseProvider struct{ latency time.Duration }

func (p reverseProvider) Generate(ctx context.Context, prompt string) (string, error) {
	if err := sleep(ctx, p.latency); err != nil {
		return "", err
	}
	words := strings.Fields(reportText(prompt))
	slices.Reverse(words)
	return strings.Join(words, " "), nil
}

// reportText strips SummarizeReport's instruction prefix from the prompt.
func reportText(prompt string) string {
	_, text, found := strings.Cut(prompt, ": ")
	if !found {
		return prompt
	}
	return text
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// NewGenerator picks a provider by name: the only place that knows the
// concrete types.
func NewGenerator(name string, latency time.Duration) (TextGenerator, error) {
	switch name {
	case "echo":
		return echoProvider{latency: latency}, nil
	case "reverse":
		return reverseProvider{latency: latency}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q (want echo or reverse)", name)
	}
}
