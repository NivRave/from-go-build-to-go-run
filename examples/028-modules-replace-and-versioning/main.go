// Command modules-replace-and-versioning decodes the three forms of Go
// pseudo-version and points at the nested modules in workspace/, which show
// semantic import versioning and replace directives.
//
// Slides: slides/028-modules-replace-and-versioning.pdf
//
// Run:
//
//	go run ./examples/028-modules-replace-and-versioning
//	cd examples/028-modules-replace-and-versioning/workspace/app && go run .
package main

import (
	"fmt"
	"time"
)

func main() {
	section("1. Decoding pseudo-versions (slide p07)")
	decodePseudoVersions()

	section("2. Major versions side by side, with replace (slides p02-p03, p06)")
	workspaceHint()
}

// The three shapes the go command generates, depending on the latest tag
// reachable from the commit. Slide p07 shows only the first.
var samples = []struct{ version, note string }{
	{"v0.0.0-20260101120000-abcdefabcdef", "no tag yet"},
	{"v1.2.4-0.20260102093000-0123456789ab", "commit after tag v1.2.3"},
	{"v1.2.3-pre.0.20260103180000-fedcba987654", "commit after pre-release tag v1.2.3-pre"},
}

func decodePseudoVersions() {
	for _, s := range samples {
		p, err := ParsePseudo(s.version)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		base := p.Base
		if base == "" {
			base = "(none)"
		}
		fmt.Printf("%s  (%s)\n", p.Version, s.note)
		fmt.Printf("  base tag: %s\n", base)
		fmt.Printf("  time:     %s\n", p.Time.Format(time.RFC3339))
		fmt.Printf("  commit:   %s\n", p.Rev)
		fmt.Printf("  sorts:    %s\n", ordering(p))
	}
}

// ordering explains where a pseudo-version sorts. The "-0." and ".0." pieces
// exist so that the pseudo-version is a pre-release of the *next* version and
// therefore sorts after its base tag but before any real release.
func ordering(p Pseudo) string {
	if p.Base == "" {
		return "before every tagged release"
	}
	return "after " + p.Base + ", before the next release"
}

// workspaceHint prints how to run the nested modules. They live in their own
// go.mod files, so `go run ./examples/...` from the repository root never
// builds them.
func workspaceHint() {
	fmt.Println("workspace/ holds three nested modules: example.com/greeter, example.com/greeter/v2 and example.com/app.")
	fmt.Println("Run them from their own directory:")
	fmt.Println("  cd examples/028-modules-replace-and-versioning/workspace/app")
	fmt.Println("  go run .")
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
