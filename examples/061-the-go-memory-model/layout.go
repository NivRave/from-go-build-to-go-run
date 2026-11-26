package main

import (
	"fmt"
	"unsafe"
)

// The structs from the slides, unchanged. The comments give the 64-bit
// numbers (amd64, arm64); main prints what your GOARCH really does.

// BadStruct (slide p03): 1 + 7 padding + 8 + 4 + 4 padding = 24 bytes.
type BadStruct struct {
	A bool    // offset 0, then 7 bytes of padding so B is 8-aligned
	B float64 // offset 8
	C int32   // offset 16, then 4 bytes of tail padding
}

// GoodStruct (slide p03): 8 + 4 + 1 + 3 padding = 16 bytes.
type GoodStruct struct {
	B float64 // offset 0
	C int32   // offset 8
	A bool    // offset 12, then 3 bytes of tail padding
}

// LastField (slide p05): a zero-size final field gets padded, so that a
// pointer to it can't point past the end of the struct (into the next
// object in memory, which would keep that object alive). 8 + 8 = 16 bytes.
type LastField struct {
	A int64
	B struct{}
}

// FirstField (slide p05): the same fields, zero-size one first: 8 bytes.
type FirstField struct {
	B struct{}
	A int64
}

// field describes one struct field for printing.
type field struct {
	name                string
	offset, size, align uintptr
}

func printLayout(name string, size, align uintptr, fields []field) {
	fmt.Printf("%-10s size %2d  align %d\n", name, size, align)
	for _, f := range fields {
		fmt.Printf("  %-2s offset %2d  size %d  align %d\n", f.name, f.offset, f.size, f.align)
	}
}

// structLayouts prints size, alignment and field offsets for the slides'
// structs. unsafe.Sizeof, Alignof and Offsetof are compile-time constants:
// nothing is read through unsafe pointers here.
func structLayouts() {
	var b BadStruct
	printLayout("BadStruct", unsafe.Sizeof(b), unsafe.Alignof(b), []field{
		{"A", unsafe.Offsetof(b.A), unsafe.Sizeof(b.A), unsafe.Alignof(b.A)},
		{"B", unsafe.Offsetof(b.B), unsafe.Sizeof(b.B), unsafe.Alignof(b.B)},
		{"C", unsafe.Offsetof(b.C), unsafe.Sizeof(b.C), unsafe.Alignof(b.C)},
	})
	var g GoodStruct
	printLayout("GoodStruct", unsafe.Sizeof(g), unsafe.Alignof(g), []field{
		{"B", unsafe.Offsetof(g.B), unsafe.Sizeof(g.B), unsafe.Alignof(g.B)},
		{"C", unsafe.Offsetof(g.C), unsafe.Sizeof(g.C), unsafe.Alignof(g.C)},
		{"A", unsafe.Offsetof(g.A), unsafe.Sizeof(g.A), unsafe.Alignof(g.A)},
	})
	var l LastField
	printLayout("LastField", unsafe.Sizeof(l), unsafe.Alignof(l), []field{
		{"A", unsafe.Offsetof(l.A), unsafe.Sizeof(l.A), unsafe.Alignof(l.A)},
		{"B", unsafe.Offsetof(l.B), unsafe.Sizeof(l.B), unsafe.Alignof(l.B)},
	})
	var f FirstField
	printLayout("FirstField", unsafe.Sizeof(f), unsafe.Alignof(f), []field{
		{"B", unsafe.Offsetof(f.B), unsafe.Sizeof(f.B), unsafe.Alignof(f.B)},
		{"A", unsafe.Offsetof(f.A), unsafe.Sizeof(f.A), unsafe.Alignof(f.A)},
	})
}

// alignmentIsNotSize shows the rule slide p02 gets slightly wrong: a value
// is aligned to its alignment, not its size. All three are 8-aligned on
// 64-bit, though their sizes are 24, 16 and 24.
func alignmentIsNotSize() {
	var (
		arr [3]int64
		str string
		sl  []byte
	)
	fmt.Printf("[3]int64  size %2d  align %d\n", unsafe.Sizeof(arr), unsafe.Alignof(arr))
	fmt.Printf("string    size %2d  align %d\n", unsafe.Sizeof(str), unsafe.Alignof(str))
	fmt.Printf("[]byte    size %2d  align %d\n", unsafe.Sizeof(sl), unsafe.Alignof(sl))
}
