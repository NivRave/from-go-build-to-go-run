package main

import (
	"errors"
	"testing"
)

// FuzzParse states properties that must hold for every input, then lets the
// fuzzer search for a counterexample.
//
// Plain `go test` runs only the seed corpus below (and anything saved in
// testdata/fuzz/FuzzParse/), as ordinary regression tests. Real fuzzing,
// coverage-guided mutation of the seeds, needs -fuzz:
//
//	go test -fuzz=FuzzParse -fuzztime=10s ./examples/024-advanced-testing
//
// A failing input is written to testdata/fuzz/FuzzParse/ and from then on
// replays on every plain `go test`.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"key=value",
		"port=8080",
		" spaced = out ",
		"a=b=c",
		"=",
		"=value",
		"no-equals",
		"",
		"bad key=1",
		"ключ=значение",
		"\xff=\xfe", // invalid UTF-8
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Property 1: never panic. If Parse panics, the fuzzer reports it
		// together with the input that caused it.
		kv, err := Parse(input)

		// Property 2: every failure is a syntax error.
		if err != nil {
			if !errors.Is(err, ErrSyntax) {
				t.Fatalf("Parse(%q) error %v does not wrap ErrSyntax", input, err)
			}
			return
		}

		// Property 3: round trip. Rendering a parsed pair and parsing it
		// again gives the same pair.
		again, err := Parse(kv.String())
		if err != nil {
			t.Fatalf("Parse(%q) = %#v, but re-parsing %q failed: %v", input, kv, kv.String(), err)
		}
		if again != kv {
			t.Fatalf("round trip changed the pair: %q -> %#v -> %q -> %#v", input, kv, kv.String(), again)
		}
	})
}
