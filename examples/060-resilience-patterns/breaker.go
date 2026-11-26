package main

import (
	"errors"
	"sync"
	"time"
)

// A minimal circuit breaker with the same vocabulary as sony/gobreaker
// (Settings, Counts, ReadyToTrip, Execute, ErrOpenState), written with the
// standard library only. Like gobreaker/v2 it is generic, so Execute returns
// a typed value and the caller never needs the type assertion that makes the
// slide p07 code panic when the breaker is open.
//
// Not implemented, for brevity: gobreaker's Interval (periodically clearing
// counts while closed) and its generations (discarding the result of a call
// that started before the last state change).

// State is the breaker's state.
type State int

const (
	StateClosed   State = iota // normal: calls go through, failures are counted
	StateHalfOpen              // probing: a few calls go through to test recovery
	StateOpen                  // tripped: calls fail at once without reaching the provider
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

var (
	// ErrOpenState is returned, without calling fn, while the breaker is open.
	ErrOpenState = errors.New("circuit breaker is open")
	// ErrTooManyRequests is returned in half-open state once MaxRequests
	// probes are already in progress.
	ErrTooManyRequests = errors.New("circuit breaker: too many requests while half-open")
)

// Counts are the request statistics for the current state. They reset on
// every state change.
type Counts struct {
	Requests             int
	TotalSuccesses       int
	TotalFailures        int
	ConsecutiveSuccesses int
	ConsecutiveFailures  int
}

// Settings configure a CircuitBreaker.
type Settings struct {
	Name string
	// MaxRequests is how many probe calls half-open state lets through; that
	// many consecutive successes close the breaker. Default 1.
	MaxRequests int
	// Timeout is how long the breaker stays open before going half-open.
	Timeout time.Duration
	// ReadyToTrip decides, after each failure while closed, whether to open.
	// Default: more than 5 consecutive failures, as on the slide.
	ReadyToTrip func(Counts) bool
	// OnStateChange is called after every transition, outside the lock.
	OnStateChange func(name string, from, to State)
	// IsSuccessful classifies fn's error. Default: err == nil. Use it to keep
	// errors that say nothing about the provider's health, such as the
	// caller's own context.Canceled, from tripping the breaker.
	IsSuccessful func(err error) bool
}

// CircuitBreaker guards calls that return a T.
type CircuitBreaker[T any] struct {
	st Settings

	mu       sync.Mutex
	state    State
	counts   Counts
	openedAt time.Time
	probes   int // half-open calls in progress
}

func NewCircuitBreaker[T any](st Settings) *CircuitBreaker[T] {
	if st.MaxRequests <= 0 {
		st.MaxRequests = 1
	}
	if st.ReadyToTrip == nil {
		st.ReadyToTrip = func(c Counts) bool { return c.ConsecutiveFailures > 5 }
	}
	if st.IsSuccessful == nil {
		st.IsSuccessful = func(err error) bool { return err == nil }
	}
	return &CircuitBreaker[T]{st: st}
}

// State returns the current state, moving from open to half-open if Timeout
// has passed.
func (cb *CircuitBreaker[T]) State() State {
	cb.mu.Lock()
	from, to := cb.refresh(time.Now())
	state := cb.state
	cb.mu.Unlock()
	cb.notify(from, to)
	return state
}

// Execute runs fn if the breaker allows it and records the result. While the
// breaker is open it returns the zero T and ErrOpenState without calling fn.
func (cb *CircuitBreaker[T]) Execute(fn func() (T, error)) (T, error) {
	var zero T
	if err := cb.before(); err != nil {
		return zero, err
	}
	res, err := fn()
	cb.after(cb.st.IsSuccessful(err))
	return res, err
}

func (cb *CircuitBreaker[T]) before() error {
	cb.mu.Lock()
	from, to := cb.refresh(time.Now())
	var err error
	switch cb.state {
	case StateOpen:
		err = ErrOpenState
	case StateHalfOpen:
		if cb.probes >= cb.st.MaxRequests {
			err = ErrTooManyRequests
		} else {
			cb.probes++
		}
	}
	if err == nil {
		cb.counts.Requests++
	}
	cb.mu.Unlock()
	cb.notify(from, to)
	return err
}

func (cb *CircuitBreaker[T]) after(success bool) {
	cb.mu.Lock()
	var from, to State
	if success {
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0
		if cb.state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.st.MaxRequests {
			from, to = cb.setState(StateClosed, time.Now())
		}
	} else {
		cb.counts.TotalFailures++
		cb.counts.ConsecutiveFailures++
		cb.counts.ConsecutiveSuccesses = 0
		switch {
		case cb.state == StateHalfOpen:
			from, to = cb.setState(StateOpen, time.Now()) // the probe failed: back off again
		case cb.state == StateClosed && cb.st.ReadyToTrip(cb.counts):
			from, to = cb.setState(StateOpen, time.Now())
		}
	}
	cb.mu.Unlock()
	cb.notify(from, to)
}

// refresh moves open to half-open once Timeout has passed. Callers hold mu.
func (cb *CircuitBreaker[T]) refresh(now time.Time) (from, to State) {
	if cb.state == StateOpen && now.Sub(cb.openedAt) >= cb.st.Timeout {
		return cb.setState(StateHalfOpen, now)
	}
	return cb.state, cb.state
}

// setState switches state and resets the counts. Callers hold mu.
func (cb *CircuitBreaker[T]) setState(s State, now time.Time) (from, to State) {
	from = cb.state
	cb.state = s
	cb.counts = Counts{}
	cb.probes = 0
	if s == StateOpen {
		cb.openedAt = now
	}
	return from, s
}

// notify runs the callback outside the lock, so OnStateChange may call back
// into the breaker (for example State) without deadlocking.
func (cb *CircuitBreaker[T]) notify(from, to State) {
	if from != to && cb.st.OnStateChange != nil {
		cb.st.OnStateChange(cb.st.Name, from, to)
	}
}
