// Command the-go-memory-model demonstrates struct layout (size, alignment,
// padding and field order) and the Go memory model (happens-before and
// correct publication of data between goroutines).
//
// Slides: slides/061-the-go-memory-model.pdf
//
// Run:
//
//	go run ./examples/061-the-go-memory-model
//	GOARCH=386 go run ./examples/061-the-go-memory-model
//	go test -run='^$' -bench=. -benchmem ./examples/061-the-go-memory-model
//	go run -race ./examples/061-the-go-memory-model -race-demo
package main

import (
	"flag"
	"fmt"
	"runtime"
	"unsafe"
)

func main() {
	raceDemo := flag.Bool("race-demo", false, "run the unsynchronised publish (meant for go run -race)")
	flag.Parse()

	if *raceDemo {
		section("Data race: publishing without synchronisation")
		publishRacy()
		return
	}

	section("1. Platform")
	platform()

	section("2. Struct padding and field order (slides p03, p05, p07)")
	structLayouts()

	section("3. Alignment is not size (slides p02, p04)")
	alignmentIsNotSize()

	section("4. The memory model: publishing data safely (not on the slides)")
	memoryModel()
}

// platform prints what the layout numbers depend on. On 32-bit targets
// (386, arm) int64 and float64 align to 4, not 8, so BadStruct shrinks to 16
// bytes: try GOARCH=386 go run ./examples/061-the-go-memory-model.
func platform() {
	fmt.Printf("GOARCH=%s, pointer size %d bytes\n", runtime.GOARCH, unsafe.Sizeof(uintptr(0)))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
