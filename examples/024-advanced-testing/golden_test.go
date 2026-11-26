package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update is not a built-in go test flag: this file declares it, and go test
// passes flags it doesn't know through to the test binary.
//
//	go test ./examples/024-advanced-testing -update
//
// rewrites testdata/report.golden from the current output. Review the change
// with git diff before committing it.
var update = flag.Bool("update", false, "update golden files")

// Tests run with the package directory as the working directory, so a
// relative testdata path is the convention (go build ignores testdata/).
var goldenPath = filepath.Join("testdata", "report.golden")

func renderSample(t *testing.T) string {
	t.Helper()
	settings, err := ParseLines(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatal(err)
	}
	return RenderReport("Service configuration", settings)
}

func readGolden(t *testing.T) string {
	t.Helper()
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file (run with -update to create it): %v", err)
	}
	// Tolerate a CRLF checkout on Windows.
	return strings.ReplaceAll(string(want), "\r\n", "\n")
}

func TestRenderReportGolden(t *testing.T) {
	got := renderSample(t)

	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", goldenPath)
	}

	if want := readGolden(t); got != want {
		t.Errorf("RenderReport output differs from %s (run with -update if the change is intended)\n--- got:\n%s\n--- want:\n%s",
			goldenPath, got, want)
	}
}

// TestReportFromFile is integration-style: it goes through the real file
// system. Heavy tests like this (databases, containers, networks) are the
// ones to skip during quick local runs with -short.
func TestReportFromFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping file-system test in -short mode")
	}
	t.Parallel()

	path := filepath.Join(t.TempDir(), "service.conf") // removed automatically
	if err := os.WriteFile(path, []byte(sampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	settings, err := ParseLines(f)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := RenderReport("Service configuration", settings), readGolden(t); got != want {
		t.Errorf("report from file differs from %s:\n%s", goldenPath, got)
	}
}

func TestParseLinesReportsLine(t *testing.T) {
	t.Parallel()
	_, err := ParseLines(strings.NewReader("a=1\n\nbroken\n"))
	if err == nil || !strings.HasPrefix(err.Error(), "line 3: ") {
		t.Fatalf("err = %v, want it to start with \"line 3: \"", err)
	}
}
