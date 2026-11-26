package main

// Role is a bitmask (slide p08): each constant is a distinct bit, so a single
// value can hold any combination. 1 << iota works because iota counts the
// lines of the const block and the omitted expressions repeat "1 << iota".
// The standard library's fs.FileMode permission bits use the same pattern.
type Role int

const (
	Admin  Role = 1 << iota // 1 (0001)
	Editor                  // 2 (0010)
	Viewer                  // 4 (0100)
)

// Has reports whether every bit of flag is set in r.
func (r Role) Has(flag Role) bool {
	return r&flag == flag
}
