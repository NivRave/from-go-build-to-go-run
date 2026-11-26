// External (black-box) tests: package split_test can use only the exported
// API, exactly like a real caller. The main reason this package kind exists
// is to break import cycles: split_test may import packages that themselves
// import split.
package split_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/NivRave/from-go-build-to-go-run/examples/023-testing-the-go-way/split"
)

// assertEqual keeps the assertions short. t.Helper() makes a failure report
// the caller's line instead of this one (slide p06).
func assertEqual(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) { // type-safe, no reflection (Go 1.21)
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		sep   string
		want  []string
	}{
		{"simple", "a/b/c", "/", []string{"a", "b", "c"}},
		{"no sep", "abc", "/", []string{"abc"}}, // run alone with -run 'TestSplit/no_sep'
		{"trailing", "a/", "/", []string{"a", ""}},
		{"leading", "/a", "/", []string{"", "a"}},
		{"empty input", "", "/", []string{""}},
		{"multi-char sep", "a::b::c", "::", []string{"a", "b", "c"}},
		{"empty sep", "abc", "", []string{"abc"}},
	}

	for _, tt := range tests {
		// No `tt := tt` copy: since Go 1.22 every iteration has its own tt,
		// so the parallel subtests below can't all see the last case.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertEqual(t, split.Split(tt.input, tt.sep), tt.want)
		})
	}
}

// Example functions are tests too: go test checks the output, and godoc
// shows them next to Split.
func ExampleSplit() {
	fmt.Printf("%q\n", split.Split("a/b/c", "/"))
	// Output: ["a" "b" "c"]
}
