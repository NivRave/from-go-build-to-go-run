// Command json demonstrates encoding/json: marshalling with tags, streaming
// a sequence of values with a Decoder, why DisallowUnknownFields doesn't stop
// mass assignment, delayed decoding with json.RawMessage, and the float64
// precision trap.
//
// Slides: slides/029-json.pdf
//
// Run:
//
//	go run ./examples/029-json
//	go test -bench=. -benchmem ./examples/029-json
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// User is the domain type from slide p02, plus the privileged field that
// slide p04 worries about.
type User struct {
	ID      int    `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

func main() {
	section("1. Marshal vs. Unmarshal (slide p02)")
	marshalUnmarshal()

	section("2. Decoding a stream of values (slide p03)")
	streamDecode()
	trailingData()

	section("3. DisallowUnknownFields vs. mass assignment (slide p04)")
	massAssignment()

	section("4. Delayed decoding with json.RawMessage (slide p05)")
	rawMessage()

	section("5. The float64 trap (slide p06)")
	precision()
}

func marshalUnmarshal() {
	u := User{ID: 1, Email: "gopher@example.com"}
	data, err := json.Marshal(u) // the slide drops this error; don't
	if err != nil {
		fmt.Println("marshal error:", err)
		return
	}
	fmt.Printf("marshal:   %s\n", data)

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		fmt.Println("unmarshal error:", err)
		return
	}
	fmt.Printf("unmarshal: %+v\n", decoded)

	// Tags don't make matching case-sensitive in encoding/json v1: an exact
	// match is tried first, then a case-insensitive one.
	var shouting User
	if err := json.Unmarshal([]byte(`{"ID":2,"EMAIL":"LOUD@example.com"}`), &shouting); err != nil {
		fmt.Println("unmarshal error:", err)
		return
	}
	fmt.Printf("uppercase keys still match: %+v\n", shouting)
}

// streamDecode reads newline-delimited JSON one value at a time. This, and not
// needing io.ReadAll, is what a Decoder buys you. Each Decode still reads one
// whole value into memory, so a single huge object is not processed
// incrementally.
func streamDecode() {
	ndjson := `{"id":1,"email":"a@example.com"}
{"id":2,"email":"b@example.com"}
{"id":3,"email":"c@example.com"}
`
	dec := json.NewDecoder(strings.NewReader(ndjson))
	for dec.More() {
		var u User
		if err := dec.Decode(&u); err != nil {
			fmt.Println("decode error:", err)
			return
		}
		fmt.Printf("decoded user %d <%s>\n", u.ID, u.Email)
	}
}

// trailingData shows that Unmarshal rejects junk after the value but a single
// Decode call accepts it. decodeSingle closes the gap.
func trailingData() {
	const input = `{"id":1} garbage`
	var u User
	fmt.Printf("%-26s %v\n", "Unmarshal:", json.Unmarshal([]byte(input), &u))
	fmt.Printf("%-26s %v\n", "Decode:", json.NewDecoder(strings.NewReader(input)).Decode(&u))
	fmt.Printf("%-26s %v\n", "decodeSingle:", decodeSingle(strings.NewReader(input), &u))
	fmt.Printf("%-26s %v\n", "decodeSingle, clean input:", decodeSingle(strings.NewReader(`{"id":1}`+"\n"), &u))
}

// decodeSingle decodes exactly one JSON value from r and fails if anything
// but whitespace follows it. The second Decode must return io.EOF.
//
// For an HTTP body, also wrap r with http.MaxBytesReader to bound memory, and
// reply with a generic message rather than err.Error().
func decodeSingle(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}

// StrictDecode is slide p04's function, generalised to any target.
func StrictDecode(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("strict decode failed: %w", err)
	}
	return nil
}

// CreateUserRequest is the input DTO: it contains only what a client may set.
// The handler copies it into a User, so IsAdmin can't come from the request.
type CreateUserRequest struct {
	Email string `json:"email"`
}

func massAssignment() {
	const payload = `{"email":"mallory@example.com","is_admin":true}`

	// is_admin is a *known* field of User, so DisallowUnknownFields lets it in.
	var u User
	err := StrictDecode(strings.NewReader(payload), &u)
	fmt.Printf("User, strict:              err=%v IsAdmin=%v  <- mass assignment\n", err, u.IsAdmin)

	// The DTO has no such field. Decoded leniently, is_admin is ignored...
	var req CreateUserRequest
	err = json.Unmarshal([]byte(payload), &req)
	created := User{ID: 42, Email: req.Email} // IsAdmin stays false
	fmt.Printf("CreateUserRequest:         err=%v IsAdmin=%v\n", err, created.IsAdmin)

	// ...and decoded strictly, it is rejected as an unknown field.
	err = StrictDecode(strings.NewReader(payload), &CreateUserRequest{})
	fmt.Printf("CreateUserRequest, strict: err=%v\n", err)
}

// Envelope is slide p05's wrapper: Payload stays raw bytes until Type says
// what it is. RawMessage is still scanned and copied, so "delayed" is not
// "free".
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// UserCreated and PaymentReceived are the two payload types.
type UserCreated struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type PaymentReceived struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

func rawMessage() {
	const events = `[
		{"type":"user.created","payload":{"id":7,"email":"new@example.com"}},
		{"type":"payment.received","payload":{"amount_cents":1999,"currency":"EUR"}},
		{"type":"user.deleted","payload":{"id":7}}
	]`
	var envs []Envelope
	if err := json.Unmarshal([]byte(events), &envs); err != nil {
		fmt.Println("decode error:", err)
		return
	}
	for _, e := range envs {
		msg, err := handle(e)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(msg)
	}
}

func handle(e Envelope) (string, error) {
	switch e.Type {
	case "user.created":
		var p UserCreated
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s: %+v", e.Type, p), nil
	case "payment.received":
		var p PaymentReceived
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s: %+v", e.Type, p), nil
	default:
		return "", fmt.Errorf("unknown event type %q", e.Type)
	}
}

// precision decodes an ID just above 2^53, the largest range in which a
// float64 represents every integer exactly.
func precision() {
	const doc = `{"id":9007199254740993}` // 2^53 + 1

	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		fmt.Println("error:", err)
		return
	}
	f := m["id"].(float64) // a JSON number in map[string]any is always float64
	fmt.Printf("map[string]any: %v (%T) -> int64 %d\n", f, m["id"], int64(f))

	dec := json.NewDecoder(strings.NewReader(doc))
	dec.UseNumber()
	var mn map[string]any
	if err := dec.Decode(&mn); err != nil {
		fmt.Println("error:", err)
		return
	}
	n, err := mn["id"].(json.Number).Int64() // UseNumber makes every number a json.Number
	fmt.Printf("UseNumber:      %v (%T) -> int64 %d, err=%v\n", mn["id"], mn["id"], n, err)

	var s struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(doc), &s); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("int64 field:    %d\n", s.ID)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
