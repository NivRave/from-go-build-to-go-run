package main

import "strings"

// JoinNaive concatenates with +=. Strings are immutable, so every += copies
// everything built so far into a new allocation: O(n) allocations and
// O(n^2) bytes copied for n parts.
func JoinNaive(parts []string, sep string) string {
	var s string
	for i, p := range parts {
		if i > 0 {
			s += sep
		}
		s += p
	}
	return s
}

// JoinBuilder computes the final length first and grows a strings.Builder
// once, so it makes exactly one allocation however many parts there are.
func JoinBuilder(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	n := len(sep) * (len(parts) - 1)
	for _, p := range parts {
		n += len(p)
	}

	var b strings.Builder
	b.Grow(n)
	for i, p := range parts {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(p)
	}
	return b.String()
}
