// Command error-handling demonstrates adding context to errors: wrapping with
// %w, inspecting chains with errors.Is, errors.As and Go 1.26's
// errors.AsType, joining errors, logging once at the top, sanitising errors
// at the API edge, and propagating errors from goroutines with errgroup.
//
// Slides: slides/022-error-handling.pdf
//
// Run:
//
//	go run ./examples/022-error-handling
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// A config path that is guaranteed not to exist: a fresh temporary
	// directory with nothing in it.
	dir, err := os.MkdirTemp("", "022-error-handling-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.json")

	section("1. Wrap with %w, detect with errors.Is (slides p02-p03)")
	wrapAndIs(path)

	section("2. Extract details with errors.As and errors.AsType (slide p04)")
	asAndAsType(path)

	section("3. Multiple errors: errors.Join and multiple %w (slide p06)")
	joinErrors()

	section("4. Handle or return: log once, at the top (slide p05)")
	handleOrReturn()

	section("5. Sanitise at the API edge (slide p08)")
	sanitize()

	section("6. Concurrent errors with errgroup (slide p07)")
	concurrentErrors()
}

// ---- 1. wrap and Is -----------------------------------------------------------

// Config is what LoadConfig returns.
type Config struct {
	Port     int    `json:"port"`
	LogLevel string `json:"log_level"`
}

// DefaultConfig is the fallback when no config file exists.
var DefaultConfig = Config{Port: 8080, LogLevel: "info"}

// LoadConfig reads a JSON config file. It wraps with "load config: %w" and
// not the slide's "config: open %s: %w": the *fs.PathError from os.ReadFile
// already says "open <path>: ...", so repeating it would print the operation
// and path twice.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", filepath.Base(path), err)
	}
	return cfg, nil
}

func wrapAndIs(path string) {
	cfg, err := LoadConfig(path)
	// The full message contains OS-specific text ("no such file or
	// directory" on Unix, "The system cannot find the file specified." on
	// Windows), so the demo prints only the parts that are the same
	// everywhere.
	fmt.Println(`err starts with "load config: open ":`, strings.HasPrefix(err.Error(), "load config: open "))
	fmt.Println("err == fs.ErrNotExist:         ", err == fs.ErrNotExist) //nolint:errorlint // intentional: shows that == misses wrapped errors
	fmt.Println("errors.Is(err, fs.ErrNotExist):", errors.Is(err, fs.ErrNotExist))

	if errors.Is(err, fs.ErrNotExist) { // os.ErrNotExist is the same value
		cfg = DefaultConfig
	}
	fmt.Printf("using defaults: %+v\n", cfg)
}

// ---- 2. As and AsType -----------------------------------------------------------

func asAndAsType(path string) {
	_, err := LoadConfig(path)

	// The classic form: declare a target, pass its address.
	var perr *fs.PathError // os.PathError is an alias of fs.PathError
	if errors.As(err, &perr) {
		fmt.Printf("errors.As:     Op=%q Path=.../%s\n", perr.Op, filepath.Base(perr.Path))
	}

	// Go 1.26: the generic form returns the typed value directly.
	if perr, ok := errors.AsType[*fs.PathError](err); ok {
		fmt.Printf("errors.AsType: Op=%q Path=.../%s\n", perr.Op, filepath.Base(perr.Path))
	}
}

// ---- 3. Join and multiple %w ----------------------------------------------------

var (
	ErrRedisClose = errors.New("redis: close: broken pipe")
	ErrFileClose  = errors.New("file: close: disk full")
)

func closeDB() error    { return nil }
func closeRedis() error { return ErrRedisClose }
func closeFile() error  { return ErrFileClose }

func joinErrors() {
	// errors.Join drops nil errors and returns nil if all of them are nil.
	errs := errors.Join(closeDB(), closeRedis(), closeFile())
	fmt.Printf("errors.Join message (one line per error):\n%v\n", errs)
	fmt.Println("errors.Is(errs, ErrRedisClose):", errors.Is(errs, ErrRedisClose))
	fmt.Println("errors.Is(errs, ErrFileClose): ", errors.Is(errs, ErrFileClose))

	// Since Go 1.20 one fmt.Errorf may wrap several errors; the result is a
	// tree, and errors.Is searches every branch.
	both := fmt.Errorf("shutdown: %w; %w", ErrRedisClose, ErrFileClose)
	fmt.Println("multiple %w:", both)
	fmt.Println("errors.Is finds both:", errors.Is(both, ErrRedisClose) && errors.Is(both, ErrFileClose))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
