// Package auth is private to examples/027-packages-and-go-mod (slide p03).
//
// Because it lives under an internal/ directory, the go command lets only
// code rooted at examples/027-packages-and-go-mod import it. Any other
// package, even one in the same module, gets a compile error:
//
//	use of internal package github.com/NivRave/from-go-build-to-go-run/examples/027-packages-and-go-mod/internal/auth not allowed
//
// That makes it safe to change everything in here without breaking anyone.
package auth

import "slices"

// admins is unexported state; callers only see Allowed.
var admins = []string{"gopher", "rob"}

// Allowed reports whether user may use the admin console.
func Allowed(user string) bool {
	return slices.Contains(admins, user)
}
