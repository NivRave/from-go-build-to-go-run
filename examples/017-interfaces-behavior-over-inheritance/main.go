// Command interfaces-behavior-over-inheritance demonstrates Go's implicit
// interfaces: discovering an interface from concrete types, accepting
// io.Writer instead of a concrete file, composing small interfaces, the
// compile-time satisfaction check, and generics as the type-safe alternative
// to any.
//
// Slides: slides/017-interfaces-behavior-over-inheritance.pdf
//
// Run:
//
//	go run ./examples/017-interfaces-behavior-over-inheritance
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	section("1. Discovering an interface (slides p02, p05)")
	discoverInterface()

	section("2. Accept interfaces: one SaveData, four destinations (slides p03-p04)")
	saveEverywhere()

	section("3. Composition via embedding (slide p08)")
	composition()

	section("4. Compile-time satisfaction check (slide p07)")
	staticCheck()

	section("5. any vs generics (slide p06)")
	anyVsGenerics()
}

// ---- 1. Discovering an interface -------------------------------------------

// Step 1 (slide p05): write the concrete type you need today.

// ConsoleWriter prints log lines to standard output (slide p02).
type ConsoleWriter struct{}

func (cw ConsoleWriter) Log(m string) {
	fmt.Println(m)
}

// Step 2: a second implementation shows up, here an in-memory logger that a
// test could inspect.
type memLogger struct {
	lines []string
}

func (l *memLogger) Log(m string) {
	l.lines = append(l.lines, m)
}

// Step 3: only now extract the behaviour both share. Neither type mentions
// Logger; they satisfy it simply by having the method.
type Logger interface {
	Log(message string)
}

// Greet depends on the behaviour, not on either concrete type.
func Greet(l Logger, name string) {
	l.Log("hello, " + name)
}

func discoverInterface() {
	mem := &memLogger{}
	for _, l := range []Logger{ConsoleWriter{}, mem} {
		Greet(l, "gopher")
	}
	fmt.Printf("memLogger captured %q\n", mem.lines)
}

// ---- 2. Accept interfaces, return structs ----------------------------------

// SaveData accepts any io.Writer, so the same function writes to a terminal,
// a buffer, a file or a socket. Unlike the slide's signature it returns the
// write error: an io.Writer can fail, and the caller should hear about it.
func SaveData(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}

func saveEverywhere() {
	data := []byte("payload\n")

	// os.Stdout is an *os.File.
	fmt.Print("os.Stdout     -> ")
	must(SaveData(os.Stdout, data))

	// *bytes.Buffer: an in-memory destination, ideal for tests.
	var buf bytes.Buffer
	must(SaveData(&buf, data))
	fmt.Printf("%-13T -> %q\n", &buf, buf.String())

	// A temporary file on disk, removed afterwards.
	f, err := os.CreateTemp("", "017-savedata-*.txt")
	must(err)
	defer os.Remove(f.Name())
	must(SaveData(f, data))
	must(f.Close())
	onDisk, err := os.ReadFile(f.Name())
	must(err)
	fmt.Printf("%-13T -> %q (temp file)\n", f, onDisk)

	// net.Pipe returns two ends of an in-process net.Conn. net.Conn is
	// already an interface; the slide's *net.Conn would be a pointer to an
	// interface, which has no methods at all:
	//
	//	var _ io.Writer = (*net.Conn)(nil)
	//	// compile error: cannot use (*net.Conn)(nil) (value of type *net.Conn) as io.Writer
	//	// value in variable declaration: *net.Conn does not implement io.Writer
	//	// (type *net.Conn is pointer to interface, not interface)
	client, server := net.Pipe()
	go func() {
		// A pipe is synchronous: Write blocks until the other end reads,
		// so the writer runs in its own goroutine.
		must(SaveData(client, data))
		must(client.Close())
	}()
	received, err := io.ReadAll(server)
	must(err)
	must(server.Close())
	fmt.Printf("%-13s -> %q (net.Pipe)\n", "net.Conn", received)
}

// ---- 3. Composition ----------------------------------------------------------

// ReadWriter is built from two one-method interfaces (slide p08). The
// standard library already has io.ReadWriter; this is the same declaration.
type ReadWriter interface {
	io.Reader
	io.Writer
}

// countingWriter embeds an io.Writer, so it is an io.Writer itself, and
// overrides Write to count the bytes passing through.
type countingWriter struct {
	io.Writer
	n int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.Writer.Write(p) // delegate to the embedded writer
	c.n += n
	return n, err
}

func composition() {
	// *bytes.Buffer has both Read and Write, so it satisfies ReadWriter
	// without ever naming it.
	var rw ReadWriter = &bytes.Buffer{}
	fmt.Fprint(rw, "composed")
	got, _ := io.ReadAll(rw)
	fmt.Printf("ReadWriter round trip: %q\n", got)

	var out bytes.Buffer
	cw := &countingWriter{Writer: &out}
	fmt.Fprintf(cw, "%s, %s!", "hello", "composition")
	fmt.Fprint(cw, " more")
	fmt.Printf("countingWriter saw %d bytes: %q\n", cw.n, out.String())
}

// ---- 4. Compile-time satisfaction check ---------------------------------------

// These lines cost nothing at run time. If someone renames memLogger.Log or
// changes its signature, the build breaks here, before any test runs. The
// idiom works in any package, not only internal ones.
var (
	_ Logger    = (*memLogger)(nil)
	_ Logger    = ConsoleWriter{}
	_ io.Writer = (*countingWriter)(nil)
)

// The slide p07 version fails because MyStruct has no DoWork method:
//
//	type MyInterface interface{ DoWork() }
//	type MyStruct struct{}
//	var _ MyInterface = (*MyStruct)(nil)
//	// compile error: cannot use (*MyStruct)(nil) (value of type *MyStruct) as MyInterface
//	// value in variable declaration: *MyStruct does not implement MyInterface
//	// (missing method DoWork)
//
// Adding `func (*MyStruct) DoWork() {}` makes it compile.

func staticCheck() {
	fmt.Println("*memLogger, ConsoleWriter and *countingWriter were checked by the compiler")
	// Note that memLogger (the value type) does not satisfy Logger: Log has
	// a pointer receiver, so only *memLogger has it in its method set.
	var v any = memLogger{}
	_, ok := v.(Logger)
	fmt.Println("memLogger (value) is a Logger:", ok)
	v = &memLogger{}
	_, ok = v.(Logger)
	fmt.Println("*memLogger is a Logger:      ", ok)
}

// ---- 5. any vs generics -------------------------------------------------------

// SumAny accepts anything, so the compiler can't help: every element needs a
// type switch, and a bad element is only discovered at run time.
func SumAny(xs []any) (float64, error) {
	var total float64
	for i, x := range xs {
		switch v := x.(type) {
		case int:
			total += float64(v)
		case float64:
			total += v
		default:
			return 0, fmt.Errorf("element %d: unsupported type %T", i, x)
		}
	}
	return total, nil
}

// Sum states its constraint in the signature (generics, Go 1.18), so a bad
// call doesn't compile:
//
//	Sum([]string{"a"})
//	// compile error: string does not satisfy int | float64 (string missing in int | float64)
func Sum[T int | float64](xs []T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

func anyVsGenerics() {
	fmt.Println("Sum[int]:    ", Sum([]int{1, 2, 3}))
	fmt.Println("Sum[float64]:", Sum([]float64{1.5, 3}))

	total, err := SumAny([]any{1, 2.5, 4})
	fmt.Println("SumAny:      ", total, err)
	total, err = SumAny([]any{1, 2.5, "4"})
	fmt.Println("SumAny:      ", total, err)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
