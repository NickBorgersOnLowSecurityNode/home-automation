// Package clock provides a time abstraction for testable time-dependent code.
// Use RealClock for production and MockClock for testing.
package clock

import (
	"sync"
	"time"
)

// Clock is an interface for time operations, allowing time to be mocked in tests.
type Clock interface {
	// Now returns the current time
	Now() time.Time

	// After waits for the duration to elapse and then sends the current time on the returned channel
	After(d time.Duration) <-chan time.Time

	// AfterFunc waits for the duration to elapse and then calls f in its own goroutine.
	// It returns a Timer that can be used to cancel the call using its Stop method.
	AfterFunc(d time.Duration, f func()) Timer

	// Sleep pauses the current goroutine for at least the duration d
	Sleep(d time.Duration)

	// Since returns the time elapsed since t
	Since(t time.Time) time.Duration
}

// Timer represents a single event that can be stopped
type Timer interface {
	// Stop prevents the Timer from firing. Returns true if the call stops the timer,
	// false if the timer has already expired or been stopped.
	Stop() bool

	// Reset changes the timer to expire after duration d.
	// Returns true if the timer had been active, false if the timer had expired or been stopped.
	Reset(d time.Duration) bool
}

// RealClock implements Clock using the standard time package
type RealClock struct{}

// realTimer wraps time.Timer to implement our Timer interface
type realTimer struct {
	timer *time.Timer
}

// NewRealClock creates a new RealClock instance
func NewRealClock() *RealClock {
	return &RealClock{}
}

// Now returns the current time
func (c *RealClock) Now() time.Time {
	return time.Now()
}

// After waits for the duration to elapse and then sends the current time
func (c *RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// AfterFunc waits for the duration to elapse and then calls f
func (c *RealClock) AfterFunc(d time.Duration, f func()) Timer {
	return &realTimer{timer: time.AfterFunc(d, f)}
}

// Sleep pauses the current goroutine for at least the duration d
func (c *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// Since returns the time elapsed since t
func (c *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// Stop prevents the Timer from firing
func (t *realTimer) Stop() bool {
	return t.timer.Stop()
}

// Reset changes the timer to expire after duration d
func (t *realTimer) Reset(d time.Duration) bool {
	return t.timer.Reset(d)
}

// MockClock is a Clock implementation for testing that allows manual time control
type MockClock struct {
	mu      sync.Mutex
	current time.Time
	timers  []*mockTimer
}

type mockTimer struct {
	clock    *MockClock
	deadline time.Time
	f        func()
	stopped  bool
	mu       sync.Mutex
}

// NewMockClock creates a new MockClock starting at the given time
func NewMockClock(start time.Time) *MockClock {
	return &MockClock{
		current: start,
		timers:  make([]*mockTimer, 0),
	}
}

// Now returns the mock current time
func (c *MockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// After returns a channel that will receive the time after duration d
func (c *MockClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.AfterFunc(d, func() {
		c.mu.Lock()
		t := c.current
		c.mu.Unlock()
		ch <- t
	})
	return ch
}

// AfterFunc schedules f to be called after duration d
func (c *MockClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()

	timer := &mockTimer{
		clock:    c,
		deadline: c.current.Add(d),
		f:        f,
		stopped:  false,
	}
	c.timers = append(c.timers, timer)
	return timer
}

// Sleep does nothing immediately in MockClock - time only advances via Advance()
func (c *MockClock) Sleep(d time.Duration) {
	// In mock mode, Sleep is a no-op. Use Advance() to move time forward.
	// This allows tests to control exactly when time passes.
}

// Since returns the time elapsed since t using the mock current time
func (c *MockClock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current.Sub(t)
}

// Advance moves the mock clock forward by duration d and fires any timers that have expired
func (c *MockClock) Advance(d time.Duration) {
	c.mu.Lock()
	newTime := c.current.Add(d)
	c.current = newTime

	// Find all timers that should fire
	var toFire []*mockTimer
	var remaining []*mockTimer

	for _, timer := range c.timers {
		timer.mu.Lock()
		if !timer.stopped && !timer.deadline.After(newTime) {
			toFire = append(toFire, timer)
		} else if !timer.stopped {
			remaining = append(remaining, timer)
		}
		timer.mu.Unlock()
	}

	c.timers = remaining
	c.mu.Unlock()

	// Fire timers outside the lock to prevent deadlocks
	for _, timer := range toFire {
		timer.mu.Lock()
		if !timer.stopped {
			timer.stopped = true
			f := timer.f
			timer.mu.Unlock()
			f()
		} else {
			timer.mu.Unlock()
		}
	}
}

// Set sets the mock clock to a specific time and fires any expired timers
func (c *MockClock) Set(t time.Time) {
	c.mu.Lock()
	oldTime := c.current
	c.mu.Unlock()

	if t.After(oldTime) {
		c.Advance(t.Sub(oldTime))
	} else {
		c.mu.Lock()
		c.current = t
		c.mu.Unlock()
	}
}

// Stop prevents the timer from firing
func (t *mockTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	wasActive := !t.stopped
	t.stopped = true
	return wasActive
}

// Reset changes the timer to expire after duration d from now
func (t *mockTimer) Reset(d time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	wasActive := !t.stopped
	t.stopped = false

	t.clock.mu.Lock()
	t.deadline = t.clock.current.Add(d)
	// Re-add to timers list if it was stopped
	if !wasActive {
		t.clock.timers = append(t.clock.timers, t)
	}
	t.clock.mu.Unlock()

	return wasActive
}
