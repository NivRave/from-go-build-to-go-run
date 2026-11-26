// Command app uses v1 and v2 of example.com/greeter in the same build
// (semantic import versioning) and shows the replace directives from its
// go.mod in the binary's build information.
//
// This is a nested module, so it is not part of the repository's root module.
// It belongs to examples/028-modules-replace-and-versioning.
//
// Run:
//
//	cd examples/028-modules-replace-and-versioning/workspace/app
//	go run .
package main

import (
	"fmt"
	"runtime/debug"

	greeterv1 "example.com/greeter"
	greeterv2 "example.com/greeter/v2"
)

func main() {
	fmt.Println("== Two major versions in one build (slide p02) ==")
	fmt.Println("v1:", greeterv1.Hello("Gopher"))
	for _, lang := range []string{"es", "fr"} {
		msg, err := greeterv2.Hello("Gopher", lang)
		if err != nil {
			fmt.Println("v2:", "error:", err)
			continue
		}
		fmt.Println("v2:", msg)
	}

	fmt.Println()
	fmt.Println("== Dependencies and their replacements (slide p03) ==")
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("no build info")
		return
	}
	fmt.Println("main module:", info.Main.Path)
	for _, d := range info.Deps {
		line := fmt.Sprintf("%s %s", d.Path, d.Version)
		// A directory replacement has no version, so Replace.Version is
		// "(devel)". Under go.work, workspace modules have no Replace at all.
		if d.Replace != nil {
			line += " => " + d.Replace.Path
			if d.Replace.Version != "" {
				line += " " + d.Replace.Version
			}
		}
		fmt.Println(line)
	}
}
