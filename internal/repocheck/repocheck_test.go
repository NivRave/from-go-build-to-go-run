// Package repocheck enforces the repository layout conventions described in
// CONTRIBUTING.md. It contains only tests and is run by `go test ./...`.
package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// requireAllExamples is flipped to true once every part has an example
// (final phase of the rollout). Until then, missing examples are allowed.
const requireAllExamples = false

var (
	slugRE = regexp.MustCompile(`^\d{3}-[a-z0-9]+(-[a-z0-9]+)*$`)
	linkRE = regexp.MustCompile(`\]\(([^)\s]+)\)`)

	// Headings every examples/<slug>/README.md must contain, in this order.
	requiredHeadings = []string{
		"## What this shows",
		"## Run",
		"## What to look for",
		"## Slides",
	}
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// slugs returns the base names in dir that match want (".pdf" files or dirs).
func slugs(t *testing.T, dir string, wantDir bool) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	out := make(map[string]bool)
	for _, e := range entries {
		name := e.Name()
		switch {
		case wantDir && e.IsDir():
			out[name] = true
		case !wantDir && !e.IsDir() && strings.HasSuffix(name, ".pdf"):
			out[strings.TrimSuffix(name, ".pdf")] = true
		case !wantDir:
			t.Errorf("slides/%s: only .pdf files belong in slides/", name)
		}
	}
	return out
}

func TestLayout(t *testing.T) {
	root := repoRoot(t)
	slides := slugs(t, filepath.Join(root, "slides"), false)
	examples := slugs(t, filepath.Join(root, "examples"), true)

	for s := range slides {
		if !slugRE.MatchString(s) {
			t.Errorf("slides/%s.pdf: name must be NNN-kebab-case.pdf", s)
		}
		if requireAllExamples && !examples[s] {
			t.Errorf("slides/%s.pdf has no examples/%s/ directory", s, s)
		}
	}

	for e := range examples {
		if !slides[e] {
			t.Errorf("examples/%s/ has no matching slides/%s.pdf", e, e)
			continue
		}
		dir := filepath.Join(root, "examples", e)
		src, err := os.ReadFile(filepath.Join(dir, "main.go"))
		if err != nil {
			t.Errorf("examples/%s/main.go is missing", e)
		} else if want := "// Slides: slides/" + e + ".pdf"; !strings.Contains(string(src), want) {
			t.Errorf("examples/%s/main.go doc comment must contain %q", e, want)
		}
		checkExampleReadme(t, e, filepath.Join(dir, "README.md"))
	}
}

func checkExampleReadme(t *testing.T, slug, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("examples/%s/README.md is missing", slug)
		return
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n") // tolerate CRLF checkouts
	if want := "# Part " + slug[:3] + ": "; !strings.HasPrefix(text, want) {
		t.Errorf("examples/%s/README.md must start with %q", slug, want)
	}
	last := -1
	for _, h := range requiredHeadings {
		i := strings.Index(text, "\n"+h+"\n")
		if i < 0 {
			t.Errorf("examples/%s/README.md is missing heading %q", slug, h)
			continue
		}
		if i < last {
			t.Errorf("examples/%s/README.md: heading %q is out of order", slug, h)
		}
		last = i
	}
	if want := "../../slides/" + slug + ".pdf"; !strings.Contains(text, "("+want+")") {
		t.Errorf("examples/%s/README.md must link to %s", slug, want)
	}
	checkLinks(t, filepath.Dir(path), text, "examples/"+slug+"/README.md")
}

// TestRootReadmeLinks verifies every relative link in README.md and
// CONTRIBUTING.md points at a file or directory that exists.
func TestRootReadmeLinks(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"README.md", "CONTRIBUTING.md"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		checkLinks(t, root, string(b), name)
	}
}

func checkLinks(t *testing.T, base, text, file string) {
	t.Helper()
	for _, m := range linkRE.FindAllStringSubmatch(text, -1) {
		target := m[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		target, _, _ = strings.Cut(target, "#")
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(target))); err != nil {
			t.Errorf("%s: broken link %q", file, m[1])
		}
	}
}
