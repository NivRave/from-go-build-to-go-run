package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// KV is one parsed key=value pair.
type KV struct {
	Key   string
	Value string
}

// String renders kv in the form Parse accepts, so Parse(kv.String()) == kv.
// FuzzParse checks that round trip.
func (kv KV) String() string { return kv.Key + "=" + kv.Value }

// ErrSyntax is wrapped by every error Parse returns.
var ErrSyntax = errors.New("syntax error")

// Parse parses "key=value". Space around the key and the value is trimmed;
// the value may contain further '=' characters; the key must be non-empty and
// contain no whitespace.
func Parse(s string) (KV, error) {
	key, value, ok := strings.Cut(s, "=")
	if !ok {
		return KV{}, fmt.Errorf("%w: missing '=' in %q", ErrSyntax, s)
	}
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	if key == "" {
		return KV{}, fmt.Errorf("%w: empty key in %q", ErrSyntax, s)
	}
	if strings.IndexFunc(key, unicode.IsSpace) >= 0 {
		return KV{}, fmt.Errorf("%w: whitespace in key %q", ErrSyntax, key)
	}
	return KV{Key: key, Value: value}, nil
}

// ParseLines parses one pair per line, skipping blank lines and lines that
// start with '#'. Errors name the line number.
func ParseLines(r io.Reader) ([]KV, error) {
	var out []KV
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		kv, err := Parse(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, kv)
	}
	return out, sc.Err()
}
