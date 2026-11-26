// Command error-philosophy demonstrates errors as values: sentinels and
// custom error types that unwrap, behaviour checks with errors.As, guard
// clauses, the typed-nil gotcha, and where panic still belongs.
//
// Slides: slides/021-error-philosophy.pdf
//
// Run:
//
//	go run ./examples/021-error-philosophy
package main

import (
	"errors"
	"fmt"
	"time"
)

func main() {
	section("1. Sentinel errors and custom error types (slides p03-p04)")
	sentinelsAndTypes()

	section("2. Opaque errors: check behaviour, not type (slide p05)")
	behaviourChecks()

	section("3. Guard clauses keep the happy path left (slide p08)")
	guardClauses()

	section("4. The typed-nil gotcha (slide p03)")
	typedNil()

	section("5. Panic is not an error (slide p07)")
	panicVsError()
}

// ---- 1. sentinels and custom types --------------------------------------------

// ErrNotFound is a sentinel: a fixed value callers compare against with
// errors.Is. Sentinels are normally created with errors.New.
var ErrNotFound = errors.New("not found")

// QueryError carries metadata (which query failed) and the underlying cause.
type QueryError struct {
	Query string
	Err   error
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query %q failed: %v", e.Query, e.Err)
}

// Unwrap is what the slide's version is missing. Without it errors.Is and
// errors.As stop at QueryError and never see e.Err.
func (e *QueryError) Unwrap() error { return e.Err }

// FindUser pretends to query a database that has no such user.
func FindUser(id int) error {
	return &QueryError{Query: fmt.Sprintf("SELECT * FROM users WHERE id = %d", id), Err: ErrNotFound}
}

func sentinelsAndTypes() {
	err := FindUser(42)
	fmt.Println("err:", err)
	fmt.Println("errors.Is(err, ErrNotFound):", errors.Is(err, ErrNotFound)) // the "what"

	var qe *QueryError
	if errors.As(err, &qe) { // the "where": metadata from the typed error
		fmt.Println("failed query:", qe.Query)
	}
}

// ---- 2. behaviour checks --------------------------------------------------------

// retryable is unexported on purpose: callers never import it. Any error type
// in any package that has a Retryable() bool method satisfies it. It replaces
// the slide's Temporary(), which net.Error deprecated in Go 1.18 because
// "temporary" was never well defined.
type retryable interface {
	Retryable() bool
}

// RateLimitError is one such type. A caller can retry it without knowing the
// type exists.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
}

func (e *RateLimitError) Retryable() bool { return true }

// IsRetryable walks the whole error chain. errors.As accepts a pointer to an
// interface type and reports whether any error in the chain implements it.
func IsRetryable(err error) bool {
	var r retryable
	return errors.As(err, &r) && r.Retryable()
}

// isRetryableBare is the slide's version: a plain type assertion only looks
// at the outermost error, so any fmt.Errorf("...: %w") wrapper hides the
// behaviour.
func isRetryableBare(err error) bool {
	r, ok := err.(retryable)
	return ok && r.Retryable()
}

func behaviourChecks() {
	direct := &RateLimitError{RetryAfter: 2 * time.Second}
	wrapped := fmt.Errorf("fetch quotes: %w", direct)

	fmt.Println("wrapped:", wrapped)
	fmt.Printf("%-22s %-8s %s\n", "", "direct", "wrapped")
	fmt.Printf("%-22s %-8v %v\n", "type assertion:", isRetryableBare(direct), isRetryableBare(wrapped))
	fmt.Printf("%-22s %-8v %v\n", "errors.As (interface):", IsRetryable(direct), IsRetryable(wrapped))
	fmt.Println("IsRetryable(ErrNotFound):", IsRetryable(ErrNotFound))
}

// ---- 3. guard clauses -------------------------------------------------------------

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password")
	errConnRefused     = errors.New("connection refused")
)

type User struct {
	Email    string
	Password string
}

