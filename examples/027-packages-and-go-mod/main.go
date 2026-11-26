// Command packages-and-go-mod demonstrates module structure: a public package
// next to an internal one, the build information go.mod produces at runtime,
// and an offline simulator of Minimal Version Selection.
//
// Slides: slides/027-packages-and-go-mod.pdf
//
// Run:
//
//	go run ./examples/027-packages-and-go-mod
//	go test ./examples/027-packages-and-go-mod/...
package main

import (
	"fmt"
	"maps"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/NivRave/from-go-build-to-go-run/examples/027-packages-and-go-mod/greet"
	"github.com/NivRave/from-go-build-to-go-run/examples/027-packages-and-go-mod/internal/auth"
)

func main() {
	section("1. Public vs. internal packages (slide p03)")
	packages()

	section("2. What go.mod produces at runtime (slide p02)")
	buildInfo()

	section("3. Minimal Version Selection (slide p04)")
	mvsSlideExample()
	mvsUpgrade()
}

// packages uses both kinds of package. main may import internal/auth because
// main lives in the directory that contains internal/. Code outside
// examples/027-packages-and-go-mod cannot (see the README for the real
// compiler error).
func packages() {
	for _, user := range []string{"gopher", "guest"} {
		fmt.Printf("%-16s admin=%v\n", greet.Hello(user), auth.Allowed(user))
	}
}

// buildInfo prints what the go command embedded in this binary from go.mod.
// The output depends on how the program was built (go run, go build, go test)
// and on the toolchain, so it has no Example test.
func buildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("no build info (binary built without module support)")
		return
	}
	fmt.Println("main module:", info.Main.Path)
	fmt.Println("version:    ", info.Main.Version)
	fmt.Println("package:    ", info.Path)
	fmt.Println("built with: ", info.GoVersion)
	if len(info.Deps) == 0 {
		fmt.Println("deps:        none (this example imports only the standard library)")
	}
	for _, d := range info.Deps {
		fmt.Printf("dep:         %s %s\n", d.Path, d.Version)
	}
	for _, s := range info.Settings {
		if s.Key == "GOOS" || s.Key == "GOARCH" || strings.HasPrefix(s.Key, "vcs") {
			fmt.Printf("setting:     %s=%s\n", s.Key, s.Value)
		}
	}
}

// slideGraph is slide p04's scenario: serviceA and serviceB need different
// minimum versions of lib, and a newer lib v1.9.0 has been published. The two
// lib versions in the graph also disagree about log.
var slideGraph = Graph{
	"myapp":           {"serviceA@v1.0.0", "serviceB@v1.0.0"},
	"serviceA@v1.0.0": {"lib@v1.2.0"},
	"serviceB@v1.0.0": {"lib@v1.5.0"},
	"lib@v1.2.0":      {"log@v1.1.0"},
	"lib@v1.5.0":      {"log@v1.0.0"},
	"lib@v1.9.0":      {"log@v1.3.0"}, // published, but nobody requires it
}

func mvsSlideExample() {
	fmt.Println("serviceA needs lib v1.2.0, serviceB needs lib v1.5.0, lib v1.9.0 exists:")
	fmt.Println("  build list:", strings.Join(BuildList(slideGraph, "myapp"), " "))
	fmt.Println("  lib: the highest minimum (v1.5.0), not the newest release (v1.9.0)")
	fmt.Println("  log: v1.1.0, because lib v1.2.0 is still in the graph and asks for it")
}

func mvsUpgrade() {
	// go get lib@v1.5.2 adds (or raises) the main module's own requirement.
	g := maps.Clone(slideGraph)
	g["myapp"] = append(slices.Clone(g["myapp"]), "lib@v1.5.2")
	fmt.Println("after go get lib@v1.5.2 (myapp now requires it directly):")
	fmt.Println("  build list:", strings.Join(BuildList(g, "myapp"), " "))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
