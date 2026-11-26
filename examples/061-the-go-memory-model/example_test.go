//go:build amd64 || arm64

// The layout numbers are for 64-bit targets. This file is limited to the two
// 64-bit architectures CI runs on, so `GOARCH=386 go test` still passes.

package main

func Example_structLayouts() {
	structLayouts()
	// Output:
	// BadStruct  size 24  align 8
	//   A  offset  0  size 1  align 1
	//   B  offset  8  size 8  align 8
	//   C  offset 16  size 4  align 4
	// GoodStruct size 16  align 8
	//   B  offset  0  size 8  align 8
	//   C  offset  8  size 4  align 4
	//   A  offset 12  size 1  align 1
	// LastField  size 16  align 8
	//   A  offset  0  size 8  align 8
	//   B  offset  8  size 0  align 1
	// FirstField size  8  align 8
	//   B  offset  0  size 0  align 1
	//   A  offset  0  size 8  align 8
}

func Example_alignmentIsNotSize() {
	alignmentIsNotSize()
	// Output:
	// [3]int64  size 24  align 8
	// string    size 16  align 8
	// []byte    size 24  align 8
}
