package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

// Sensor is slide p02's type: with omitempty, a reading of 0 disappears.
type Sensor struct {
	Temperature int `json:"temp,omitempty"`
}

// NullableSensor is slide p03's fix: a nil pointer is "missing", a pointer to
// 0 is "zero degrees".
type NullableSensor struct {
	Temperature *int `json:"temp,omitempty"`
}

// Price treats "no currency" as zero, so a price of 0 EUR is still sent.
// omitzero (Go 1.24) calls IsZero when a type has one.
type Price struct {
	Cents    int64  `json:"cents"`
	Currency string `json:"currency"`
}

// IsZero reports whether the price is unset.
func (p Price) IsZero() bool { return p.Currency == "" }

// Event compares omitempty and omitzero on the same kinds of field.
type Event struct {
	Name       string    `json:"name"`
	At         time.Time `json:"at,omitzero"`
	AtLegacy   time.Time `json:"at_legacy,omitempty"` // structs are never "empty"
	Price      Price     `json:"price,omitzero"`
	PriceEmpty Price     `json:"price_empty,omitempty"`
}

// UnixTime encodes a time as a JSON number of seconds since 1970 (slide p04).
// Being a defined type, it has none of time.Time's methods; convert back with
// time.Time(u).
type UnixTime time.Time

// MarshalJSON emits a JSON number such as 1767225600, not a string.
// strconv.AppendInt avoids fmt.
func (u UnixTime) MarshalJSON() ([]byte, error) {
	return strconv.AppendInt(nil, time.Time(u).Unix(), 10), nil
}

// UnmarshalJSON parses a JSON number (slide p05). Unlike the slide, it treats
// null as "no value" and leaves u unchanged, which is the convention the
// standard library follows.
func (u *UnixTime) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	var timestamp int64
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}
	*u = UnixTime(time.Unix(timestamp, 0).UTC())
	return nil
}

// slideUnixTime is slide p05's UnmarshalJSON, kept to show the null bug:
// json.Unmarshal of null into an int64 is a silent no-op, so the timestamp
// stays 0 and the time becomes 1970-01-01.
type slideUnixTime time.Time

func (u *slideUnixTime) UnmarshalJSON(b []byte) error {
	var timestamp int64
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}
	*u = slideUnixTime(time.Unix(timestamp, 0).UTC())
	return nil
}

// Legacy decodes an API that quotes its numbers and booleans. The ",string"
// option handles that without a custom Unmarshaler.
type Legacy struct {
	Count  int  `json:"count,string"`
	Active bool `json:"active,string"`
}

// User replaces LastSeen with a Unix timestamp using the alias pattern
// (slide p06). The value receiver makes it work for User and *User alike.
type User struct {
	Name     string    `json:"name"`
	LastSeen time.Time `json:"last_seen"`
}

// MarshalJSON encodes LastSeen as seconds. Alias has User's fields but none of
// its methods, so json.Marshal doesn't call this method again and recurse
// forever. (*Alias)(&u) is a pointer conversion: nothing is copied.
func (u User) MarshalJSON() ([]byte, error) {
	type Alias User
	return json.Marshal(&struct {
		LastSeen int64 `json:"last_seen"` // shallower, so it hides Alias.LastSeen
		*Alias
	}{
		LastSeen: u.LastSeen.Unix(),
		Alias:    (*Alias)(&u),
	})
}

// PtrUser is the same type with slide p06's pointer receiver. json.Marshal
// only finds that method when it has a pointer (or an addressable value), so
// marshalling a PtrUser value silently falls back to the default encoding.
type PtrUser struct {
	Name     string    `json:"name"`
	LastSeen time.Time `json:"last_seen"`
}

func (u *PtrUser) MarshalJSON() ([]byte, error) {
	type Alias PtrUser
	return json.Marshal(&struct {
		LastSeen int64 `json:"last_seen"`
		*Alias
	}{
		LastSeen: u.LastSeen.Unix(),
		Alias:    (*Alias)(u),
	})
}
