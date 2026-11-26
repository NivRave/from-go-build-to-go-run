package main

// IntGenerator returns a pull-style generator for the inclusive range
// [start, end]. The returned closure owns `current`; because the closure
// outlives this call, escape analysis moves `current` to the heap (slide p03).
// It is a single small allocation regardless of how many values are produced.
//
// A generator is NOT safe for concurrent use: two goroutines calling the same
// closure race on `current`. Create one generator per goroutine instead.
func IntGenerator(start, end int) func() (int, bool) {
	current := start
	return func() (int, bool) {
		if current > end {
			return 0, false // sentinel: the stream is exhausted
		}
		val := current
		current++
		return val, true
	}
}

// FilterEven wraps a generator and yields only even values (slide p06).
func FilterEven(next func() (int, bool)) func() (int, bool) {
	return func() (int, bool) {
		for {
			v, ok := next()
			if !ok {
				return 0, false
			}
			if v%2 == 0 {
				return v, true
			}
		}
	}
}

// Multiply wraps a generator and scales every value by factor (slide p06).
func Multiply(next func() (int, bool), factor int) func() (int, bool) {
	return func() (int, bool) {
		v, ok := next()
		if !ok {
			return 0, false
		}
		return v * factor, true
	}
}
