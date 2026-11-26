package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// RenderReport formats settings as an aligned text table. Its output is long
// enough that comparing it to a golden file (golden_test.go) beats writing
// the expected string inline.
func RenderReport(title string, settings []KV) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", title, strings.Repeat("=", len(title)))

	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tLENGTH")
	for _, kv := range settings {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", kv.Key, kv.Value, len(kv.Value))
	}
	tw.Flush()

	fmt.Fprintf(&b, "%d settings\n", len(settings))
	return b.String()
}
