package main

import (
	"errors"
	"time"
)

// ErrWatchdog is returned when no heartbeat arrived within the timeout.
var ErrWatchdog = errors.New("worker watchdog triggered")

// Watchdog returns ErrWatchdog once timeout passes without a value on
// heartbeat (slide p05). It returns exactly timeout after the last heartbeat.
//
// Unlike the slide, there is no `if !t.Stop() { <-t.C }` drain before Reset.
// Since Go 1.23 (with `go 1.23` or later in go.mod) timer channels are
// synchronous: Reset and Stop guarantee that no stale value is received
// afterwards, so a plain Reset is correct. The old drain idiom can even block
// forever if the value was already received.
//
// A closed heartbeat channel counts as the last heartbeat: the case is
// switched off and the watchdog fires timeout later.
func Watchdog(heartbeat <-chan struct{}, timeout time.Duration) error {
	t := time.NewTimer(timeout) // the only way out is t firing, so no Stop is needed
	for {
		select {
		case _, ok := <-heartbeat:
			if !ok {
				heartbeat = nil // no more beats can come; let the timer decide
			}
			t.Reset(timeout) // the worker is still pulsing: push the deadline out
		case <-t.C:
			// No pulse. The slide's comment says "restarting the worker", but
			// the watchdog only reports; restarting is the caller's decision.
			return ErrWatchdog
		}
	}
}
