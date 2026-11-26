// Command why-go demonstrates three of the slide's reasons in code: a single
// binary that knows what it was built for and from, and concurrency that is
// small and explicit.
//
// Slides: slides/001-why-go.pdf
//
// Run:
//
//	go run ./examples/001-why-go
//	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o app-linux-arm64 ./examples/001-why-go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
)

func main() {
	section("1. One self-describing binary (Deployment Heaven, slide p06)")
	platform()

	section("2. Build info embedded in the binary (slide p06)")
	buildInfo()

	section("3. Honest concurrency in miniature (slide p05)")
	concurrentSum()
}

// platform prints what this binary was compiled for and what it sees at run
// time. GOOS and GOARCH are fixed at build time: a binary runs only on the
// platform it was built for, so "run it anywhere" means cross-compiling.
func platform() {
	fmt.Printf("GOOS/GOARCH: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Go version:  %s\n", runtime.Version())
	fmt.Printf("NumCPU:      %d\n", runtime.NumCPU())
	// Since Go 1.25 the default GOMAXPROCS respects a container's CPU limit
	// (cgroup quota on Linux), so it can be lower than NumCPU.
	fmt.Printf("GOMAXPROCS:  %d\n", runtime.GOMAXPROCS(0))
}

// buildInfo reads the module and build settings the linker embedded in the
// binary. `go version -m <binary>` prints the same data from outside.
func buildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("no build info (binary built without module support)")
		return
	}
	field("main module", info.Main.Path)
	field("built with", info.GoVersion)

	var vcs []debug.BuildSetting
	for _, s := range info.Settings {
		switch s.Key {
		case "CGO_ENABLED", "GOOS", "GOARCH":
			field("setting", s.Key+"="+s.Value)
		case "vcs", "vcs.revision", "vcs.time", "vcs.modified":
			vcs = append(vcs, s)
		}
	}
	// `go build` stamps VCS info when it runs inside a repository. `go run`
	// and `go test` don't, so its absence is normal rather than an error.
	if len(vcs) == 0 {
		field("vcs", "not stamped (expected under go run; use go build)")
		return
	}
	for _, s := range vcs {
		field(s.Key, s.Value)
	}
}

// field prints one aligned "label: value" line.
func field(label, value string) {
	fmt.Printf("%-14s%s\n", label+":", value)
}

// concurrentSum fans out 4 workers that square numbers and send the results
// on a channel, then sums them. The goroutines finish in any order, but the
// total is deterministic because addition doesn't care about order.
func concurrentSum() {
	const workers = 4
	nums := []int{1, 2, 3, 4, 5, 6, 7, 8}
	results := make(chan int, len(nums)) // buffered: workers never block on send

	var wg sync.WaitGroup
	for w := range workers {
		// WaitGroup.Go (Go 1.25) does Add(1), go, and defer Done() in one call.
		wg.Go(func() {
			for i := w; i < len(nums); i += workers {
				results <- nums[i] * nums[i]
			}
		})
	}
	// Close the channel once every worker is done, so the range below ends.
	go func() {
		wg.Wait()
		close(results)
	}()

	sum := 0
	for r := range results {
		sum += r
	}
	fmt.Printf("%d workers squared %v; sum of squares = %d\n", workers, nums, sum)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
