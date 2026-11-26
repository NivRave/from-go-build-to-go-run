package main

// Section 3 (growth trace) is not asserted: the capacities depend on the Go
// version and allocator size classes. The array sizes in section 1 depend
// on the platform's word size, so only arrayValues is asserted there.

func Example_arrayValues() {
	arrayValues()
	// Output:
	// after b := a; b[0] = 99:  a = [1 2 3]  b = [99 2 3]
	// after mutateArray(a):     a = [1 2 3]
	// after mutateSlice(s):     s = [100 2 3]
}

func Example_sharedBacking() {
	sharedBacking()
	// Output:
	// sub[0] = 99:          original = [1 99 3 4]  sub = [99 3]
	// after slices.Clone:   original = [1 2 3 4]  independent = [99 3]
}

func Example_nilVsEmpty() {
	nilVsEmpty()
	// Output:
	// var s []int         == nil: true  len 0 cap  0  []          JSON null
	// []int{}             == nil: false len 0 cap  0  []          JSON []
	// make([]int, 0, 5)   == nil: false len 0 cap  5  []          JSON []
	// make([]int, 5, 10)  == nil: false len 5 cap 10  [0 0 0 0 0] JSON [0,0,0,0,0]
	// Payload: {"nil":null,"empty":[],"omitzero_empty":[]}
}
