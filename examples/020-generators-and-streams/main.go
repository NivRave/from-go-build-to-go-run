// Command generators-and-streams demonstrates lazy generators and streaming
// pipelines: the closure-based pattern from the slides, the Go 1.23 iter.Seq
// equivalent, and a line stream that reports errors correctly.
//
// Slides: slides/020-generators-and-streams.pdf
//
// Run:
//
//	go run ./examples/020-generators-and-streams
//	go test -bench=. -benchmem ./examples/020-generators-and-streams
package main

import (
	"fmt"
	"strings"
)

func main() {
	section("1. Closure pipeline (slides p03-p06)")
	closurePipeline()

	section("2. The same pipeline with iter.Seq (Go 1.23+)")
	iterPipeline()

	section("3. Laziness: break stops the producer")
	earlyExit()

	section("4. Streams must report errors (slides p05, p07)")
	streamErrors()
}

func closurePipeline() {
	// Source -> Filter -> Map, exactly as on slide p06.
	pipeline := Multiply(FilterEven(IntGenerator(1, 10)), 10)
	for {
		v, ok := pipeline()
		if !ok {
			break
		}
		fmt.Print(v, " ")
	}
	fmt.Println()
}

func iterPipeline() {
	isEven := func(v int) bool { return v%2 == 0 }
	times10 := func(v int) int { return v * 10 }
	for v := range Map(Filter(Ints(1, 10), isEven), times10) {
		fmt.Print(v, " ")
	}
	fmt.Println()
}

func earlyExit() {
	produced := 0
	counting := func(v int) int { produced++; return v }
	// The source could run to 1e9; we only pay for what we consume.
	for v := range Map(Ints(1, 1_000_000_000), counting) {
		if v == 3 {
			break
		}
	}
	fmt.Printf("consumed up to 3, producer generated %d values\n", produced)
}

func streamErrors() {
	longLine := strings.Repeat("x", 100*1024) // 100 KiB: over the 64 KiB default
	input := "first\n" + longLine + "\nlast\n"

	n, err := CountLines(NewLineStream(strings.NewReader(input), 0))
	fmt.Printf("default buffer: %d lines, err = %v\n", n, err)

	n, err = CountLines(NewLineStream(strings.NewReader(input), 1<<20))
	fmt.Printf("1 MiB buffer:   %d lines, err = %v\n", n, err)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
