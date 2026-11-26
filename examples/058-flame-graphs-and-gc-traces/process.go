package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Slide p07 shows two functions both named process: a "slow" one that builds
// a new two-element slice per item and a "fast" one that reuses one buffer.
// They are renamed here so both compile in one package.
//
// Whether "slow" is actually slow depends entirely on what save does with its
// argument, so each version comes with two kinds of save:
//
//	tally.save   reads the slice and forgets it   (non-retaining)
//	archive.save keeps the slice for later         (retaining)
//
// Check what escape analysis decides with:
//
//	go build -gcflags=-m -o bin/ ./examples/058-flame-graphs-and-gc-traces

// tally is a non-retaining sink: save only reads the slice.
type tally struct{ bytes int }

func (t *tally) save(update []string) {
	for _, s := range update {
		t.bytes += len(s)
	}
}

// archive is a retaining sink: save stores the slice header itself, so the
// backing array must stay valid after save returns.
type archive struct{ entries [][]string }

func (a *archive) save(update []string) {
	a.entries = append(a.entries, update)
}

// processSlow is the slide's "slow" version. With a non-retaining save the
// slice literal does not escape, so the compiler puts it on the stack and the
// loop allocates nothing.
func processSlow(items []string, t *tally) {
	for _, item := range items {
		update := []string{item, "processed"}
		t.save(update)
	}
}

// processFast is the slide's "fast" version. With a non-retaining save it
// costs the same as processSlow: zero heap allocations.
func processFast(items []string, t *tally) {
	buf := make([]string, 2)
	for _, item := range items {
		buf[0] = item
		buf[1] = "processed"
		t.save(buf)
	}
}

// processSlowRetained is the "slow" version with a retaining save. Now the
// slice escapes and every item costs one heap allocation, which is correct:
// each stored entry needs its own backing array.
func processSlowRetained(items []string, a *archive) {
	for _, item := range items {
		update := []string{item, "processed"}
		a.save(update)
	}
}

// processFastRetained is the "fast" version with a retaining save. It
// allocates once, but every stored entry shares the same backing array, so
// after the loop they all read the last item. Faster, and wrong.
func processFastRetained(items []string, a *archive) {
	buf := make([]string, 2)
	for _, item := range items {
		buf[0] = item
		buf[1] = "processed"
		a.save(buf)
	}
}

// The helpers below are the other allocation fixes from slides p05 and p07,
// each paired with its naive version for the benchmarks.

// bufPool holds *bytes.Buffer, not bytes.Buffer: putting a non-pointer value
// into a sync.Pool boxes it in an interface, which allocates on every Put
// (staticcheck SA6002).
var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// renderFresh builds a line in a brand-new buffer every call.
func renderFresh(id int) string {
	var buf bytes.Buffer
	writeLine(&buf, id)
	return buf.String()
}

// renderPooled reuses buffers from bufPool. Only the returned string is
// allocated; the buffer's backing array is recycled.
func renderPooled(id int) string {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writeLine(buf, id)
	s := buf.String()
	bufPool.Put(buf)
	return s
}

func writeLine(buf *bytes.Buffer, id int) {
	for i := range 8 {
		buf.WriteString("field")
		buf.WriteString(strconv.Itoa(i))
		buf.WriteByte('=')
		buf.WriteString(strconv.Itoa(id))
		buf.WriteByte(' ')
	}
}

// collectAppend grows a slice from empty, reallocating as it goes.
func collectAppend(n int) []int {
	var out []int
	for i := range n {
		out = append(out, i)
	}
	return out
}

// collectPrealloc sizes the slice once with make([]T, 0, cap).
func collectPrealloc(n int) []int {
	out := make([]int, 0, n)
	for i := range n {
		out = append(out, i)
	}
	return out
}

// labelSprintf, labelConcat and labelBuilder produce the same string.
// fmt.Sprintf boxes its arguments and parses the format at run time.
func labelSprintf(name string, id int) string {
	return fmt.Sprintf("%s-%d", name, id)
}

func labelConcat(name string, id int) string {
	return name + "-" + strconv.Itoa(id)
}

// labelBuilder formats the digits into a stack array with strconv.AppendInt
// instead of strconv.Itoa, so the only heap allocation is the Builder's
// single Grow. With b.WriteString(strconv.Itoa(id)) it would be two.
func labelBuilder(name string, id int) string {
	var digits [20]byte
	num := strconv.AppendInt(digits[:0], int64(id), 10)
	var b strings.Builder
	b.Grow(len(name) + 1 + len(num))
	b.WriteString(name)
	b.WriteByte('-')
	b.Write(num)
	return b.String()
}
