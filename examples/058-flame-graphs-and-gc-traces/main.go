// Command flame-graphs-and-gc-traces demonstrates allocation pressure and how
// to prove an optimisation: it runs an allocation-heavy loop and an optimised
// one, prints GC metrics for each, and writes heap profiles to diff.
//
// Slides: slides/058-flame-graphs-and-gc-traces.pdf
//
// Run:
//
//	go run ./examples/058-flame-graphs-and-gc-traces
//	GODEBUG=gctrace=1 go run ./examples/058-flame-graphs-and-gc-traces
//	go test -run='^$' -bench=. -benchmem ./examples/058-flame-graphs-and-gc-traces
package main

import (
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"runtime/pprof"
	"strconv"
)

// lines is how many log lines each loop formats. The heavy loop allocates
// roughly 70 MB in total but never holds more than a few bytes of it.
const lines = 1_000_000

// A heap profile is cumulative since the process started, so two profiles
// taken one after the other in the same process can't show "before vs
// after": the second contains the first. Slide p08 compares two runs, so this
// program does the same: it starts itself twice, once per phase, and tells
// each child which phase to run through these environment variables.
const (
	phaseEnv   = "FLAMES_PHASE"
	profileEnv = "FLAMES_PROFILE"
)

func main() {
	if phase := os.Getenv(phaseEnv); phase != "" {
		if err := runPhase(phase, os.Getenv(profileEnv)); err != nil {
			log.Fatal(err)
		}
		return
	}

	dir, err := os.MkdirTemp("", "flames-demo-")
	if err != nil {
		log.Fatal(err)
	}
	before := filepath.Join(dir, "heap_before.pprof")
	after := filepath.Join(dir, "heap_after.pprof")

	section("1. Allocation-heavy loop: fmt.Sprintf per line (slides p04, p05)")
	if err := runChild("before", before); err != nil {
		log.Fatal(err)
	}

	section("2. Optimised loop: one reused []byte buffer (slide p07)")
	if err := runChild("after", after); err != nil {
		log.Fatal(err)
	}

	section("3. Prove the win with a differential profile (slides p06, p08)")
	fmt.Println("# red = more than the base, green = less; View -> Flame Graph:")
	fmt.Printf("go tool pprof -http=:8080 -sample_index=alloc_space -diff_base=%s %s\n", before, after)
	fmt.Println("# the same comparison as text:")
	fmt.Printf("go tool pprof -top -sample_index=alloc_space -diff_base=%s %s\n", before, after)
	fmt.Println("# clean up when done:")
	fmt.Printf("rm -r %s\n", dir)
}

// runChild re-runs this binary with phaseEnv set, so the phase gets a fresh
// heap and a heap profile of its own. GODEBUG (for gctrace) is inherited.
func runChild(phase, profile string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), phaseEnv+"="+phase, profileEnv+"="+profile)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// runPhase runs one loop, prints what the GC saw, and writes a heap profile.
func runPhase(phase, profile string) error {
	var sum uint64
	switch phase {
	case "before":
		sum = heavyLoop(lines)
	case "after":
		sum = lightLoop(lines)
	default:
		return fmt.Errorf("unknown phase %q", phase)
	}
	// The checksum is identical for both loops: same output, different cost.
	fmt.Printf("formatted %d lines, checksum %x\n", lines, sum)
	printGCMetrics()

	// A heap profile shows the state as of the last completed GC. The
	// optimised loop may finish without triggering one, so force it.
	runtime.GC()
	f, err := os.Create(profile)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", profile)
	return f.Close()
}

// heavyLoop formats every line with fmt.Sprintf and converts it to []byte
// for the hash: a new string, a boxed int and a new byte slice per line.
// Nothing is retained, so this is churn, not a leak (slide p06).
func heavyLoop(n int) uint64 {
	h := fnv.New64a()
	for i := range n {
		line := fmt.Sprintf("event=%d status=%s\n", i, "processed")
		_, _ = io.WriteString(h, line)
	}
	return h.Sum64()
}

// lightLoop writes the same bytes into one reused buffer with strconv, so the
// loop body allocates nothing.
func lightLoop(n int) uint64 {
	h := fnv.New64a()
	buf := make([]byte, 0, 64)
	for i := range n {
		buf = append(buf[:0], "event="...)
		buf = strconv.AppendInt(buf, int64(i), 10)
		buf = append(buf, " status=processed\n"...)
		_, _ = h.Write(buf)
	}
	return h.Sum64()
}

// printGCMetrics reads the two runtime/metrics counters from the spec. Both
// are cumulative since the process started, which here means "this phase".
func printGCMetrics() {
	samples := []metrics.Sample{
		{Name: "/gc/cycles/total:gc-cycles"},
		{Name: "/gc/heap/allocs:bytes"},
	}
	metrics.Read(samples)
	fmt.Printf("GC cycles:       %d\n", samples[0].Value.Uint64())
	fmt.Printf("heap allocated:  %.1f MB\n", float64(samples[1].Value.Uint64())/1e6)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
