package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Loose key-value pairs versus LogAttrs, against two handlers:
//
//   - slog.DiscardHandler (Go 1.24+): Enabled reports false, so nothing is
//     formatted. What remains is the cost at the call site, which is where
//     the []any boxing happens.
//   - a JSONHandler writing to io.Discard: the full formatting path.
//
// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/049-slog

// Package-level variables rather than constants: the compiler boxes a
// constant into an interface without allocating, which would hide the very
// cost being measured. Real log calls log variables.
var (
	benchSize = 1024
	benchUser = "niv"
	benchPath = "/files/a.txt"
)

func benchLogger(b *testing.B, name string) *slog.Logger {
	b.Helper()
	switch name {
	case "discard":
		return slog.New(slog.DiscardHandler)
	case "json":
		return slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	b.Fatalf("unknown handler %q", name)
	return nil
}

func BenchmarkLooseArgs(b *testing.B) {
	for _, h := range []string{"discard", "json"} {
		b.Run(h, func(b *testing.B) {
			logger := benchLogger(b, h)
			b.ReportAllocs()
			for b.Loop() {
				logger.Info("upload", "size", benchSize, "user", benchUser, "path", benchPath)
			}
		})
	}
}

func BenchmarkLogAttrs(b *testing.B) {
	ctx := context.Background()
	for _, h := range []string{"discard", "json"} {
		b.Run(h, func(b *testing.B) {
			logger := benchLogger(b, h)
			b.ReportAllocs()
			for b.Loop() {
				logger.LogAttrs(ctx, slog.LevelInfo, "upload",
					slog.Int("size", benchSize),
					slog.String("user", benchUser),
					slog.String("path", benchPath),
				)
			}
		})
	}
}
