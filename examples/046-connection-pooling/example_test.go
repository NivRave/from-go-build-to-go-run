package main

// traceReuse is deterministic: the transport puts a connection back in the
// pool before the body's final Read returns io.EOF, so the next sequential
// request always finds it.
func Example_traceReuse() {
	traceReuse()
	// Output:
	// request 1: Reused=false WasIdle=false (first request: nothing to reuse)
	// request 2: Reused=true  WasIdle=true  (previous body read to EOF and closed)
	// request 3: Reused=true  WasIdle=true  (previous body read to EOF and closed; this one is closed unread)
	// request 4: Reused=false WasIdle=false (previous body closed WITHOUT reading it)
	// server accepted 2 connections for 4 requests
}
