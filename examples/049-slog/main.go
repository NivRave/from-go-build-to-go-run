// Command slog demonstrates structured logging with log/slog: JSON output,
// loose key-value pairs versus LogAttrs, groups and scoped loggers, a
// handler wrapper that adds a request ID from the context, changing the
// level at run time, redirecting the log package, and Go 1.26's
// MultiHandler.
//
// Slides: slides/049-slog.pdf
//
// Run:
//
//	go run ./examples/049-slog
//	go test -run='^$' -bench=. -benchmem ./examples/049-slog
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
)

func main() {
	section("1. JSON handler: loose key-values vs LogAttrs (slides p03-p04)")
	jsonBasics()

	section("2. Groups, With and WithGroup (slide p06)")
	groupsAndScopes()

	section("3. A ContextHandler adds request_id from the context (slide p05)")
	contextHandler()

	section("4. Changing the level at run time with slog.LevelVar (slide p08)")
	dynamicLevel()

	section("5. slog.SetDefault also redirects the log package (slide p07)")
	setDefault()

	section("6. Fan-out with slog.NewMultiHandler (Go 1.26, slide p07)")
	multiHandler()
}

// dropTime is a ReplaceAttr function that removes the top-level time
// attribute. JSONHandler always writes "time" first otherwise, which would
// make this output different on every run. Production logs keep it.
func dropTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return a
}

// newJSONLogger returns a JSON logger on w whose minimum level is read from
// level on every call.
func newJSONLogger(w io.Writer, level *slog.LevelVar) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: dropTime,
	}))
}

func jsonBasics() {
	var levelVar slog.LevelVar // zero value is LevelInfo
	logger := newJSONLogger(os.Stdout, &levelVar)

	// Loose key-value pairs: convenient. Each value is boxed into an `any`.
	logger.Info("incoming_request",
		"method", "GET",
		"path", "/users",
		"status", 200,
	)

	// LogAttrs takes typed Attrs only, so no []any is built. Typed Attrs can
	// also be passed to Info; it's LogAttrs that skips the boxing.
	logger.LogAttrs(context.Background(), slog.LevelInfo, "upload",
		slog.Int("size", 1024),
		slog.String("user", "niv"),
	)
}

func groupsAndScopes() {
	var levelVar slog.LevelVar
	logger := newJSONLogger(os.Stdout, &levelVar)

	// slog.Group nests attributes, so "user.id" can't collide with another id.
	logger.Info("user_update",
		slog.Group("user",
			slog.Int("id", 42),
			slog.String("email", "user@example.com"),
		),
	)

	// With pre-formats attributes once and adds them to every line.
	billing := logger.With("service", "billing")
	billing.Info("charged", "amount_cents", 999)

	// WithGroup puts everything logged afterwards under one key.
	httpLog := logger.WithGroup("http").With("method", "GET")
	httpLog.Info("request done", "status", 200)
}

func contextHandler() {
	var levelVar slog.LevelVar
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: &levelVar, ReplaceAttr: dropTime})
	logger := slog.New(ContextHandler{base})

	ctx := WithRequestID(context.Background(), "req-7f3a")

	logger.InfoContext(ctx, "fetching user", "user_id", 42)
	logger.With("component", "db").InfoContext(ctx, "query ok", "rows", 1) // still wrapped after With
	logger.Info("no context, no request_id")
}

func dynamicLevel() {
	var levelVar slog.LevelVar // starts at Info
	logger := newJSONLogger(os.Stdout, &levelVar)

	logger.Debug("cache miss", "key", "user:42") // dropped: below Info
	logger.Info("serving", "min_level", levelVar.Level())

	// Set is safe to call from another goroutine (for example an admin
	// endpoint or a SIGHUP handler) while the logger is in use.
	levelVar.Set(slog.LevelDebug)
	logger.Debug("cache miss", "key", "user:42") // now emitted
	logger.Info("serving", "min_level", levelVar.Level())
}

func setDefault() {
	// Save and restore the global state, so this demo (and its Example test)
	// leaves the process as it found it.
	oldDefault, oldOut, oldFlags := slog.Default(), log.Writer(), log.Flags()
	defer func() {
		slog.SetDefault(oldDefault)
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	}()

	var levelVar slog.LevelVar
	slog.SetDefault(newJSONLogger(os.Stdout, &levelVar))

	slog.Info("via slog.Info", "who", "new code")
	log.Printf("via log.Printf from %s code", "legacy") // now JSON at INFO level
}

func multiHandler() {
	opts := &slog.HandlerOptions{ReplaceAttr: dropTime}
	logger := slog.New(slog.NewMultiHandler(
		slog.NewTextHandler(os.Stdout, opts), // e.g. a human-readable console
		slog.NewJSONHandler(os.Stdout, opts), // e.g. a file or collector
	))
	logger.Info("fan-out", "sinks", 2)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
