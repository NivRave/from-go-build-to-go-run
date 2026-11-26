package main

import (
	"strings"
	"testing"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/024-advanced-testing
//
// -run='^$' matches no test, so only benchmarks run. That matters even more
// when profiling (-cpuprofile), where tests would pollute the profile.

// makeParts is the "expensive setup" both styles must keep out of the
// measurement.
func makeParts(n int) []string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = strings.Repeat("x", 8)
	}
	return parts
}

const numParts = 200

// sink keeps old-style benchmark results alive. Without it the compiler may
// inline the call, see that the result is unused and delete the work.
var sink string

// ---- Old style: b.N loop (before Go 1.24) ------------------------------------

// The b.N loop needs b.ResetTimer after setup. The whole function, setup
// included, also runs once per b.N ramp-up step (1, 100, 10000, ...), and
// the result must be stored somewhere to stop the compiler from optimising
// the call away.

func BenchmarkJoinNaiveOld(b *testing.B) {
	parts := makeParts(numParts)
	b.ReportAllocs()
	// Don't measure makeParts.
	b.ResetTimer()
	for i := 0; i < b.N; i++ { //nolint:intrange // intentional: the pre-Go 1.24 b.N loop being contrasted with b.Loop
		sink = JoinNaive(parts, ",")
	}
}

func BenchmarkJoinBuilderOld(b *testing.B) {
	parts := makeParts(numParts)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ { //nolint:intrange // intentional: the pre-Go 1.24 b.N loop being contrasted with b.Loop
		sink = JoinBuilder(parts, ",")
	}
}

// ---- New style: b.Loop (Go 1.24+) ----------------------------------------------

// b.Loop is better on every count: the timer starts at the first call to
// b.Loop, so setup before the loop is excluded without ResetTimer; the
// benchmark function itself runs only once, so setup runs once; and calls
// inside the loop are kept alive, so no sink variable is needed.

func BenchmarkJoinNaive(b *testing.B) {
	parts := makeParts(numParts)
	b.ReportAllocs()
	for b.Loop() {
		JoinNaive(parts, ",")
	}
}

func BenchmarkJoinBuilder(b *testing.B) {
	parts := makeParts(numParts)
	b.ReportAllocs()
	for b.Loop() {
		JoinBuilder(parts, ",")
	}
}

// Both joiners must agree, or the benchmarks compare different work.
func TestJoinersAgree(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 1, 2, numParts} {
		parts := makeParts(n)
		if got, want := JoinBuilder(parts, ", "), JoinNaive(parts, ", "); got != want {
			t.Errorf("n=%d: JoinBuilder = %q, JoinNaive = %q", n, got, want)
		}
	}
}
