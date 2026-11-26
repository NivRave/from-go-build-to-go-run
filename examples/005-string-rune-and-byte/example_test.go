package main

// Section 5 (header sizes) is not asserted: the sizes halve on 32-bit
// platforms.

func Example_lengthTrap() {
	lengthTrap()
	// Output:
	// "Go"     len=2  utf8.RuneCountInString=2
	// "coöts"  len=6  utf8.RuneCountInString=5
	// "שלום"   len=8  utf8.RuneCountInString=4
	// "Go 🌏"   len=7  utf8.RuneCountInString=4
}

func Example_runesAreNotCharacters() {
	runesAreNotCharacters()
	// Output:
	// "👍🏽": len=8 runes=2
	//   U+1F44D
	//   U+1F3FD
}

func Example_byteLoop() {
	byteLoop()
	// Output:
	// for i := 0; i < len(s); i++ (bytes):
	//   s[0] = 'G'      0x47
	//   s[1] = 'o'      0x6f
	//   s[2] = ' '      0x20
	//   s[3] = 'ð'      0xf0
	//   s[4] = '\u009f' 0x9f
	//   s[5] = '\u008c' 0x8c
	//   s[6] = '\u008f' 0x8f
}

func Example_rangeLoop() {
	rangeLoop()
	// Output:
	// for index, runeVal := range s (runes):
	//   G starts at byte 0
	//   o starts at byte 1
	//     starts at byte 2
	//   🌏 starts at byte 3
}

func Example_invalidUTF8() {
	invalidUTF8()
	// Output:
	// range: offset 0 -> U+0061
	// range: offset 1 -> U+FFFD
	// range: offset 2 -> U+0062
	// DecodeRuneInString("\xff") = U+FFFD, width 1, RuneError? true
}
