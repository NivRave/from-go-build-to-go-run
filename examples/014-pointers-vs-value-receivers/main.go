// Command pointers-vs-value-receivers demonstrates what value and pointer
// receivers actually do: copies vs shared state, method sets, shallow
// copies, nil receivers, and why a value receiver is not "thread-safe".
//
// Slides: slides/014-pointers-vs-value-receivers.pdf
//
// Run:
//
//	go run ./examples/014-pointers-vs-value-receivers
//	go run -race ./examples/014-pointers-vs-value-receivers -race-demo
//	go test -run='^$' -bench=. -benchmem ./examples/014-pointers-vs-value-receivers
package main

import (
	"flag"
	"fmt"
	"strings"
	"sync"
)

func main() {
	raceDemo := flag.Bool("race-demo", false, "run the intentionally racy value-receiver demo (use with go run -race)")
	flag.Parse()

	if *raceDemo {
		valueReceiverRace()
		return
	}

	section("1. Value receiver copies, pointer receiver shares (slides p02, p03)")
	receivers()

	section("2. Method sets and interfaces (slide p04)")
	methodSets()

	section("3. Value receivers make shallow copies (slides p02, p08)")
	shallowCopy()

	section("4. Nil receivers and the typed-nil gotcha (slide p07)")
	nilReceivers()
}

// --- 1. Receivers -----------------------------------------------------------

type Counter struct {
	count int
}

// Increment works on a copy: the caller's Counter is unchanged.
func (c Counter) Increment() {
	c.count++
	fmt.Printf("Inside method (copy): %d\n", c.count)
}

// SharedCounter is the same thing with a pointer receiver. It is a separate
// type so that neither type mixes receiver kinds (slide p06).
type SharedCounter struct {
	count int
}

func (c *SharedCounter) Increment() {
	c.count++
	fmt.Printf("Inside method (pointer): %d\n", c.count)
}

func receivers() {
	c := Counter{count: 10}
	c.Increment()
	fmt.Printf("Original caller: %d\n", c.count)

	s := SharedCounter{count: 10}
	s.Increment() // Go takes &s automatically
	fmt.Printf("Original caller: %d\n", s.count)
}

// --- 2. Method sets ---------------------------------------------------------

type User struct {
	Name string
}

// All of User's methods use pointer receivers, as slide p06 recommends once
// one of them needs to.
func (u *User) UpdateName(newName string) {
	u.Name = newName
}

// GetName handles a nil receiver, giving a default for optional data.
func (u *User) GetName() string {
	if u == nil {
		return "Guest"
	}
	return u.Name
}

type Namer interface {
	UpdateName(string)
}

// Only *User has UpdateName in its method set.
var _ Namer = &User{}

// The value form does not compile:
//
//	var _ Namer = User{}
//	// cannot use User{} (value of struct type User) as Namer value in variable declaration:
//	// User does not implement Namer (method UpdateName has pointer receiver)

func rename(n Namer, name string) { n.UpdateName(name) }

func methodSets() {
	// A variable is addressable, so u.UpdateName means (&u).UpdateName.
	u := User{Name: "Ada"}
	u.UpdateName("Grace")
	fmt.Println("u.UpdateName on a variable:", u.Name)

	// Through an interface it has to be a pointer.
	rename(&u, "Linus")
	fmt.Println("rename(&u, ...) via Namer: ", u.Name)

	// A composite literal is not addressable, so this does not compile:
	//
	//	User{}.UpdateName("x") // cannot call pointer method UpdateName on User
}

// --- 3. Shallow copies ------------------------------------------------------

type Playlist struct {
	Name  string
	Songs []string
}

// Shout has a value receiver, so p is a copy. But the copy's Songs slice
// header points at the same backing array as the caller's, so writing an
// element changes the caller's data. Only Name, a direct field, is isolated.
func (p Playlist) Shout() {
	p.Name = strings.ToUpper(p.Name)
	p.Songs[0] = strings.ToUpper(p.Songs[0])
	fmt.Printf("inside Shout:  %s %v\n", p.Name, p.Songs)
}

func shallowCopy() {
	p := Playlist{Name: "mix", Songs: []string{"intro", "outro"}}
	p.Shout()
	fmt.Printf("caller after:  %s %v\n", p.Name, p.Songs)
}

// --- 4. Nil receivers -------------------------------------------------------

type NameGetter interface {
	GetName() string
}

// findUser returns a *User inside a NameGetter. When nothing is found, the
// interface still holds a type (*User), so it is not nil.
func findUser(name string) NameGetter { //nolint:staticcheck // intentional: SA4023 notes this never returns a nil interface; that is the gotcha
	var u *User
	if name != "" {
		u = &User{Name: name}
	}
	return u
}

// findUserFixed returns a nil interface when nothing is found.
func findUserFixed(name string) NameGetter {
	if name == "" {
		return nil
	}
	return &User{Name: name}
}

//nolint:staticcheck // intentional: SA4023 flags the typed-nil comparison in here, which is the gotcha being shown
func nilReceivers() {
	var u *User
	fmt.Println("(*User)(nil).GetName() =", u.GetName())

	g := findUser("")
	isNil := g == nil
	fmt.Printf("findUser(\"\") == nil:      %t (dynamic type %T, GetName %q)\n", isNil, g, g.GetName())

	fmt.Printf("findUserFixed(\"\") == nil: %t\n", findUserFixed("") == nil)
}

// --- 5. The "thread-safety bonus" is a data race ----------------------------

type Settings struct {
	Timeout int
	Retries int
}

// Describe has a value receiver. Calling it copies the whole Settings, and
// that copy is an unsynchronised read of the caller's struct.
func (s Settings) Describe() string {
	return fmt.Sprintf("timeout=%d retries=%d", s.Timeout, s.Retries)
}

// valueReceiverRace writes a Settings in one goroutine while another calls
// a value-receiver method on it. The race detector reports the copy made for
// the receiver. It only runs with -race-demo, never from a test.
func valueReceiverRace() {
	cfg := Settings{Timeout: 30, Retries: 3}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 1000 {
			cfg.Timeout = i
		}
	})
	wg.Go(func() {
		for range 1000 {
			_ = cfg.Describe()
		}
	})
	wg.Wait()
	fmt.Println("done; run with go run -race to see the report")
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
