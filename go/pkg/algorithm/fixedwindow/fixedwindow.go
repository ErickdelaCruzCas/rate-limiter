// Package fixedwindow implements the Fixed Window rate limiting algorithm.
//
// # Algorithm Overview
//
// The Fixed Window algorithm divides time into fixed-size windows (e.g., 1 second, 1 minute)
// aligned to the epoch. Each window has a counter that tracks the number of permits consumed.
// When a window ends, the counter resets to zero for the next window.
//
// Key characteristics:
//   - Simple counter-based implementation
//   - O(1) memory per instance
//   - O(1) time complexity per request
//   - Window boundaries aligned to epoch (deterministic)
//   - Counter resets at window boundaries
//
// # Algorithm Trade-offs
//
// Pros:
//   - Extremely simple to implement and understand
//   - Minimal memory footprint (just a counter and timestamp)
//   - Fast: no complex calculations, just integer arithmetic
//   - Predictable: window boundaries are deterministic
//
// Cons:
//   - Boundary problem: Can allow up to 2x the limit at window edges
//   - Coarse-grained: Does not track request distribution within window
//   - Reset spikes: All capacity becomes available at window boundary
//
// # The Boundary Problem
//
// This is the main weakness of Fixed Window. Consider a limit of 100 requests/minute:
//
//	Time:    ...  0:59  1:00  1:01  ...
//	Window:      [  A  ]|[  B  ]
//	Requests:       100    100
//	             <-----------200 requests in 2 seconds!
//
// At 0:59, a client makes 100 requests (allowed, window A at limit).
// At 1:00, window resets, client makes 100 more requests (allowed, window B at limit).
// Result: 200 requests in 2 seconds, violating the intended rate.
//
// This problem is inherent to the algorithm and cannot be fixed without
// changing to a sliding window approach.
//
// # When to Use
//
// Fixed Window is appropriate for:
//   - Strict per-period quotas (e.g., "100 API calls per day")
//   - Billing/metering where period boundaries matter
//   - Simple rate limiting where burst tolerance is acceptable
//   - Memory-constrained environments
//
// Avoid Fixed Window when:
//   - You need to prevent bursts at period boundaries (use Sliding Window)
//   - You need precise rate control (use Token Bucket or Sliding Window Log)
//   - You care about request distribution within periods
//
// # Concurrency Model
//
// This implementation uses sync.Mutex for thread-safety. The window counter
// and timestamp must update atomically, making mutex the natural choice.
//
// # Example Usage
//
//	// 10 requests per 1-second window
//	clk := clock.NewSystemClock()
//	fw, err := fixedwindow.New(clk, 1_000_000_000, 10)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check rate limit
//	result, err := fw.TryAcquire("user:123", 1)
//	if result.Decision == model.DecisionAllow {
//	    processRequest()
//	} else {
//	    retryAfter := time.Duration(result.RetryAfterNanos)
//	    log.Printf("Rate limited, retry after %v", retryAfter)
//	}
//
// # Testing with ManualClock
//
//	clk := clock.NewManualClock(0)
//	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10) // 10 per second
//
//	// Make 10 requests (all allowed)
//	for i := 0; i < 10; i++ {
//	    fw.TryAcquire("user1", 1) // ALLOW
//	}
//
//	// 11th request rejected
//	result, _ := fw.TryAcquire("user1", 1) // REJECT
//
//	// Advance to next window
//	clk.AdvanceNanos(1_000_000_000)
//
//	// Counter reset, request allowed
//	fw.TryAcquire("user1", 1) // ALLOW
//
// # Implementation Details
//
// Window alignment:
//
//	windowStart = (now / windowNanos) * windowNanos
//
// This aligns windows to epoch, ensuring consistent boundaries across instances.
//
// Retry-after calculation:
//
//	retryAfter = (windowStart + windowNanos) - now
//
// This returns the time until the next window starts.
package fixedwindow

