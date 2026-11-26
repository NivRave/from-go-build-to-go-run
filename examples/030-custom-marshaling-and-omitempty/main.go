// Command custom-marshaling-and-omitempty demonstrates when "zero" means
// "missing" in encoding/json: omitempty versus pointers and Go 1.24's
// omitzero, custom Marshaler and Unmarshaler types, the alias pattern and its
// pointer-receiver gotcha, and time zones.
//
// Slides: slides/030-custom-marshaling-and-omitempty.pdf
//
// Run:
//
//	go run ./examples/030-custom-marshaling-and-omitempty
//	go test -bench=. -benchmem ./examples/030-custom-marshaling-and-omitempty
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// instant is the fixed time every demo uses, so the output is deterministic.
var instant = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	section("1. The zero-value collision and the pointer fix (slides p02-p03)")
	omitEmpty()

	section("2. omitzero (Go 1.24) vs. omitempty")
	omitZero()

	section("3. Custom Marshaler and Unmarshaler (slides p04-p05)")
	unixTime()

	section("4. The alias pattern and the pointer-receiver gotcha (slide p06)")
	aliasPattern()

	section("5. time.Time and time zones (slide p07)")
	timeZones()
}

// show marshals v and prints it after label.
func show(label string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%-28s error: %v\n", label, err)
		return
	}
	line(label, string(data))
}

// line prints v after label, aligned with show's output.
func line(label string, v any) {
	fmt.Printf("%-28s %v\n", label, v)
}

func omitEmpty() {
	show("Sensor{Temperature: 0}", Sensor{Temperature: 0})
	// Go 1.26: new accepts an expression, so no temp variable is needed.
	show("NullableSensor{new(0)}", NullableSensor{Temperature: new(0)})
	show("NullableSensor{nil}", NullableSensor{Temperature: nil})

	// On the way in, a pointer tells 0 apart from "not there", but it can't
	// tell a missing key from an explicit null: both leave it nil.
	for _, in := range []string{`{}`, `{"temp":null}`, `{"temp":0}`} {
		var s NullableSensor
		if err := json.Unmarshal([]byte(in), &s); err != nil {
			fmt.Println("error:", err)
			continue
		}
		got := "nil"
		if s.Temperature != nil {
			got = fmt.Sprint(*s.Temperature)
		}
		fmt.Printf("decode %-14s Temperature = %s\n", in, got)
	}
}

func omitZero() {
	// omitempty never omits a struct, so the zero time and the zero Price are
	// still sent. omitzero omits them, using Price.IsZero for Price.
	show("Event, nothing set:", Event{Name: "unset"})
	show("Event, all set (0 EUR):", Event{
		Name: "free", At: instant, AtLegacy: instant,
		Price: Price{Cents: 0, Currency: "EUR"}, PriceEmpty: Price{Cents: 0, Currency: "EUR"},
	})
}

// Login carries a UnixTime, so its JSON has a number instead of a string.
type Login struct {
	At UnixTime `json:"at"`
}

func unixTime() {
	data, err := json.Marshal(Login{At: UnixTime(instant)})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	line("marshal:", string(data))

	var back Login
	if err := json.Unmarshal(data, &back); err != nil {
		fmt.Println("error:", err)
		return
	}
	line("round trip equal:", time.Time(back.At).Equal(instant))

	// null must leave the existing value alone.
	kept := Login{At: UnixTime(instant)}
	if err := json.Unmarshal([]byte(`{"at":null}`), &kept); err != nil {
		fmt.Println("error:", err)
		return
	}
	line("null, this UnmarshalJSON:", time.Time(kept.At).Format(time.RFC3339))

	var slide struct {
		At slideUnixTime `json:"at"`
	}
	slide.At = slideUnixTime(instant)
	if err := json.Unmarshal([]byte(`{"at":null}`), &slide); err != nil {
		fmt.Println("error:", err)
		return
	}
	line("null, slide p05 version:", time.Time(slide.At).Format(time.RFC3339))

	// Quoted legacy values need no custom code: use the ",string" option.
	var legacy Legacy
	if err := json.Unmarshal([]byte(`{"count":"42","active":"true"}`), &legacy); err != nil {
		fmt.Println("error:", err)
		return
	}
	line("\",string\" in:", fmt.Sprintf("%+v", legacy))
	show("\",string\" out:", legacy)
}

func aliasPattern() {
	u := User{Name: "gopher", LastSeen: instant}
	show("value receiver, Marshal(u):", u)
	show("value receiver, Marshal(&u):", &u)

	pu := PtrUser{Name: "gopher", LastSeen: instant}
	show("ptr receiver, Marshal(u):", pu) // MarshalJSON silently skipped
	show("ptr receiver, Marshal(&u):", &pu)
}

func timeZones() {
	est := time.FixedZone("EST", -5*3600) // a fixed zone keeps the output deterministic
	local := instant.In(est)
	show("in EST:", local)
	show("in UTC:", local.UTC())
	line("same instant:", local.Equal(local.UTC()))
	// encoding/json uses RFC3339Nano and trims trailing zeros.
	show("with 1.5 s:", instant.Add(1500*time.Millisecond))
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
