package main

import (
	"reflect"
	"testing"
)

// Each select locks every channel it lists (in address order), polls them in
// a random order, then unlocks them, so its cost grows with the number of
// cases even when only one of them is ready. Run with:
//
//	go test -run='^$' -bench=Select ./examples/035-select
//
// In every benchmark exactly one channel (the last case) holds a value.

func newChans(n int) []chan int {
	cs := make([]chan int, n)
	for i := range cs {
		cs[i] = make(chan int, 1)
	}
	return cs
}

func BenchmarkSelect2Cases(b *testing.B) {
	c := newChans(2)
	for b.Loop() {
		c[1] <- 1
		select {
		case <-c[0]:
		case <-c[1]:
		}
	}
}

func BenchmarkSelect16Cases(b *testing.B) {
	c := newChans(16)
	for b.Loop() {
		c[15] <- 1
		select {
		case <-c[0]:
		case <-c[1]:
		case <-c[2]:
		case <-c[3]:
		case <-c[4]:
		case <-c[5]:
		case <-c[6]:
		case <-c[7]:
		case <-c[8]:
		case <-c[9]:
		case <-c[10]:
		case <-c[11]:
		case <-c[12]:
		case <-c[13]:
		case <-c[14]:
		case <-c[15]:
		}
	}
}

// reflect.Select handles a number of cases only known at run time (slide
// p08 does not mention it). It is slower than a compiled select of the same
// size, so use it when the case count is genuinely dynamic.
func BenchmarkReflectSelect16Cases(b *testing.B) {
	c := newChans(16)
	cases := make([]reflect.SelectCase, len(c))
	for i, ch := range c {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	for b.Loop() {
		c[15] <- 1
		reflect.Select(cases)
	}
}
