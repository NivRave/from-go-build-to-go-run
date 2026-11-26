package main

// Section 1 (heap sizes) and section 3 (memory sharing) are not asserted:
// heap figures vary by machine and GC timing.

func Example_equalFold() {
	equalFold()
	// Output:
	// "Go" vs "GO"           EqualFold=true  ToLower==ToLower=true
	// "\u00df" vs "SS"       EqualFold=false ToLower==ToLower=false
	// "\u212a" vs "k"        EqualFold=true  ToLower==ToLower=true
}

func Example_validation() {
	validation()
	// Output:
	// validatePayload("ok") = <nil>
	// validatePayload("ok\xff\xfeok") = malformed UTF-8
	// strings.ToValidUTF8 -> "ok\ufffdok"
	// range over "\xff\xfe": 0:U+FFFD 1:U+FFFD
}

func Example_hostPort() {
	hostPort()
	// Output:
	// example.com  Sprintf: example.com:8080  JoinHostPort: example.com:8080
	// ::1          Sprintf: ::1:8080          JoinHostPort: [::1]:8080
	// strings.Join([]string{part1, part2}, "-") = go-build
}
