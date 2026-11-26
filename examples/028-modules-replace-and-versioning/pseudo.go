package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Pseudo is a decoded pseudo-version (slide p07). The go command generates
// one when you depend on a commit that has no tag. Its three forms all sort
// after the tag they are based on and before the next possible release:
//
//	vX.0.0-yyyymmddhhmmss-abcdefabcdef        no earlier tag
//	vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdefabcdef  latest tag is vX.Y.Z
//	vX.Y.Z-pre.0.yyyymmddhhmmss-abcdefabcdef  latest tag is vX.Y.Z-pre
type Pseudo struct {
	Version string
	Base    string    // the tag this commit follows; "" when there is none
	Time    time.Time // commit time, always UTC
	Rev     string    // 12-character commit hash prefix
}

// ErrNotPseudo is returned for versions that are not pseudo-versions.
var ErrNotPseudo = errors.New("not a pseudo-version")

// ParsePseudo decodes v. It is a hand-rolled, stdlib-only version of what
// golang.org/x/mod/module.PseudoVersionBase and friends do.
func ParsePseudo(v string) (Pseudo, error) {
	notPseudo := fmt.Errorf("%q: %w", v, ErrNotPseudo)

	// Strip a build suffix such as +incompatible; it isn't part of the encoding.
	core, _, _ := strings.Cut(v, "+")

	// The revision is everything after the last '-'.
	i := strings.LastIndexByte(core, '-')
	if i < 0 {
		return Pseudo{}, notPseudo
	}
	rest, rev := core[:i], core[i+1:]
	if len(rev) != 12 {
		return Pseudo{}, notPseudo
	}

	// The 14-digit UTC timestamp sits just before the revision.
	const stamp = "20060102150405"
	if len(rest) < len(stamp)+1 {
		return Pseudo{}, notPseudo
	}
	ts, prefix := rest[len(rest)-len(stamp):], rest[:len(rest)-len(stamp)]
	t, err := time.Parse(stamp, ts)
	if err != nil {
		return Pseudo{}, notPseudo
	}

	p := Pseudo{Version: v, Time: t, Rev: rev}
	switch {
	case strings.HasSuffix(prefix, "-0."):
		// vX.Y.(Z+1)-0.<ts>: undo the patch increment to recover vX.Y.Z.
		base := strings.TrimSuffix(prefix, "-0.")
		major, minor, patch, ok := splitRelease(base)
		if !ok || patch == 0 {
			return Pseudo{}, notPseudo
		}
		p.Base = fmt.Sprintf("v%d.%d.%d", major, minor, patch-1)
	case strings.HasSuffix(prefix, ".0."):
		// vX.Y.Z-pre.0.<ts>: the base is the pre-release tag itself.
		base := strings.TrimSuffix(prefix, ".0.")
		release, pre, found := strings.Cut(base, "-")
		if _, _, _, ok := splitRelease(release); !ok || !found || pre == "" {
			return Pseudo{}, notPseudo
		}
		p.Base = base
	case strings.HasSuffix(prefix, "-"):
		// vX.0.0-<ts>: no tag before this commit.
		_, minor, patch, ok := splitRelease(strings.TrimSuffix(prefix, "-"))
		if !ok || minor != 0 || patch != 0 {
			return Pseudo{}, notPseudo
		}
	default:
		return Pseudo{}, notPseudo
	}
	return p, nil
}

// splitRelease parses "vMAJOR.MINOR.PATCH".
func splitRelease(v string) (major, minor, patch int, ok bool) {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if !strings.HasPrefix(v, "v") || len(parts) != 3 {
		return 0, 0, 0, false
	}
	var n [3]int
	for i, s := range parts {
		x, err := strconv.Atoi(s)
		if err != nil || x < 0 {
			return 0, 0, 0, false
		}
		n[i] = x
	}
	return n[0], n[1], n[2], true
}
