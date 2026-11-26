package main

// FilterInPlace keeps the even numbers, reusing nums's backing array (slide
// p02, with the missing newline after n := 0 restored). It allocates nothing,
// but it overwrites the caller's data: only use it on a throwaway slice.
//
// For element types that contain pointers, also clear(nums[n:]) before
// returning, so the stale tail doesn't keep garbage reachable.
func FilterInPlace(nums []int) []int {
	n := 0
	for _, x := range nums {
		if x%2 == 0 {
			nums[n] = x // Overwriting the original array
			n++
		}
	}
	return nums[:n] // Return only the filtered part
}

// FilterAlloc keeps the even numbers in a new slice and leaves nums alone.
func FilterAlloc(nums []int) []int {
	out := make([]int, 0, len(nums))
	for _, x := range nums {
		if x%2 == 0 {
			out = append(out, x)
		}
	}
	return out
}

func isOdd(x int) bool { return x%2 != 0 }
