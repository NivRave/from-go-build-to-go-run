package main

import (
	"errors"
	"testing"
	"time"
)

func TestParsePseudo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		version  string
		wantBase string
		wantTime time.Time
		wantRev  string
	}{
		{
			version:  "v0.0.0-20260101120000-abcdefabcdef",
			wantBase: "",
			wantTime: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			wantRev:  "abcdefabcdef",
		},
		{
			version:  "v2.0.0-20260101120000-abcdefabcdef",
			wantBase: "", // a /v2 module before its first v2 tag
			wantTime: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			wantRev:  "abcdefabcdef",
		},
		{
			version:  "v1.2.4-0.20260102093000-0123456789ab",
			wantBase: "v1.2.3",
			wantTime: time.Date(2026, 1, 2, 9, 30, 0, 0, time.UTC),
			wantRev:  "0123456789ab",
		},
		{
			version:  "v1.3.1-0.20260102093000-0123456789ab",
			wantBase: "v1.3.0",
			wantTime: time.Date(2026, 1, 2, 9, 30, 0, 0, time.UTC),
			wantRev:  "0123456789ab",
		},
		{
			version:  "v1.2.3-pre.0.20260103180000-fedcba987654",
			wantBase: "v1.2.3-pre",
			wantTime: time.Date(2026, 1, 3, 18, 0, 0, 0, time.UTC),
			wantRev:  "fedcba987654",
		},
		{
			version:  "v1.2.3-rc.1.0.20260103180000-fedcba987654",
			wantBase: "v1.2.3-rc.1",
			wantTime: time.Date(2026, 1, 3, 18, 0, 0, 0, time.UTC),
			wantRev:  "fedcba987654",
		},
		{
			version:  "v2.0.1-0.20260104000000-aaaaaaaaaaaa+incompatible",
			wantBase: "v2.0.0",
			wantTime: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			wantRev:  "aaaaaaaaaaaa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()
			p, err := ParsePseudo(tt.version)
			if err != nil {
				t.Fatalf("ParsePseudo(%q) error: %v", tt.version, err)
			}
			if p.Base != tt.wantBase || !p.Time.Equal(tt.wantTime) || p.Rev != tt.wantRev {
				t.Errorf("ParsePseudo(%q) = base %q, time %v, rev %q; want %q, %v, %q",
					tt.version, p.Base, p.Time, p.Rev, tt.wantBase, tt.wantTime, tt.wantRev)
			}
		})
	}
}

func TestParsePseudoRejects(t *testing.T) {
	t.Parallel()
	for _, v := range []string{
		"v1.2.3",                               // a plain release
		"v1.2.4-patch",                         // a pre-release tag (slide p04)
		"v1.2.0-0.20260102093000-0123456789ab", // "-0." form needs patch >= 1
		"v1.2.3-20260101120000-abcdefabcdef",   // no-tag form must be vX.0.0
		"v0.0.0-20261301120000-abcdefabcdef",   // month 13
		"v0.0.0-20260101120000-abc",            // revision too short
		"latest",
	} {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			if _, err := ParsePseudo(v); !errors.Is(err, ErrNotPseudo) {
				t.Errorf("ParsePseudo(%q) error = %v, want ErrNotPseudo", v, err)
			}
		})
	}
}

func Example_decodePseudoVersions() {
	decodePseudoVersions()
	// Output:
	// v0.0.0-20260101120000-abcdefabcdef  (no tag yet)
	//   base tag: (none)
	//   time:     2026-01-01T12:00:00Z
	//   commit:   abcdefabcdef
	//   sorts:    before every tagged release
	// v1.2.4-0.20260102093000-0123456789ab  (commit after tag v1.2.3)
	//   base tag: v1.2.3
	//   time:     2026-01-02T09:30:00Z
	//   commit:   0123456789ab
	//   sorts:    after v1.2.3, before the next release
	// v1.2.3-pre.0.20260103180000-fedcba987654  (commit after pre-release tag v1.2.3-pre)
	//   base tag: v1.2.3-pre
	//   time:     2026-01-03T18:00:00Z
	//   commit:   fedcba987654
	//   sorts:    after v1.2.3-pre, before the next release
}
