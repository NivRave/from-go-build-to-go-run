// Command structs-and-methods demonstrates struct layout and padding, useful
// zero values, embedding, struct tags, and encapsulation with unexported
// fields and constructors.
//
// Slides: slides/013-structs-and-methods.pdf
//
// Run:
//
//	go run ./examples/013-structs-and-methods
//	go vet -tags vetdemo ./examples/013-structs-and-methods
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
	"unsafe"
)

func main() {
	section("1. Field order and padding (slide p02)")
	layout()

	section("2. Useful zero values (slide p03)")
	zeroValues()

	section("3. Embedding (slides p04, p06)")
	embedding()

	section("4. Struct tags: omitempty vs omitzero (slide p05)")
	structTags()

	section("5. Encapsulation and constructors (slides p07, p08)")
	encapsulation()
}

// --- 1. Layout ------------------------------------------------------------

// Padded puts an 8-byte field between two bools. Each field starts at a
// multiple of its type's alignment, so A is followed by 7 bytes of padding,
// and the struct is padded at the end to a multiple of its own alignment.
type Padded struct {
	A bool
	B int64
	C bool
}

// Optimized holds the same fields, largest first.
type Optimized struct {
	B int64
	A bool
	C bool
}

func layout() {
	var p Padded
	var o Optimized
	fmt.Printf("Padded    size %2d, align %d, offsets A=%d B=%d C=%d\n",
		unsafe.Sizeof(p), unsafe.Alignof(p), unsafe.Offsetof(p.A), unsafe.Offsetof(p.B), unsafe.Offsetof(p.C))
	fmt.Printf("Optimized size %2d, align %d, offsets B=%d A=%d C=%d\n",
		unsafe.Sizeof(o), unsafe.Alignof(o), unsafe.Offsetof(o.B), unsafe.Offsetof(o.A), unsafe.Offsetof(o.C))
}

// --- 2. Zero values -------------------------------------------------------

// Config needs no constructor: a zero sync.Mutex is unlocked and a nil
// slice can be appended to. The mutex is unexported so callers cannot lock
// the struct's internals, and Config is always used through a pointer,
// because copying it would copy the lock (see vetdemo.go).
type Config struct {
	mu    sync.Mutex
	Paths []string
}

func (c *Config) AddPath(p string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Paths = append(c.Paths, p)
}

func zeroValues() {
	var c Config // no NewConfig needed
	fmt.Printf("zero Config: Paths = %v, nil = %t\n", c.Paths, c.Paths == nil)
	c.AddPath("/etc/app")
	c.AddPath("/usr/local/etc/app")
	fmt.Println("after AddPath:", c.Paths)

	// The standard library follows the same rule.
	var buf bytes.Buffer
	buf.WriteString("bytes.Buffer works from its zero value too")
	fmt.Println(buf.String())
}

// --- 3. Embedding ---------------------------------------------------------

type User struct {
	ID   int
	Name string
}

// Greet uses u.Name. The slide p06 version uses a User without a Name field;
// this User has one.
func (u User) Greet() string { return "Hello, " + u.Name }

// Admin contains a User. User's fields and methods are promoted to Admin,
// but an Admin is not a User.
type Admin struct {
	User
	Level int
}

type Greeter interface {
	Greet() string
}

func welcome(g Greeter) string { return g.Greet() }

func describeUser(u User) string { return fmt.Sprintf("user #%d %s", u.ID, u.Name) }

func embedding() {
	a := Admin{User: User{ID: 1, Name: "Ada"}, Level: 9}

	fmt.Println("promoted field:  a.Name =", a.Name)
	fmt.Println("promoted method: a.Greet() =", a.Greet())

	// Admin satisfies Greeter through the promoted Greet method.
	fmt.Println("as a Greeter:   ", welcome(a))

	// But it cannot be used where a User is expected:
	//
	//	describeUser(a) // compile error: cannot use a (variable of struct type Admin) as User value in argument to describeUser
	//
	// Pass the embedded value explicitly instead.
	fmt.Println("a.User:         ", describeUser(a.User))
}

// --- 4. Struct tags -------------------------------------------------------

type Profile struct {
	ID      int       `json:"id" db:"user_id"`
	Email   string    `json:"email,omitempty"`   // "" is dropped: good
	Retries int       `json:"retries,omitempty"` // 0 is dropped, even if 0 was meaningful
	Created time.Time `json:"created,omitempty"` // never dropped: omitempty ignores structs
	Updated time.Time `json:"updated,omitzero"`  // dropped when Updated.IsZero() (Go 1.24)
}

func structTags() {
	b, err := json.Marshal(Profile{ID: 1})
	if err != nil {
		fmt.Println("marshal:", err)
		return
	}
	fmt.Println(string(b))

	// Encoders read tags through reflection. So can you.
	f, _ := reflect.TypeFor[Profile]().FieldByName("ID")
	fmt.Printf("field %s: json=%q db=%q\n", f.Name, f.Tag.Get("json"), f.Tag.Get("db"))
}

// --- 5. Encapsulation -----------------------------------------------------

// Account keeps its state unexported. Money is stored as integer cents,
// because float64 cannot represent most decimal amounts exactly.
type Account struct {
	owner string
	cents int64
}

// NewAccount is the gatekeeper: it is the only way to create an Account
// from another package with its fields set.
func NewAccount(owner string) *Account {
	return &Account{owner: owner}
}

// Deposit is the only way to change the balance, so it can enforce the rule
// that deposits are positive.
func (a *Account) Deposit(cents int64) error {
	if cents <= 0 {
		return fmt.Errorf("deposit must be positive, got %d", cents)
	}
	a.cents += cents
	return nil
}

func (a *Account) Balance() int64 { return a.cents }

// Database has no useful zero value: it is meaningless without a URL, so
// the constructor validates its input.
type Database struct {
	url string
}

func NewDatabase(url string) (*Database, error) {
	if url == "" {
		return nil, errors.New("url required")
	}
	return &Database{url: url}, nil
}

func encapsulation() {
	acc := NewAccount("ada")
	_ = acc.Deposit(1050)
	if err := acc.Deposit(-500); err != nil {
		fmt.Println("rejected:", err)
	}
	fmt.Printf("%s's balance: %d cents ($%d.%02d)\n", acc.owner, acc.Balance(), acc.Balance()/100, acc.Balance()%100)

	// 0.1 + 0.2 is not 0.3 in floating point; in cents it is exact.
	a, b := 0.1, 0.2
	fmt.Printf("float64: 0.1 + 0.2 = %.17f; cents: 10 + 20 = %d\n", a+b, 10+20)

	if _, err := NewDatabase(""); err != nil {
		fmt.Println(`NewDatabase(""):`, err)
	}
	db, err := NewDatabase("postgres://localhost/app")
	fmt.Printf("NewDatabase(url): url = %s, err = %v\n", db.url, err)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
