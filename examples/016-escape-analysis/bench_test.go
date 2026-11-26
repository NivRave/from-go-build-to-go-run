package main

import "testing"

// Run with:
//
//	go test -bench=. -benchmem ./examples/016-escape-analysis
//
// The allocs/op column is the result. The benchmarks reuse the demo
// functions from main.go, one per demo line.

// TestBoxingAllocs pins the boxing results with testing.AllocsPerRun. It
// checks only results that do not depend on inlining, so it also holds when
// optimisations are disabled with -gcflags=-l.
func TestBoxingAllocs(t *testing.T) {
	tests := []struct {
		name string
		f    func()
		want float64
	}{
		{"big struct", func() { sinkAny = bigStruct }, 1},
		{"constant 42", func() { sinkAny = 42 }, 0},
		{"small value at run time", func() { sinkAny = smallValue }, 0},
		{"large value at run time", func() { sinkAny = largeValue }, 1},
		{"noinline pointer return", func() { sinkInt = GetConfigPointerNoInline().Timeout }, 1},
		{"store(&u)", func() { u := User{Age: 1}; store(&u) }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: AllocsPerRun counts allocations process-wide.
			if got := testing.AllocsPerRun(100, tt.f); got != tt.want {
				t.Errorf("allocs per run = %v, want %v", got, tt.want)
			}
		})
	}
}

// 1. Inlining

func BenchmarkHeapAllocInlined(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = *HeapAlloc()
	}
}

func BenchmarkGetConfigPointerInlined(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = GetConfigPointer().Timeout
	}
}

func BenchmarkGetConfigPointerNoInline(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = GetConfigPointerNoInline().Timeout
	}
}

func BenchmarkGetConfigValue(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = GetConfigValue().Timeout
	}
}

// 2. Size

func BenchmarkMakeConst8(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = constSmall()
	}
}

func BenchmarkMakeConst1M(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = constHuge()
	}
}

func BenchmarkMakeVariable4(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = variable(small)
	}
}

func BenchmarkMakeVariable100(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkInt = variable(big)
	}
}

// 3. Boxing

func BenchmarkBoxBigStruct(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkAny = bigStruct
	}
}

func BenchmarkBoxConstant(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkAny = 42
	}
}

func BenchmarkBoxSmallValue(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkAny = smallValue
	}
}

func BenchmarkBoxLargeValue(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sinkAny = largeValue
	}
}

// 4. Pointer use

func BenchmarkProcessPointer(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		u := User{Name: "Ada", Age: 36}
		sinkInt = Process(&u)
	}
}

func BenchmarkStorePointer(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		u := User{Name: "Ada", Age: 36}
		store(&u)
	}
}
