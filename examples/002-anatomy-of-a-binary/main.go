// Command anatomy-of-a-binary makes Go's startup order visible: the runtime
// starts, package-level variables are initialised in dependency order, init()
// runs, and only then is main.main called.
//
// Slides: slides/002-anatomy-of-a-binary.pdf
//
// Run:
//
//	go run ./examples/002-anatomy-of-a-binary
//	go build -o hello ./examples/002-anatomy-of-a-binary
//	go build -ldflags="-s -w" -o hello-small ./examples/002-anatomy-of-a-binary
//	go version -m hello
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// trace records each startup step as it happens. It has no initialiser, so
// it is simply zero (nil) before any other package-level code runs.
var trace []string

// greeting is declared first but initialised second: its initialiser calls
// makeGreeting, which reads name, so the compiler orders name before it.
// Package initialisation follows dependencies, not source order.
var greeting = makeGreeting()

var name = loadName()

func loadName() string {
	trace = append(trace, "var name     = loadName()     (declared second, initialised first)")
	return "Gopher"
}

func makeGreeting() string {
	trace = append(trace, "var greeting = makeGreeting() (depends on name)")
	return "Hello, " + name + "!"
}

// init runs after every package-level variable of this package is set, and
// after the init functions of every imported package (fmt, runtime, ...).
func init() {
	trace = append(trace, "init()                        (greeting = "+greeting+")")
}

func main() {
	section("1. Startup order (slides p04-p06)")
	printTrace()
	fmt.Printf("%d. main.main\n", len(trace)+1)

	section("2. The binary knows where it came from (slide p07)")
	moduleInfo()
}

// printTrace prints the steps that ran before main.main. By the time any of
// them ran, the runtime had already set up the heap, the scheduler and the GC.
func printTrace() {
	for i, step := range trace {
		fmt.Printf("%d. %s\n", i+1, step)
	}
}

// moduleInfo ties the running binary back to go.mod.
func moduleInfo() {
	fmt.Println("runtime.Version():", runtime.Version())
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Println("module path:      ", info.Main.Path)
		fmt.Println("built with:       ", info.GoVersion)
	}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
