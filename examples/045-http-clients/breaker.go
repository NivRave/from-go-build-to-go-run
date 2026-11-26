package main

import (
	"errors"
	"sync"
	"time"
)

// There is no circuit breaker in the standard library (slide p08). This is a
// minimal hand-rolled one; github.com/sony/gobreaker is a mature package if
// you need more (rolling windows, metrics, callbacks).

// State is the breaker state.
type State int

const (
	Closed   State = iota // requests flow; failures are counted
	Open                  // requests fail fast with ErrOpen
	HalfOpen              // one probe request is allowed through
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	}
	return "unknown"
}

// ErrOpen is returned without calling the function while the breaker is
// open (or while a half-open probe is already in flight).
var ErrOpen = errors.New("circuit breaker is open")

// Breaker opens after threshold consecutive failures, stays open for
// openTimeout, then lets a single probe through: success closes it, failure
// opens it again. It is safe for concurrent use.
type Breaker struct {
	threshold   int
	openTimeout time.Duration

	mu       sync.Mutex
	state    State
	failures int       // consecutive failures while closed
	openedAt time.Time // when the breaker last opened
	probing  bool      // a half-open probe is in flight
}

// NewBreaker returns a closed breaker.
func NewBreaker(threshold int, openTimeout time.Duration) *Breaker {
	return &Breaker{threshold: max(1, threshold), openTimeout: openTimeout}
}

// Do runs fn if the breaker allows it and records the outcome. The lock is
// not held while fn runs, so a slow call doesn't block other callers.
func (b *Breaker) Do(fn func() error) error {
	if err := b.allow(); err != nil {
		return err
	}
	err := fn()
	b.record(err)
	return err
}

// State reports the current state, including an open breaker whose timeout
// has passed (it is half-open: the next call will be a probe).
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == Open && time.Since(b.openedAt) >= b.openTimeout {
		return HalfOpen
	}
	return b.state
}

func (b *Breaker) allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case Open:
		if time.Since(b.openedAt) < b.openTimeout {
			return ErrOpen
		}
		b.state = HalfOpen
		b.probing = true
		return nil
	case HalfOpen:
		if b.probing {
			return ErrOpen // only one probe at a time
		}
		b.probing = true
		return nil
	}
	return nil
}

func (b *Breaker) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err == nil {
		b.state, b.failures, b.probing = Closed, 0, false
		return
	}
	switch b.state {
	case HalfOpen:
		b.state, b.openedAt, b.probing = Open, time.Now(), false
	case Closed:
		b.failures++
		if b.failures >= b.threshold {
			b.state, b.openedAt = Open, time.Now()
		}
	}
}
