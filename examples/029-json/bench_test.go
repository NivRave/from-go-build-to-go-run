package main

import (
	"encoding/json"
	"testing"
)

// Decode the same document into a struct and into map[string]any (slide p07).
// Run with:
//
//	go test -run='^$' -bench=. -benchmem ./examples/029-json
//
// and again with GOEXPERIMENT=jsonv2 to compare the experimental
// implementation (Go 1.25+), which the v1 API uses under that experiment.
var benchDoc = []byte(`{
	"id": 12345,
	"email": "gopher@example.com",
	"is_admin": false,
	"name": "Gopher",
	"tags": ["go", "json", "benchmarks"],
	"address": {"city": "Tel Aviv", "zip": "6100000"}
}`)

type benchUser struct {
	ID      int      `json:"id"`
	Email   string   `json:"email"`
	IsAdmin bool     `json:"is_admin"`
	Name    string   `json:"name"`
	Tags    []string `json:"tags"`
	Address struct {
		City string `json:"city"`
		Zip  string `json:"zip"`
	} `json:"address"`
}

func BenchmarkDecodeStruct(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var u benchUser
		if err := json.Unmarshal(benchDoc, &u); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMap(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var m map[string]any
		if err := json.Unmarshal(benchDoc, &m); err != nil {
			b.Fatal(err)
		}
	}
}
