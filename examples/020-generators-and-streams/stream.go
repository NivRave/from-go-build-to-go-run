package main

import (
	"bufio"
	"io"
)

// Stream abstracts a source of values (slide p05). Unlike the slide version it
// has Err: bufio.Scanner and sql.Rows both separate "done" from "failed", and
// a stream API without Err makes a truncated read look like a clean finish.
type Stream interface {
	Next() bool
	Value() string
	Err() error
}

// LineStream streams lines from r, holding one line in memory at a time.
type LineStream struct {
	sc *bufio.Scanner
}

// NewLineStream reads lines up to maxLine bytes. bufio.Scanner's default limit
// is 64 KiB (bufio.MaxScanTokenSize); a longer line stops the scan with
// bufio.ErrTooLong, which is only visible through Err.
func NewLineStream(r io.Reader, maxLine int) *LineStream {
	sc := bufio.NewScanner(r)
	if maxLine > bufio.MaxScanTokenSize {
		sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLine)
	}
	return &LineStream{sc: sc}
}

func (s *LineStream) Next() bool    { return s.sc.Scan() }
func (s *LineStream) Value() string { return s.sc.Text() }
func (s *LineStream) Err() error    { return s.sc.Err() }

// CountLines consumes any Stream and reports how many values it produced.
// It always checks Err after the loop.
func CountLines(s Stream) (int, error) {
	n := 0
	for s.Next() {
		n++
	}
	return n, s.Err()
}
