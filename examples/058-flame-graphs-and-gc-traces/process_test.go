package main

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
)

// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/058-flame-graphs-and-gc-traces
//
// The point of these benchmarks is to measure the claims on slides p05 and
// p07 instead of trusting them.

func testItems(n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = "item-" + strconv.Itoa(i)
	}
	return items
}

var benchItems = testItems(1000)

// Benchmark results are stored in package-level sinks so the compiler can't
// prove them unused and skip (or stack-allocate) the work being measured.
var (
	sinkString string
	sinkInts   []int
)

// Slide p07 with a non-retaining save: escape analysis keeps the "slow"
// version's slice literal on the stack, so both report 0 allocs/op.

func BenchmarkProcessSlow(b *testing.B) {
	b.ReportAllocs()
	var t tally
	for b.Loop() {
		processSlow(benchItems, &t)
	}
}

func BenchmarkProcessFast(b *testing.B) {
	b.ReportAllocs()
	var t tally
	for b.Loop() {
		processFast(benchItems, &t)
	}
}

// With a retaining save the slice escapes: one allocation per item (plus the
// archive's own growth, reset every iteration). This is the cost of being
// correct; processFastRetained is cheaper only because it is wrong.
func BenchmarkProcessSlowRetained(b *testing.B) {
	b.ReportAllocs()
	a := archive{entries: make([][]string, 0, len(benchItems))}
	for b.Loop() {
		a.entries = a.entries[:0]
		processSlowRetained(benchItems, &a)
	}
}

// TestProcessFastRetainedAliasing shows the bug in the "fast" version when
// save keeps its argument: every stored entry shares one backing array.
func TestProcessFastRetainedAliasing(t *testing.T) {
	t.Parallel()
	items := []string{"a", "b", "c"}

	var slow archive
	processSlowRetained(items, &slow)
	for i, e := range slow.entries {
		if e[0] != items[i] {
			t.Errorf("slow: entry %d = %q, want %q", i, e[0], items[i])
		}
	}

	var fast archive
	processFastRetained(items, &fast)
	for i, e := range fast.entries {
		if e[0] != "c" {
			t.Errorf("fast: entry %d = %q; expected the aliasing bug to make it %q", i, e[0], "c")
		}
	}
	if &fast.entries[0][0] != &fast.entries[2][0] {
		t.Error("fast: entries should share one backing array")
	}
}

func TestProcessNonRetainingAgree(t *testing.T) {
	t.Parallel()
	var slow, fast tally
	processSlow(benchItems, &slow)
	processFast(benchItems, &fast)
	if slow.bytes != fast.bytes {
		t.Errorf("slow saw %d bytes, fast saw %d", slow.bytes, fast.bytes)
	}
}

// sync.Pool vs a fresh bytes.Buffer per call (slide p05).

func BenchmarkBufferFresh(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkString = renderFresh(42)
	}
}

func BenchmarkBufferPooled(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkString = renderPooled(42)
	}
}

func TestRenderAgree(t *testing.T) {
	t.Parallel()
	if fresh, pooled := renderFresh(7), renderPooled(7); fresh != pooled {
		t.Errorf("renderFresh = %q, renderPooled = %q", fresh, pooled)
	}
}

// make([]T, 0, cap) vs append growth (slide p07).

const collectN = 10_000

func BenchmarkAppendGrow(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInts = collectAppend(collectN)
	}
}

func BenchmarkAppendPrealloc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInts = collectPrealloc(collectN)
	}
}

func TestCollectAgree(t *testing.T) {
	t.Parallel()
	if !slices.Equal(collectAppend(100), collectPrealloc(100)) {
		t.Error("collectAppend and collectPrealloc differ")
	}
}

// fmt.Sprintf vs strconv concatenation vs strings.Builder (slide p07).

func BenchmarkLabelSprintf(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkString = labelSprintf("worker", 123456)
	}
}

func BenchmarkLabelConcat(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkString = labelConcat("worker", 123456)
	}
}

func BenchmarkLabelBuilder(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkString = labelBuilder("worker", 123456)
	}
}

func TestLabelsAgree(t *testing.T) {
	t.Parallel()
	want := fmt.Sprint("worker-", 123456)
	for name, got := range map[string]string{
		"Sprintf": labelSprintf("worker", 123456),
		"Concat":  labelConcat("worker", 123456),
		"Builder": labelBuilder("worker", 123456),
	} {
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// The two loops from main.go must produce identical output.
func TestLoopsAgree(t *testing.T) {
	t.Parallel()
	if h, l := heavyLoop(1000), lightLoop(1000); h != l {
		t.Errorf("heavyLoop = %x, lightLoop = %x", h, l)
	}
}
