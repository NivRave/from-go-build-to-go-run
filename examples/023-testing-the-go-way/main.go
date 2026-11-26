// Command testing-the-go-way uses the split package from this part. The
// interesting files are the tests in ./split; this program only shows what
// the code under test does.
//
// Slides: slides/023-testing-the-go-way.pdf
//
// Run:
//
//	go run ./examples/023-testing-the-go-way
//	go test -v ./examples/023-testing-the-go-way/...
//	go test -run 'TestSplit/no_sep' -v ./examples/023-testing-the-go-way/split
//	go test -cover ./examples/023-testing-the-go-way/...
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/NivRave/from-go-build-to-go-run/examples/023-testing-the-go-way/split"
)

func main() {
	section("1. split.Split, the function under test (slide p03)")
	splitCases()

	section("2. split.Service with an injected Storer (slide p07)")
	serviceDemo()

	fmt.Println("\nNow run the tests: go test -v ./examples/023-testing-the-go-way/...")
}

func splitCases() {
	for _, c := range []struct{ s, sep string }{
		{"a/b/c", "/"},
		{"abc", "/"},
		{"a/", "/"},
		{"a::b::c", "::"},
	} {
		fmt.Printf("Split(%q, %q) = %q\n", c.s, c.sep, split.Split(c.s, c.sep))
	}
}

// mapStore is a real (in-memory) Storer.
type mapStore map[string]string

var errNoRecord = errors.New("no such record")

func (m mapStore) Load(_ context.Context, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", errNoRecord
	}
	return v, nil
}

func serviceDemo() {
	svc := split.NewService(mapStore{"user:1": "Ada,Lovelace,1815"})
	fields, err := svc.Fields(context.Background(), "user:1")
	fmt.Printf("Fields(user:1) = %q, err = %v\n", fields, err)
	fields, err = svc.Fields(context.Background(), "user:2")
	fmt.Printf("Fields(user:2) = %q, err = %v\n", fields, err)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
