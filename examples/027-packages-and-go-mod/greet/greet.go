// Package greet is the public part of examples/027-packages-and-go-mod.
//
// Slide p03 puts public code under pkg/. This example follows the slide p07
// recommendation for small modules instead and keeps public packages at the
// top of the tree, so the import path has no extra /pkg segment.
package greet

import "fmt"

// Hello returns a greeting for name.
func Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