import (
	"fmt"
	"sync"

	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// FixedWindow implements the Fixed Window rate limiting algorithm.
//
// Thread-safe: All exported methods are safe for concurrent use by multiple goroutines.
//
// Fields:
//   - clock: Time source (injected for deterministic testing)
//   - windowNanos: Window duration in nanoseconds
//   - limit: Maximum permits per window
//   - windowStart: Start timestamp of current window (aligned to epoch)
//   - used: Number of permits consumed in current window
//   - mu: Mutex protecting mutable fields (windowStart, used)
type FixedWindow struct {
	// Immutable fields
	clock       clock.Clock
	windowNanos int64
	limit       int

	// Mutable fields (protected by mu)
	mu          sync.Mutex
	windowStart int64
	used        int
}

// New creates a new FixedWindow with the specified configuration.
//
// Parameters:
//   - clk: Clock implementation for time operations
//   - windowNanos: Window duration in nanoseconds (must be > 0)
//     Common values:
//   - 1 second: 1_000_000_000
//   - 1 minute: 60_000_000_000
//   - 1 hour: 3_600_000_000_000
//   - limit: Maximum permits per window (must be > 0)
//
// Returns:
//   - *FixedWindow: Initialized window starting at current aligned epoch
//   - error: Validation error if parameters are invalid
//
// The window starts aligned to the epoch based on current time.
// For example, if windowNanos=1s and current time is 5.7 seconds since epoch,
// the window starts at 5.0 seconds and ends at 6.0 seconds.
//
// Example:
//
//	// 100 requests per minute
//	fw, err := fixedwindow.New(clock.NewSystemClock(), 60_000_000_000, 100)
func New(clk clock.Clock, windowNanos int64, limit int) (*FixedWindow, error) {
	// Validation: window duration
	if windowNanos <= 0 {
		return nil, fmt.Errorf("windowNanos must be > 0, got: %d", windowNanos)
	}

	// Validation: limit
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0, got: %d", limit)
	}

	// Initialize window aligned to current time
	now := clk.NowNanos()
	windowStart := align(now, windowNanos)

	return &FixedWindow{
		clock:       clk,
		windowNanos: windowNanos,
		limit:       limit,
		windowStart: windowStart,
		used:        0,
	}, nil
}

// TryAcquire attempts to acquire the specified number of permits.
//
// Algorithm:
// 1. Get current time and align to window boundary
// 2. If window changed: reset counter to 0
// 3. Check if used + permits <= limit
// 4. If yes: increment counter and return ALLOW
// 5. If no: calculate retry-after and return REJECT
//
// Parameters:
//   - key: Rate limit key (unused in single-instance impl)
//   - permits: Number of permits to acquire (must be > 0)
//
// Returns:
//   - model.RateLimitResult: Decision and retry timing
//   - error: Validation error if permits <= 0
//
// Retry-after calculation:
//
//	The retry-after value indicates when the next window starts.
//	Note that waiting until the next window does not guarantee success
//	if other requests consume capacity in that window.
//
// Example:
//
//	fw, _ := fixedwindow.New(clock, 1_000_000_000, 10)
//
//	// First 10 requests allowed
//	for i := 0; i < 10; i++ {
//	    result, _ := fw.TryAcquire("user1", 1) // ALLOW
//	}
//
//	// 11th request rejected
//	result, _ := fw.TryAcquire("user1", 1) // REJECT
//	// result.RetryAfterNanos = time until next window (0-1 second)
func (fw *FixedWindow) TryAcquire(key string, permits int) (model.RateLimitResult, error) {
	// Validation: permits must be positive
	if permits <= 0 {
		return model.RateLimitResult{}, fmt.Errorf("permits must be > 0, got: %d", permits)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Get current time and aligned window start
	now := fw.clock.NowNanos()
	currentWindowStart := align(now, fw.windowNanos)

	// Check if window has changed
	if currentWindowStart != fw.windowStart {
		// New window: reset counter
		fw.windowStart = currentWindowStart
		fw.used = 0
	}

	// Check if we can allow this request
	if fw.used+permits <= fw.limit {
		// Success: increment counter and allow
		fw.used += permits
		return model.Allow(), nil
	}

	// Failure: calculate retry-after
	// Client should wait until the next window starts
	nextWindowStart := fw.windowStart + fw.windowNanos
	retryAfter := nextWindowStart - now

	// Defensive: ensure retry-after is non-negative
	// (should never be negative if clock is monotonic)
	if retryAfter < 0 {
		retryAfter = 0
	}

	return model.Reject(retryAfter), nil
}

// align returns the start of the window containing the given timestamp.
//
// This function performs epoch alignment by truncating the timestamp
// to the nearest window boundary.
//
// Formula: (now / windowNanos) * windowNanos
//
// Example with windowNanos=1_000_000_000 (1 second):
//   - align(500_000_000) = 0
//   - align(1_500_000_000) = 1_000_000_000
//   - align(5_700_000_000) = 5_000_000_000
//
// This ensures that all instances of FixedWindow with the same windowNanos
// will have synchronized window boundaries, regardless of when they were created.
func align(now, windowNanos int64) int64 {
	return (now / windowNanos) * windowNanos
}
