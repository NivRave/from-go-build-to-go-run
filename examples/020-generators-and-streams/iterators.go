package main

import "iter"

// Since Go 1.23 the standard shape for a generator is iter.Seq: a function
// that pushes values into `yield` until yield returns false. Callers consume
// it with a plain `for range` loop, and `break` stops the producer.

// Ints yields every integer in the inclusive range [start, end].
func Ints(start, end int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for v := start; v <= end; v++ {
			if !yield(v) {
				return // consumer broke out of its loop: stop producing
			}
		}
	}
}

// Filter yields the values of seq for which keep returns true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

// Map yields f(v) for every value of seq.
func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}