// memDB is the fake database behind the slide's db.Save.
type memDB struct {
	users []User
	down  bool // when true, Save fails like an unreachable database
}

func (d *memDB) Save(u User) error {
	if d.down {
		return errConnRefused
	}
	d.users = append(d.users, u)
	return nil
}

var db = &memDB{}

// RegisterUser is the guard-clause version from slide p08: each failure
// returns early, and reaching the bottom means success.
func RegisterUser(u User) error {
	if u.Email == "" {
		return ErrInvalidEmail
	}

	if u.Password == "" {
		return ErrInvalidPassword
	}

	if err := db.Save(u); err != nil {
		return fmt.Errorf("database save: %w", err)
	}

	return nil
}

func guardClauses() {
	db = &memDB{} // fresh fake for every run, so the demo is repeatable

	fmt.Println("empty email:   ", RegisterUser(User{Password: "s3cret"}))
	fmt.Println("empty password:", RegisterUser(User{Email: "ada@example.com"}))

	db.down = true
	fmt.Println("db failure:    ", RegisterUser(User{Email: "ada@example.com", Password: "s3cret"}))
	db.down = false

	fmt.Println("success:       ", RegisterUser(User{Email: "ada@example.com", Password: "s3cret"}))
	fmt.Println("users saved:   ", len(db.users))
}

// ---- 4. typed nil -------------------------------------------------------------------

// validateQueryBad returns a nil *QueryError as an error. An interface value
// is nil only when both its type and its value are nil; here the type is
// *QueryError, so the caller's err != nil check is true.
//
//nolint:staticcheck // intentional: returns a typed nil to show the gotcha
func validateQueryBad(q string) error {
	var qe *QueryError
	if q == "" {
		qe = &QueryError{Query: q, Err: errors.New("empty query")}
	}
	return qe
}

// validateQuery is the fix: return the literal nil on success.
func validateQuery(q string) error {
	if q == "" {
		return &QueryError{Query: q, Err: errors.New("empty query")}
	}
	return nil
}

// typedNil compares both versions. staticcheck catches the bad one (SA4023:
// "this comparison is always true"), which is why it is silenced here.
//
//nolint:staticcheck // intentional: SA4023 flags the always-true comparison, which is the gotcha being shown
func typedNil() {
	err := validateQueryBad("SELECT 1")
	fmt.Printf("bad:   err != nil is %v (dynamic type %T, dynamic value nil)\n", err != nil, err)

	err = validateQuery("SELECT 1")
	fmt.Printf("fixed: err != nil is %v\n", err != nil)
}

// ---- 5. panic vs error -----------------------------------------------------------------

// mustLoadConfig runs during boot. Without the required setting the program
// cannot do anything useful, so it panics: this is a programming or
// deployment mistake, not a condition to handle.
func mustLoadConfig(env map[string]string) string {
	dsn, ok := env["DATABASE_URL"]
	if !ok {
		panic("boot: DATABASE_URL is required")
	}
	return dsn
}

// boot recovers only so the demo can continue; a real main would let the
// panic crash the process at startup, loudly.
func boot(env map[string]string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("boot aborted by panic:", r)
		}
	}()
	fmt.Println("boot ok, dsn =", mustLoadConfig(env))
}

// handleRequest runs while serving. A missing user is an expected outcome,
// so it is an error that the caller reports and moves past.
func handleRequest(id int) error {
	if err := FindUser(id); err != nil {
		return fmt.Errorf("handle request for user %d: %w", id, err)
	}
	return nil
}

func panicVsError() {
	boot(map[string]string{})
	boot(map[string]string{"DATABASE_URL": "postgres://localhost/app"})

	for _, id := range []int{7, 8} {
		if err := handleRequest(id); err != nil {
			fmt.Printf("request for user %d: returned an error (not found: %v), still serving\n",
				id, errors.Is(err, ErrNotFound))
		}
	}
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
