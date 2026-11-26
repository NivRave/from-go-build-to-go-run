package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's expected output can't go stale.
//
// The expected output is encoding/json v1 behaviour. Under GOEXPERIMENT=jsonv2
// the v1 API is reimplemented on top of v2 but keeps v1 semantics, so these
// examples pass there too (checked with Go 1.26.4).

func Example_marshalUnmarshal() {
	marshalUnmarshal()
	// Output:
	// marshal:   {"id":1,"email":"gopher@example.com","is_admin":false}
	// unmarshal: {ID:1 Email:gopher@example.com IsAdmin:false}
	// uppercase keys still match: {ID:2 Email:LOUD@example.com IsAdmin:false}
}

func Example_streamDecode() {
	streamDecode()
	trailingData()
	// Output:
	// decoded user 1 <a@example.com>
	// decoded user 2 <b@example.com>
	// decoded user 3 <c@example.com>
	// Unmarshal:                 invalid character 'g' after top-level value
	// Decode:                    <nil>
	// decodeSingle:              unexpected data after the JSON value
	// decodeSingle, clean input: <nil>
}

func Example_massAssignment() {
	massAssignment()
	// Output:
	// User, strict:              err=<nil> IsAdmin=true  <- mass assignment
	// CreateUserRequest:         err=<nil> IsAdmin=false
	// CreateUserRequest, strict: err=strict decode failed: json: unknown field "is_admin"
}

func Example_rawMessage() {
	rawMessage()
	// Output:
	// user.created: {ID:7 Email:new@example.com}
	// payment.received: {AmountCents:1999 Currency:EUR}
	// error: unknown event type "user.deleted"
}

func Example_precision() {
	precision()
	// Output:
	// map[string]any: 9.007199254740992e+15 (float64) -> int64 9007199254740992
	// UseNumber:      9007199254740993 (json.Number) -> int64 9007199254740993, err=<nil>
	// int64 field:    9007199254740993
}
