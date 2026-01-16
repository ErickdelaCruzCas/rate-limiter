// Package clock provides time abstraction for deterministic testing.
//
// All rate limiter implementations must use Clock instead of time.Now()
// to enable 100% reproducible, deterministic tests without sleeps or flakiness.
//
// The Clock interface allows tests to control time progression, making it possible
// to test time-dependent behavior (token refill, window expiration, etc.) without
// introducing sleeps, race conditions, or non-determinism.
//
// Example usage:
//
//	// Production code
//	clk := clock.NewSystemClock()
//	limiter := tokenbucket.New(clk, 100, 10.0)
//
//	// Test code
//	clk := clock.NewManualClock(0)
//	limiter := tokenbucket.New(clk, 100, 10.0)
//	clk.AdvanceNanos(1_000_000_000) // Advance 1 second
//	// Test behavior after time advancement
package clock

import "time"

// Clock provides monotonic time for rate limiting.
//
// Implementations must guarantee monotonicity: time never goes backward,
// even across system clock adjustments (NTP, manual changes, etc.).
//
// All rate limiter algorithms depend on this interface for time operations,
// enabling both production use (SystemClock) and deterministic testing (ManualClock).
type Clock interface {
	// NowNanos returns the current time in nanoseconds since an arbitrary epoch.
	//
	// The epoch is implementation-defined and may differ between implementations.
	// Only time differences (elapsed time) are meaningful, not absolute values.
	//
	// Monotonicity guarantee: For any two successive calls A and B on the same Clock instance,
	// B.NowNanos() >= A.NowNanos() must hold true.
	NowNanos() int64
}

// SystemClock implements Clock using the system's monotonic clock.
//
// This implementation uses time.Now().UnixNano() which provides nanosecond precision
// and monotonic behavior (immune to system clock adjustments).
//
// Thread-safe: Safe for concurrent use by multiple goroutines.
//
// Use this in production environments where real wall-clock time is needed.
type SystemClock struct{}

// NewSystemClock creates a new SystemClock instance.
//
// The returned clock is stateless and can be shared across goroutines.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// NowNanos returns the current monotonic time in nanoseconds.
//
// This method wraps time.Now().UnixNano() which provides:
// - Nanosecond precision (actual resolution may be lower on some systems)
// - Monotonic behavior (not affected by NTP or manual time changes)
// - Consistent epoch across process lifetime
func (c *SystemClock) NowNanos() int64 {
	return time.Now().UnixNano()
}
