// Command pprof demonstrates CPU, heap, mutex and block profiling: by default
// it runs three wasteful workloads, writes the profiles to a temporary
// directory and prints the go tool pprof commands to open them; with -serve it
// exposes a live service and the net/http/pprof endpoints, safely.
//
// Slides: slides/057-pprof.pdf
//
// Run:
//
//	go run ./examples/057-pprof
//	go run ./examples/057-pprof -serve -load
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
)

func main() {
	serveFlag := flag.Bool("serve", false, "run the app on "+appAddr+" and pprof on "+pprofAddr+" until Ctrl+C")
	loadFlag := flag.Bool("load", false, "with -serve: start a built-in load generator (implies -serve)")
	flag.Parse()

	// Slide p08 uses 1 for both, which records every single event. That is
	// fine on a laptop but costly in production. These values sample one
	// blocking event per 10 µs spent blocked, and 1 in 100 contention events.
	runtime.SetBlockProfileRate(10_000)
	runtime.SetMutexProfileFraction(100)

	if *serveFlag || *loadFlag {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := serve(ctx, *loadFlag); err != nil {
			log.Fatal(err)
		}
		return
	}

	dir, err := os.MkdirTemp("", "pprof-demo-")
	if err != nil {
		log.Fatal(err)
	}

	section("1. CPU profile while three workloads run (slides p02, p04)")
	cpuPath := filepath.Join(dir, "cpu.pprof")
	if err := profileCPU(cpuPath); err != nil {
		log.Fatal(err)
	}

	section("2. Heap, mutex and block profiles (slides p05, p08)")
	for _, name := range []string{"heap", "mutex", "block"} {
		if err := writeProfile(name, filepath.Join(dir, name+".pprof")); err != nil {
			log.Fatal(err)
		}
	}

	section("3. Open them with go tool pprof (slide p06)")
	printCommands(dir)
}

// profileCPU records a CPU profile to path while the three workloads run.
// The profiler samples the running goroutines' stacks about 100 times per
// second (a SIGPROF timer on Unix); it does not stop the world.
func profileCPU(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return err
	}
	start := time.Now()
	// The counts vary by machine; they are printed so the work can't be
	// optimised away and so you can compare runs.
	fmt.Printf("hashWork:  %d SHA-256 rounds (CPU-bound)\n", hashWork(700*time.Millisecond))
	fmt.Printf("allocWork: %d bytes of JSON (allocation-bound)\n", allocWork(700*time.Millisecond))
	fmt.Printf("lockWork:  %d increments by 8 goroutines (lock-contended)\n", lockWork(500*time.Millisecond, 8))
	pprof.StopCPUProfile()
	fmt.Printf("wrote %s (%.1fs of samples)\n", path, time.Since(start).Seconds())
	return f.Close()
}

// writeProfile writes one of the built-in runtime/pprof profiles in the
// binary format go tool pprof reads.
func writeProfile(name, path string) error {
	if name == "heap" {
		// The heap profile reflects the state as of the last GC. Forcing one
		// makes "in-use" numbers current.
		runtime.GC()
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := pprof.Lookup(name).WriteTo(f, 0); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	return f.Close()
}

func printCommands(dir string) {
	p := func(name string) string { return filepath.Join(dir, name) }
	fmt.Println("# hottest functions, flat and cumulative:")
	fmt.Printf("go tool pprof -top %s\n", p("cpu.pprof"))
	fmt.Println("# interactive graph and flame graph (View -> Flame Graph):")
	fmt.Printf("go tool pprof -http=:8081 %s\n", p("cpu.pprof"))
	fmt.Println("# every byte ever allocated (churn) instead of what is still live:")
	fmt.Printf("go tool pprof -http=:8081 -sample_index=alloc_space %s\n", p("heap.pprof"))
	fmt.Println("# who held contended locks / who waited:")
	fmt.Printf("go tool pprof -top %s\n", p("mutex.pprof"))
	fmt.Printf("go tool pprof -top %s\n", p("block.pprof"))
	fmt.Println("# clean up when done:")
	fmt.Printf("rm -r %s\n", dir)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
