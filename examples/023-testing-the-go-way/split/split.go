// Package split is the small library tested in Part 023. Its tests, not its
// code, are the point: see split_test.go (black-box), internal_test.go
// (white-box), service_test.go (dependency injection) and main_test.go
// (TestMain).
package split

// Split slices s into all substrings separated by sep and returns them. It
// behaves like strings.Split for a non-empty sep, except that an empty sep
// returns []string{s} instead of splitting into UTF-8 characters.
func Split(s, sep string) []string {
	if sep == "" {
		return []string{s}
	}
	var parts []string
	for {
		before, after, found := cut(s, sep)
		parts = append(parts, before)
		if !found {
			return parts
		}
		s = after
	}
}

// cut is the unexported helper behind Split. External tests can't call it;
// internal_test.go (package split) can.
func cut(s, sep string) (before, after string, found bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}
