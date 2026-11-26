// Command advanced-testing is the code under test for Part 024: a string
// joiner to benchmark, a key=value parser to fuzz, a Client consumer to mock,
// and a report renderer to compare against a golden file. main shows each
// briefly; the tests are where the techniques live.
//
// Slides: slides/024-advanced-testing.pdf
//
// Run:
//
//	go run ./examples/024-advanced-testing
//	go test -run='^$' -bench=. -benchmem ./examples/024-advanced-testing
//	go test -fuzz=FuzzParse -fuzztime=10s ./examples/024-advanced-testing
package main

import (
	"fmt"
	"strings"
)

// sampleConfig is the input main parses and reports on. golden_test.go uses
// the same text, so `go run` output and testdata/report.golden match.
const sampleConfig = `# service settings
name = checkout
port=8080
dsn = postgres://app@db/orders?sslmode=disable
log_level=info
`

func main() {
	section("1. JoinNaive vs JoinBuilder: same result, different cost (slides p02-p03)")
	joinDemo()

	section("2. Parse: the function FuzzParse attacks (slide p04)")
	parseDemo()

	section("3. Greeting depends on a Client interface (slide p06)")
	clientDemo()

	section("4. RenderReport: output checked by a golden file (slide p07)")
	reportDemo()
}

func joinDemo() {
	parts := []string{"go", "test", "-bench", "."}
	naive, builder := JoinNaive(parts, " "), JoinBuilder(parts, " ")
	fmt.Printf("JoinNaive:   %q\nJoinBuilder: %q\nequal: %v\n", naive, builder, naive == builder)
}

func parseDemo() {
	for _, in := range []string{"port=8080", " key = a=b ", "novalue", "=x", "bad key=1"} {
		kv, err := Parse(in)
		if err != nil {
			fmt.Printf("%-14q -> error: %v\n", in, err)
			continue
		}
		fmt.Printf("%-14q -> Key=%q Value=%q\n", in, kv.Key, kv.Value)
	}
}

func clientDemo() {
	fmt.Println(Greeting(StaticClient{Name: "  gopher "}))
	fmt.Println(Greeting(StaticClient{}))
}

func reportDemo() {
	settings, err := ParseLines(strings.NewReader(sampleConfig))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(RenderReport("Service configuration", settings))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
