package main

import (
	"slices"
	"strconv"
	"strings"
)

// A Graph maps a module version ("path@vX.Y.Z") to the module versions its
// go.mod requires. The main module is written without a version. A version
// that requires nothing can be left out of the map.
type Graph map[string][]string

// BuildList runs Minimal Version Selection (slide p04) from root: walk every
// module version reachable from root and, for each module path, keep the
// highest version that anything requires. It never picks a version nobody
// asked for, so a newer release appears in the build only when someone raises
// a requirement (for example with go get -u).
//
// This is the algorithm from Russ Cox's "Minimal Version Selection"
// (research.swtch.com/vgo-mvs), without the go command's refinements such as
// graph pruning, exclude and replace.
//
// The result lists root first, then one "path@version" per dependency, sorted
// by path.
func BuildList(g Graph, root string) []string {
	selected := map[string]string{} // module path -> highest version seen
	seen := map[string]bool{root: true}
	queue := []string{root}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, req := range g[node] {
			path, version, _ := strings.Cut(req, "@")
			if cur, ok := selected[path]; !ok || compareVersions(version, cur) > 0 {
				selected[path] = version
			}
			// Requirements of versions that end up *not* selected still count:
			// they are part of the graph, so they may raise other minimums.
			if !seen[req] {
				seen[req] = true
				queue = append(queue, req)
			}
		}
	}

	rootPath, _, _ := strings.Cut(root, "@")
	list := make([]string, 0, len(selected))
	for path, version := range selected {
		if path != rootPath {
			list = append(list, path+"@"+version)
		}
	}
	slices.Sort(list) // map order is random; sort for stable output
	return append([]string{root}, list...)
}

// compareVersions compares two release versions of the form vMAJOR.MINOR.PATCH
// numerically, returning -1, 0 or +1. (Plain string comparison would put
// v1.10.0 before v1.9.0.) Pre-release and build suffixes are out of scope for
// this simulator.
func compareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := range pa {
		if c := pa[i] - pb[i]; c != 0 {
			if c < 0 {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			continue // malformed component: treat as 0
		}
		out[i] = n
	}
	return out
}
