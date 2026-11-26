//go:build amd64 || arm64 || loong64 || mips64 || mips64le || ppc64 || ppc64le || riscv64 || s390x

package main

// The sizes below hold on every 64-bit architecture Go supports. On 32-bit
// platforms such as 386, int64 is only 4-byte aligned and both structs are
// smaller, so this Example is not built there.

func Example_layout() {
	layout()
	// Output:
	// Padded    size 24, align 8, offsets A=0 B=8 C=16
	// Optimized size 16, align 8, offsets B=0 A=8 C=9
}
