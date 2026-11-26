// Command advanced-string-tactics demonstrates the substring memory leak and
// its fix, what strings.EqualFold really compares, zero-copy unsafe views,
// validating external UTF-8, and building host:port strings correctly.
//
// Slides: slides/006-advanced-string-tactics.pdf
//
// Run:
//
//	go run ./examples/006-advanced-string-tactics
//	go test -run='^$' -bench=. -benchmem ./examples/006-advanced-string-tactics
package main

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"unicode/utf8"
	"unsafe"
)

func main() {
	section("1. The substring memory leak and strings.Clone (slides p02-p03)")
	substringLeak()

	section("2. What strings.EqualFold really compares (slide p04)")
	equalFold()

	section("3. Zero-copy views with unsafe, read-only (slide p05)")
	zeroCopyViews()

	section("4. Validating external data (slide p06)")
	validation()

	section("5. Building host:port (slide p07)")
	hostPort()
}

// ---- 1. Substring leak ----

// kept is package-level so that whatever it points at stays reachable, like a
// cache entry or a field of a long-lived struct.
var kept string

// getGiganticString stands in for a large payload read from disk or the
// network. 64 MB keeps the demo within the repository's memory budget.
func getGiganticString() string {
	return strings.Repeat("x", 64<<20)
}

// keepSubstring is slide p02. Once it returns, large is gone, but kept is a
// 10-byte string header that still points into the 64 MB array.
func keepSubstring() {
	large := getGiganticString()
	kept = large[:10] // Still holds a reference to all 64 MB!
}

// keepClone is slide p03: strings.Clone copies the 10 bytes into a new, tiny
// allocation, so nothing references the 64 MB array after the return.
func keepClone() {
	large := getGiganticString()
	kept = strings.Clone(large[:10]) // Now it has its own tiny array
}

func substringLeak() {
	report := func(label string) {
		fmt.Printf("%-34s HeapInuse = %3d MB, len(kept) = %d\n", label, heapInuseMB(), len(kept))
	}
	report("baseline:")

	keepSubstring()
	report("kept = large[:10]:")

	keepClone()
	report("kept = strings.Clone(large[:10]):")

	kept = "" // release the demo's memory
}

// heapInuseMB forces a full collection and reports the heap in use, in MB.
func heapInuseMB() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse >> 20
}

// ---- 2. EqualFold ----

func equalFold() {
	const kelvin = "\xe2\x84\xaa" // U+212A KELVIN SIGN, which looks like K
	pairs := [][2]string{
		{"Go", "GO"},
		{"ß", "SS"},
		{kelvin, "k"},
	}
	for _, p := range pairs {
		a, b := p[0], p[1]
		// %+q prints non-ASCII as escapes, so the Kelvin sign is visible.
		label := fmt.Sprintf("%+q vs %+q", a, b)
		lowered := strings.ToLower(a) == strings.ToLower(b) //nolint:staticcheck // intentional: the slide p04 idiom, compared with EqualFold
		fmt.Printf("%-22s EqualFold=%-5t ToLower==ToLower=%t\n", label, strings.EqualFold(a, b), lowered)
	}
	// "ß" upper-cases to the two letters "SS", which simple (one rune to one
	// rune) case folding can't express. Full folding: golang.org/x/text/cases.
}

// ---- 3. Zero-copy views ----

func zeroCopyViews() {
	s := "Senior Go"

	// 1. Standard way: allocates and copies, so b can be modified safely.
	b := []byte(s)
	b[0] = 's'

	// 2. Zero allocations: a slice header pointed at the string's own bytes.
	// It must be treated as read-only. s is a literal, so its bytes live in
	// read-only memory, and unsafeSlice[0] = 'X' would crash the process
	// with "unexpected fault address" / "fatal error: fault". For a string
	// built at run time, the write would silently change an "immutable"
	// string everywhere it is shared.
	unsafeSlice := unsafe.Slice(unsafe.StringData(s), len(s))

	fmt.Printf("copy  []byte(s):      %s (shares memory with s: %t)\n", b, &b[0] == unsafe.StringData(s))
	fmt.Printf("view  unsafe.Slice:   %s (shares memory with s: %t)\n", unsafeSlice, &unsafeSlice[0] == unsafe.StringData(s))
	fmt.Println("Println of the view: ", unsafeSlice[:6]) //nolint:staticcheck // intentional: shows that Println prints byte values, as on slide p05

	// The inverse: view a []byte as a string without copying. Only safe if
	// the bytes are never modified afterwards.
	buf := []byte("built at run time")
	view := unsafe.String(&buf[0], len(buf))
	fmt.Printf("view  unsafe.String:  %s (shares memory with buf: %t)\n", view, unsafe.StringData(view) == &buf[0])
}

// ---- 4. Validation ----

// validatePayload is slide p06's check as a complete function.
func validatePayload(payload string) error {
	if !utf8.ValidString(payload) {
		return errors.New("malformed UTF-8")
	}
	return nil
}

func validation() {
	payload := "ok\xff\xfeok"
	fmt.Printf("validatePayload(%q) = %v\n", "ok", validatePayload("ok"))
	fmt.Printf("validatePayload(%q) = %v\n", payload, validatePayload(payload))

	// Sanitise instead of rejecting: a run of invalid bytes becomes one
	// replacement string.
	replacement := string(utf8.RuneError) // U+FFFD
	fmt.Printf("strings.ToValidUTF8 -> %+q\n", strings.ToValidUTF8(payload, replacement))

	// No panic either way: range decodes each invalid byte as U+FFFD.
	invalid := "\xff\xfe"
	fmt.Printf("range over %q:", invalid)
	for i, r := range invalid {
		fmt.Printf(" %d:%U", i, r)
	}
	fmt.Println()
}

// ---- 5. host:port ----

func hostPort() {
	for _, host := range []string{"example.com", "::1"} {
		naive := fmt.Sprintf("%s:%d", host, 8080)
		fmt.Printf("%-12s Sprintf: %-17s JoinHostPort: %s\n", host, naive, net.JoinHostPort(host, "8080"))
	}
	// An IPv6 address needs brackets, or the port is ambiguous. Join is for
	// joining a slice of strings, as on the slide:
	part1, part2 := "go", "build"
	fmt.Println(`strings.Join([]string{part1, part2}, "-") =`, strings.Join([]string{part1, part2}, "-"))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
