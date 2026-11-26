package main

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"
)

// Three ways to produce JSON for the same value (slide p08). Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/030-custom-marshaling-and-omitempty
//
// json.Marshal already pools its internal buffers, so MarshalWithPool saves
// little or nothing. Encoding straight to the destination writer is the
// version that skips the intermediate []byte.
type benchOrder struct {
	ID       int64     `json:"id"`
	Customer string    `json:"customer"`
	Items    []string  `json:"items"`
	Total    float64   `json:"total"`
	Paid     bool      `json:"paid"`
	Created  time.Time `json:"created"`
}

var order = benchOrder{
	ID:       12345,
	Customer: "gopher@example.com",
	Items:    []string{"keyboard", "mouse", "monitor", "cable"},
	Total:    499.95,
	Paid:     true,
	Created:  instant,
}

func BenchmarkMarshal(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(order); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalWithPool(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := MarshalWithPool(order); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncoderToDiscard(b *testing.B) {
	b.ReportAllocs()
	enc := json.NewEncoder(io.Discard) // in a server: json.NewEncoder(w)
	for b.Loop() {
		if err := enc.Encode(order); err != nil {
			b.Fatal(err)
		}
	}
}

// TestTrailingNewline pins down the one observable difference: an Encoder
// terminates each value with '\n' and json.Marshal does not.
func TestTrailingNewline(t *testing.T) {
	t.Parallel()
	plain, err := json.Marshal(order)
	if err != nil {
		t.Fatal(err)
	}
	pooled, err := MarshalWithPool(order)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasSuffix(plain, []byte("\n")) {
		t.Errorf("json.Marshal output ends with a newline: %q", plain)
	}
	if !bytes.HasSuffix(pooled, []byte("\n")) {
		t.Errorf("MarshalWithPool output has no trailing newline: %q", pooled)
	}
	if !bytes.Equal(append(plain, '\n'), pooled) {
		t.Errorf("outputs differ beyond the newline:\n%s\n%s", plain, pooled)
	}
}
