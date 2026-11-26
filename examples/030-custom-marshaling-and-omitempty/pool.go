package main

import (
	"bytes"
	"encoding/json"
	"sync"
)

// bufferPool and MarshalWithPool are slide p08's optimisation. The benchmarks
// in bench_test.go show why it gains little: json.Marshal already keeps its
// internal encode buffers in a sync.Pool and returns one copied slice, which
// is exactly what this function re-implements, plus an Encoder allocation.
var bufferPool = sync.Pool{
	New: func() any {
		// Create a new buffer with a reasonable initial capacity.
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// maxPooledBuffer stops one huge document from pinning a huge buffer in the
// pool forever. The slide's version always calls Put.
const maxPooledBuffer = 64 << 10

// MarshalWithPool encodes v like json.Marshal, except that the result ends in
// a newline, because Encoder.Encode always appends one.
func MarshalWithPool(v any) ([]byte, error) {
	buf := bufferPool.Get().(*bytes.Buffer) // the pool only ever holds *bytes.Buffer
	buf.Reset()                             // Crucial: clear old data but keep capacity
	defer func() {
		if buf.Cap() <= maxPooledBuffer {
			bufferPool.Put(buf)
		}
	}()

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, err
	}

	// Copy the bytes out: the buffer goes back to the pool.
	res := make([]byte, buf.Len())
	copy(res, buf.Bytes())
	return res, nil
}
