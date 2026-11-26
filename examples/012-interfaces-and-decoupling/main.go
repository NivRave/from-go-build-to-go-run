// Command interfaces-and-decoupling demonstrates "accept interfaces, return
// structs": implicit satisfaction, io.Writer parameters, concrete return
// types, type assertions, and the pointer-receiver method-set trap.
//
// Slides: slides/012-interfaces-and-decoupling.pdf
//
// Run:
//
//	go run ./examples/012-interfaces-and-decoupling
//	go build -gcflags=-m -o bin/ ./examples/012-interfaces-and-decoupling
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	section("1. Implicit satisfaction (slide p02)")
	implicitSatisfaction()

	section("2. Accept interfaces: Save(w io.Writer) (slide p03)")
	acceptInterfaces()

	section("3. Return structs: OpenFile returns *File (slide p04)")
	returnStructs()

	section("4. Type switches and assertions (slide p07)")
	typeAssertions()

	section("5. The method-set trap (slide p08)")
	methodSetTrap()
}

// --- 1. Implicit satisfaction ---------------------------------------------

// Notifier is defined here, next to the code that consumes it, not next to
// the types that implement it. Neither EmailSender nor SMSSender mentions it.
type Notifier interface {
	Notify(msg string) string
}

type EmailSender struct{ Addr string }

func (e EmailSender) Notify(msg string) string { return "email to " + e.Addr + ": " + msg }

type SMSSender struct{ Phone string }

func (s SMSSender) Notify(msg string) string { return "sms to " + s.Phone + ": " + msg }

// alertAll needs only the Notify behaviour, so it asks for nothing more.
func alertAll(ns []Notifier, msg string) {
	for _, n := range ns {
		fmt.Println(n.Notify(msg))
	}
}

func implicitSatisfaction() {
	alertAll([]Notifier{
		EmailSender{Addr: "ops@example.com"},
		SMSSender{Phone: "+1-555-0100"},
	}, "disk 90% full")
}

// --- 2. Accept interfaces -------------------------------------------------

type User struct {
	ID   int
	Name string
}

// Save works with anything that can Write: a file, a network connection, a
// buffer, a hash, or a fake in a test. A `*os.File` parameter would allow
// only the first.
func Save(w io.Writer, u User) error {
	_, err := fmt.Fprintf(w, "user id=%d name=%s\n", u.ID, u.Name)
	return err
}

// countingWriter is a fake writer: it records what was written instead of
// writing it anywhere.
type countingWriter struct {
	writes int
	bytes  int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	c.bytes += len(p)
	return len(p), nil
}

func acceptInterfaces() {
	u := User{ID: 1, Name: "Ada"}

	fmt.Print("os.Stdout:     ")
	_ = Save(os.Stdout, u)

	var buf bytes.Buffer
	_ = Save(&buf, u)
	fmt.Printf("bytes.Buffer:  holds %q\n", buf.String())

	var fake countingWriter
	_ = Save(&fake, u)
	fmt.Printf("fake writer:   %d write, %d bytes\n", fake.writes, fake.bytes)
}

// --- 3. Return structs ----------------------------------------------------

// File is an in-memory stand-in for a real file, so the demo does not touch
// the file system. *File has more methods than io.Reader requires.
type File struct {
	path string
	r    *strings.Reader
}

// OpenFile returns the concrete type. Callers get every method of *File and
// can still pass it anywhere an io.Reader is accepted.
func OpenFile(path, contents string) *File {
	return &File{path: path, r: strings.NewReader(contents)}
}

func (f *File) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *File) Name() string               { return f.path }
func (f *File) Size() int64                { return f.r.Size() }

// OpenInterface is the slide's "Java way". The caller sees only Read:
//
//	r := OpenInterface("notes.txt", "...")
//	r.Name() // compile error: r.Name undefined (type io.Reader has no field or method Name)
func OpenInterface(path, contents string) io.Reader {
	return OpenFile(path, contents)
}

// countBytes accepts the smallest interface it needs.
func countBytes(r io.Reader) (int64, error) {
	return io.Copy(io.Discard, r)
}

func returnStructs() {
	f := OpenFile("notes.txt", "accept interfaces, return structs")
	fmt.Printf("Name() = %s, Size() = %d\n", f.Name(), f.Size())

	n, err := countBytes(f)
	fmt.Printf("countBytes(f) = %d, err = %v\n", n, err)

	// Getting Name back from an io.Reader needs a type assertion.
	r := OpenInterface("other.txt", "")
	if named, ok := r.(interface{ Name() string }); ok {
		fmt.Println("from io.Reader, after an assertion:", named.Name())
	}
}

// --- 4. Type switches and assertions --------------------------------------

func describe(i any) string {
	switch v := i.(type) {
	case int:
		return fmt.Sprint("Integer: ", v)
	case string:
		return fmt.Sprint("String: ", v)
	default:
		return fmt.Sprintf("Other: %T", v)
	}
}

func typeAssertions() {
	for _, v := range []any{42, "hello", 3.5} {
		fmt.Println(describe(v))
	}

	var i any = "hello"

	// Comma-ok never panics: a failed assertion gives the zero value and false.
	n, ok := i.(int)
	fmt.Printf("i.(int) with comma-ok: n = %d, ok = %t\n", n, ok)

	// A single-result assertion panics when it fails. That is fine when a
	// wrong type can only mean a bug in the program.
	fmt.Println("i.(int) without comma-ok:", uncheckedAssertion(i))
}

func uncheckedAssertion(i any) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint("recovered: ", r)
		}
	}()
	n := i.(int)
	return fmt.Sprint("got ", n)
}

// --- 5. The method-set trap -----------------------------------------------

type Shaper interface {
	Area() float64
}

type Square struct{ Side float64 }

// Area has a pointer receiver, so only *Square has it in its method set.
func (s *Square) Area() float64 { return s.Side * s.Side }

// The compliance check: a nil pointer of type *Square, assigned to the blank
// identifier. It costs nothing at run time and breaks the build if *Square
// ever stops satisfying Shaper.
var _ Shaper = (*Square)(nil)

// The value form does not compile:
//
//	var _ Shaper = Square{5}
//	// cannot use Square{…} (value of struct type Square) as Shaper value in variable declaration:
//	// Square does not implement Shaper (method Area has pointer receiver)

func methodSetTrap() {
	var s Shaper = &Square{Side: 5}
	fmt.Println("(&Square{5}).Area() via Shaper =", s.Area())

	// Calling the method directly on an addressable value still works: Go
	// takes the address for you. Only interface satisfaction is strict.
	sq := Square{Side: 3}
	fmt.Println("sq.Area() on an addressable value =", sq.Area())
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
