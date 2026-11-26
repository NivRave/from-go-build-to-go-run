package main

// Section 4 (heap sizes) is not asserted: the figures vary by machine and
// GC timing.

func Example_filtering() {
	filtering()
	// Output:
	// FilterInPlace:       result [2 4 6]  input now [2 4 6 4 5 6]
	// slices.DeleteFunc:   result [2 4 6]  input now [2 4 6 0 0 0]
	// Clone, then filter:  result [2 4 6]  input now [1 2 3 4 5 6]
}

func Example_appendGhost() {
	appendGhost()
	// Output:
	// child := parent[:2]:   parent [1 2 99 4]  child [1 2 99] (the 3 is gone)
	// child := parent[:2:2]: parent [1 2 3 4]  child [1 2 99]
}

func Example_threeIndex() {
	threeIndex()
	// Output:
	// s1: len 3 cap 5; s2: len 3 cap 3
	// s1 after append to s1: [1 2 3 10 11]
	// s2 after append to s1: [1 2 3]
	// array after s1 append: [0 1 2 3 10 11]
	// s1 after append to s2: [1 2 3 10 11]
	// s2 after append to s2: [1 2 3 20]
	// array after s2 append: [0 1 2 3 10 11]
}

func Example_nilVsEmptyJSON() {
	nilVsEmptyJSON()
	// Output:
	// []int(nil)                         -> null
	// []int{}                            -> []
	// main.Response{Items:[]string(nil)} -> {"items":null}
	// main.Response{Items:[]string{}}    -> {"items":[]}
}
