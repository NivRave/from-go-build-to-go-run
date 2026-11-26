package main

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

// Inside synctest.Test, time is virtual: it only advances when every
// goroutine in the bubble is blocked, so these tests run instantly and the
// timings are exact, not approximate.
func TestWatchdogFiresTimeoutAfterLastHeartbeat(t *testing.T) {
	tests := []struct {
		name     string
		beats    int
		interval time.Duration
		timeout  time.Duration
	}{
		{"no heartbeat at all", 0, 10 * time.Millisecond, 50 * time.Millisecond},
		{"three beats", 3, 10 * time.Millisecond, 50 * time.Millisecond},
		{"beats just inside the timeout", 100, 49 * time.Millisecond, 50 * time.Millisecond},
		{"long-running worker", 1000, time.Second, 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				heartbeat := make(chan struct{})
				go func() {
					for range tt.beats {
						time.Sleep(tt.interval)
						heartbeat <- struct{}{}
					}
				}()

				start := time.Now()
				err := Watchdog(heartbeat, tt.timeout)
				elapsed := time.Since(start)

				if !errors.Is(err, ErrWatchdog) {
					t.Fatalf("err = %v, want ErrWatchdog", err)
				}
				lastBeat := time.Duration(tt.beats) * tt.interval
				if want := lastBeat + tt.timeout; elapsed != want {
					t.Errorf("fired after %v, want exactly %v (last beat at %v + timeout %v)",
						elapsed, want, lastBeat, tt.timeout)
				}
			})
		})
	}
}

func TestWatchdogClosedHeartbeatCountsAsLastBeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		heartbeat := make(chan struct{})
		go func() {
			time.Sleep(20 * time.Millisecond)
			close(heartbeat) // the worker exits
		}()

		start := time.Now()
		if err := Watchdog(heartbeat, 50*time.Millisecond); !errors.Is(err, ErrWatchdog) {
			t.Fatalf("err = %v, want ErrWatchdog", err)
		}
		if got, want := time.Since(start), 70*time.Millisecond; got != want {
			t.Errorf("fired after %v, want %v", got, want)
		}
	})
}
