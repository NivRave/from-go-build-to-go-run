// Internal (white-box) tests: package split, so the unexported cut helper is
// reachable. Keep these for internals that are hard to reach through the
// public API; most tests should be black-box (split_test.go).
package split

import "testing"

func TestCut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		s, sep        string
		before, after string
		found         bool
	}{
		{"found in middle", "a/b", "/", "a", "b", true},
		{"first of many", "a/b/c", "/", "a", "b/c", true},
		{"not found", "abc", "/", "abc", "", false},
		{"sep at end", "a/", "/", "a", "", true},
		{"sep longer than s", "a", "::", "a", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before, after, found := cut(tt.s, tt.sep)
			if before != tt.before || after != tt.after || found != tt.found {
				t.Errorf("cut(%q, %q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.s, tt.sep, before, after, found, tt.before, tt.after, tt.found)
			}
		})
	}
}
