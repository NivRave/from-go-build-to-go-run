package main

import (
	"errors"
	"testing"
)

// failingWriter is a fake that fails every write, so the test can check that
// Save reports the error instead of swallowing it. Because Save accepts an
// io.Writer, no real file or disk-full condition is needed.
type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestSave(t *testing.T) {
	errDiskFull := errors.New("disk full")

	tests := []struct {
		name      string
		user      User
		wantBytes int
		failWith  error
	}{
		{name: "writes one line", user: User{ID: 7, Name: "Grace"}, wantBytes: len("user id=7 name=Grace\n")},
		{name: "reports write errors", user: User{ID: 1, Name: "Ada"}, failWith: errDiskFull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.failWith != nil {
				err := Save(failingWriter{err: tt.failWith}, tt.user)
				if !errors.Is(err, tt.failWith) {
					t.Fatalf("Save error = %v, want %v", err, tt.failWith)
				}
				return
			}
			var w countingWriter
			if err := Save(&w, tt.user); err != nil {
				t.Fatalf("Save: %v", err)
			}
			if w.writes != 1 || w.bytes != tt.wantBytes {
				t.Errorf("got %d writes, %d bytes; want 1 write, %d bytes", w.writes, w.bytes, tt.wantBytes)
			}
		})
	}
}
