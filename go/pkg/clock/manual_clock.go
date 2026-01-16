package clock

import (
	"fmt"
	"sync"
)

// ManualClock provides controllable time for deterministic testing.
//
// This clock allows tests to:
// - Advance time by specific durations (AdvanceNanos)
// - Jump to absolute timestamps (SetNanos)
// - Avoid sleeps and race conditions
// - Achieve 100% reproducible results
//
// ManualClock is the foundation of deterministic testing in this codebase.
// All algorithm tests use ManualClock to control time progression, eliminating
// the need for time.Sleep() calls and ensuring tests run fast and reliably.
//
// Thread-safe: Safe for concurrent use by multiple goroutines.
// The internal mutex ensures that time updates and reads are atomic.
//
// Example usage:
//
//	clk := clock.NewManualClock(0) // Start at time 0
//
//	// Test initial state
//	limiter := tokenbucket.New(clk, 10, 1.0)
//	result, _ := limiter.TryAcquire("user1", 10) // Consumes all tokens
//	// result.Decision == DecisionAllow
//
//	result, _ = limiter.TryAcquire("user1", 1)
//	// result.Decision == DecisionReject (no tokens left)
//
//	// Advance time to refill tokens
//	clk.AdvanceNanos(5_000_000_000) // Advance 5 seconds
//
//	result, _ = limiter.TryAcquire("user1", 5)
//	// result.Decision == DecisionAllow (5 tokens refilled at 1 token/sec)
type ManualClock struct {
	mu  sync.Mutex
	now int64
}

// NewManualClock creates a ManualClock starting at the specified time.
//
// The startNanos parameter defines the initial time value in nanoseconds.
// Common patterns:
// - Start at 0: NewManualClock(0)
// - Start at a specific Unix timestamp: NewManualClock(time.Now().UnixNano())
//
// The choice of startNanos is arbitrary; only time differences matter for rate limiting.
func NewManualClock(startNanos int64) *ManualClock {
	return &ManualClock{now: startNanos}
}

// NowNanos returns the current manual time in nanoseconds.
//
// This method is thread-safe and can be called concurrently with AdvanceNanos
// and SetNanos from multiple goroutines.
//
// The returned value is guaranteed to be monotonic within a single goroutine's
// perspective, but concurrent goroutines may observe time moving backward if
// SetNanos is called concurrently. For deterministic testing, avoid concurrent
// time manipulation from multiple goroutines.
func (c *ManualClock) NowNanos() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AdvanceNanos advances the clock by the specified duration.
//
// This is the primary method for simulating time passage in tests.
// It increments the current time by delta nanoseconds, preserving monotonicity.
//
// Parameters:
//   - delta: Duration in nanoseconds to advance (must be >= 0)
//
// Returns:
//   - error if delta is negative
//
// Example:
//
//	clk := NewManualClock(0)
//	clk.AdvanceNanos(1_000_000_000)  // Advance 1 second
//	clk.AdvanceNanos(500_000_000)    // Advance 0.5 seconds
//	// Current time is now 1.5 seconds
//
// Common time constants:
// - 1 nanosecond: 1
// - 1 microsecond: 1_000
// - 1 millisecond: 1_000_000
// - 1 second: 1_000_000_000
// - 1 minute: 60_000_000_000
func (c *ManualClock) AdvanceNanos(delta int64) error {
	if delta < 0 {
		return fmt.Errorf("delta must be >= 0, got: %d", delta)
	}
	c.mu.Lock()
	c.now += delta
	c.mu.Unlock()
	return nil
}

// SetNanos sets the clock to an absolute time value.
//
// Unlike AdvanceNanos, this method can move time backward, breaking monotonicity.
// Use this method carefully, typically only at the start of tests to set an initial state.
//
// Prefer AdvanceNanos over SetNanos when possible to maintain monotonicity guarantees.
//
// Parameters:
//   - value: Absolute time in nanoseconds to set
//
// Example:
//
//	clk := NewManualClock(0)
//	clk.SetNanos(1_000_000_000)  // Jump to 1 second
//	clk.SetNanos(500_000_000)    // Jump back to 0.5 seconds (non-monotonic!)
func (c *ManualClock) SetNanos(value int64) {
	c.mu.Lock()
	c.now = value
	c.mu.Unlock()
}
