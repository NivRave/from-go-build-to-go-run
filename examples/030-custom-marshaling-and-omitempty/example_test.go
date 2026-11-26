package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment, so the README's expected output can't go stale.

func Example_omitEmpty() {
	omitEmpty()
	// Output:
	// Sensor{Temperature: 0}       {}
	// NullableSensor{new(0)}       {"temp":0}
	// NullableSensor{nil}          {}
	// decode {}             Temperature = nil
	// decode {"temp":null}  Temperature = nil
	// decode {"temp":0}     Temperature = 0
}

func Example_omitZero() {
	omitZero()
	// Output:
	// Event, nothing set:          {"name":"unset","at_legacy":"0001-01-01T00:00:00Z","price_empty":{"cents":0,"currency":""}}
	// Event, all set (0 EUR):      {"name":"free","at":"2026-01-01T00:00:00Z","at_legacy":"2026-01-01T00:00:00Z","price":{"cents":0,"currency":"EUR"},"price_empty":{"cents":0,"currency":"EUR"}}
}

func Example_unixTime() {
	unixTime()
	// Output:
	// marshal:                     {"at":1767225600}
	// round trip equal:            true
	// null, this UnmarshalJSON:    2026-01-01T00:00:00Z
	// null, slide p05 version:     1970-01-01T00:00:00Z
	// ",string" in:                {Count:42 Active:true}
	// ",string" out:               {"count":"42","active":"true"}
}

func Example_aliasPattern() {
	aliasPattern()
	// Output:
	// value receiver, Marshal(u):  {"last_seen":1767225600,"name":"gopher"}
	// value receiver, Marshal(&u): {"last_seen":1767225600,"name":"gopher"}
	// ptr receiver, Marshal(u):    {"name":"gopher","last_seen":"2026-01-01T00:00:00Z"}
	// ptr receiver, Marshal(&u):   {"last_seen":1767225600,"name":"gopher"}
}

func Example_timeZones() {
	timeZones()
	// Output:
	// in EST:                      "2025-12-31T19:00:00-05:00"
	// in UTC:                      "2026-01-01T00:00:00Z"
	// same instant:                true
	// with 1.5 s:                  "2026-01-01T00:00:01.5Z"
}
