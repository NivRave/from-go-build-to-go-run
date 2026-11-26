// Command string-rune-and-byte demonstrates what a Go string really is: a
// read-only sequence of UTF-8 bytes, where len counts bytes, range decodes
// runes, and a rune is a code point rather than a visible character.
//
// Slides: slides/005-string-rune-and-byte.pdf
//
// Run:
//
//	go run ./examples/005-string-rune-and-byte
//	go test -run='^$' -bench=. -benchmem ./examples/005-string-rune-and-byte
package main

import (
	"fmt"
	"runtime"
	"unicode/utf8"
	"unsafe"
)

func main() {
	section("1. The length trap: bytes vs runes (slide p04)")
	lengthTrap()

	section("2. Runes are not characters (slide p06)")
	runesAreNotCharacters()

	section("3. Byte loop vs range loop (slides p05-p06)")
	byteLoop()
	rangeLoop()

	section("4. Invalid UTF-8 decodes as U+FFFD (slide p06)")
	invalidUTF8()

	section("5. Header sizes (slides p02-p03)")
	headerSizes()
}

func lengthTrap() {
	// "coöts" uses the precomposed ö (U+00F6, 2 bytes in UTF-8), which
	// reproduces the slide's len 6 / 5 runes. Hebrew letters are 2 bytes
	// each, and the globe emoji is 4.
	for _, s := range []string{"Go", "coöts", "שלום", "Go 🌏"} {
		fmt.Printf("%-8q len=%d  utf8.RuneCountInString=%d\n", s, len(s), utf8.RuneCountInString(s))
	}
}

func runesAreNotCharacters() {
	// One thumbs-up with a skin tone looks like one character, but it is two
	// code points: the emoji plus a modifier. Counting user-perceived
	// characters (grapheme clusters) needs a segmentation library.
	s := "👍🏽"
	fmt.Printf("%q: len=%d runes=%d\n", s, len(s), utf8.RuneCountInString(s))
	for _, r := range s {
		fmt.Printf("  %U\n", r)
	}
}

// byteLoop indexes the string, as on slide p05. s[i] is a byte, so the four
// bytes of the emoji come out as four unrelated values; %q shows how they
// look when each is (wrongly) treated as a character on its own.
func byteLoop() {
	s := "Go 🌏"
	fmt.Println("for i := 0; i < len(s); i++ (bytes):")
	for i := 0; i < len(s); i++ { //nolint:intrange // intentional: the classic index loop from slide p05
		fmt.Printf("  s[%d] = %-8q 0x%x\n", i, s[i], s[i])
	}
}

// rangeLoop is slide p06: range decodes UTF-8 and yields each rune with the
// byte offset where it starts.
func rangeLoop() {
	s := "Go 🌏"
	fmt.Println("for index, runeVal := range s (runes):")
	for index, runeVal := range s {
		fmt.Printf("  %c starts at byte %d\n", runeVal, index)
	}
}

func invalidUTF8() {
	s := "a\xffb" // 0xff can never appear in valid UTF-8
	for i, r := range s {
		fmt.Printf("range: offset %d -> %U\n", i, r)
	}
	// utf8.DecodeRuneInString is the other safe way to walk a string, and it
	// also reports the width: invalid bytes decode as RuneError, width 1.
	bad := "\xff"
	r, width := utf8.DecodeRuneInString(bad)
	fmt.Printf("DecodeRuneInString(%q) = %U, width %d, RuneError? %t\n", bad, r, width, r == utf8.RuneError)
}

func headerSizes() {
	// A string header is pointer + length; a slice header adds a capacity.
	// The values below are for 64-bit platforms (halve them on 32-bit).
	//
	// The types are lowercase rune and byte. The slide's capitalised version
	// doesn't compile: var r Rune = 'a' // compile error: undefined: Rune
	var r rune = 'a' //nolint:staticcheck // intentional: spells out the type, as slide p03 does
	var b byte = 'b'
	fmt.Println("GOARCH:", runtime.GOARCH)
	fmt.Println("unsafe.Sizeof(\"\")          =", unsafe.Sizeof(""))
	fmt.Println("unsafe.Sizeof([]byte(nil)) =", unsafe.Sizeof([]byte(nil)))
	fmt.Println("unsafe.Sizeof(rune)        =", unsafe.Sizeof(r))
	fmt.Println("unsafe.Sizeof(byte)        =", unsafe.Sizeof(b))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
