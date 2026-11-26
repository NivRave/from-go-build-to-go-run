package main

import (
	"slices"
	"testing"
)

func TestBuildList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		graph Graph
		root  string
		want  []string
	}{
		{
			name:  "no dependencies",
			graph: Graph{},
			root:  "app",
			want:  []string{"app"},
		},
		{
			name: "highest minimum wins, never a newer release",
			graph: Graph{
				"app":        {"a@v1.0.0", "b@v1.0.0"},
				"a@v1.0.0":   {"lib@v1.2.0"},
				"b@v1.0.0":   {"lib@v1.5.0"},
				"lib@v1.9.0": {}, // exists, but nobody requires it
			},
			root: "app",
			want: []string{"app", "a@v1.0.0", "b@v1.0.0", "lib@v1.5.0"},
		},
		{
			name: "versions compare numerically, not as strings",
			graph: Graph{
				"app":      {"a@v1.0.0", "b@v1.0.0"},
				"a@v1.0.0": {"lib@v1.10.0"},
				"b@v1.0.0": {"lib@v1.9.0"},
			},
			root: "app",
			want: []string{"app", "a@v1.0.0", "b@v1.0.0", "lib@v1.10.0"},
		},
		{
			name: "requirements of unselected versions still count",
			graph: Graph{
				"app":        {"a@v1.0.0", "b@v1.0.0"},
				"a@v1.0.0":   {"lib@v1.2.0"},
				"b@v1.0.0":   {"lib@v1.5.0"},
				"lib@v1.2.0": {"log@v1.1.0"},
				"lib@v1.5.0": {"log@v1.0.0"},
			},
			root: "app",
			want: []string{"app", "a@v1.0.0", "b@v1.0.0", "lib@v1.5.0", "log@v1.1.0"},
		},
		{
			name: "main module requirement raises the minimum (go get)",
			graph: Graph{
				"app":      {"a@v1.0.0", "lib@v1.5.2"},
				"a@v1.0.0": {"lib@v1.5.0"},
			},
			root: "app",
			want: []string{"app", "a@v1.0.0", "lib@v1.5.2"},
		},
		{
			name: "cycles terminate",
			graph: Graph{
				"app":      {"a@v1.0.0"},
				"a@v1.0.0": {"b@v1.0.0"},
				"b@v1.0.0": {"a@v1.1.0"},
				"a@v1.1.0": {"b@v1.0.0"},
			},
			root: "app",
			want: []string{"app", "a@v1.1.0", "b@v1.0.0"},
		},
		{
			name: "unreachable versions are ignored",
			graph: Graph{
				"app":          {"a@v1.0.0"},
				"other@v2.0.0": {"a@v1.4.0"},
			},
			root: "app",
			want: []string{"app", "a@v1.0.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := BuildList(tt.graph, tt.root); !slices.Equal(got, tt.want) {
				t.Errorf("BuildList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.2.0", "v1.2.0", 0},
		{"v1.2.0", "v1.5.0", -1},
		{"v1.10.0", "v1.9.0", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.5.2", "v1.5.10", -1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			t.Parallel()
			if got := compareVersions(tt.a, tt.b); got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// Example tests keep the README's expected output honest. buildInfo has none:
// its output depends on how the binary was built.

func Example_packages() {
	packages()
	// Output:
	// Hello, gopher!   admin=true
	// Hello, guest!    admin=false
}

func Example_mvsSlideExample() {
	mvsSlideExample()
	// Output:
	// serviceA needs lib v1.2.0, serviceB needs lib v1.5.0, lib v1.9.0 exists:
	//   build list: myapp lib@v1.5.0 log@v1.1.0 serviceA@v1.0.0 serviceB@v1.0.0
	//   lib: the highest minimum (v1.5.0), not the newest release (v1.9.0)
	//   log: v1.1.0, because lib v1.2.0 is still in the graph and asks for it
}

func Example_mvsUpgrade() {
	mvsUpgrade()
	// Output:
	// after go get lib@v1.5.2 (myapp now requires it directly):
	//   build list: myapp lib@v1.5.2 log@v1.1.0 serviceA@v1.0.0 serviceB@v1.0.0
}
