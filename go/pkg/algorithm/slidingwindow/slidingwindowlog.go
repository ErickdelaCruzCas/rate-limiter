// Package slidingwindow implements the Sliding Window Log rate limiting algorithm.
//
// # Algorithm Overview
//
// The Sliding Window Log algorithm maintains a log (slice) of timestamps for all
// events within the sliding window. On each request, it prunes expired timestamps
// and checks if adding the new request would exceed the limit.
//
// Key characteristics:
//   - Exact precision: No approximation or boundary problems
//   - O(limit) memory per instance (stores all event timestamps)
//   - O(permits) time complexity per request (must add/prune timestamps)
//   - True sliding window (not fixed to epoch boundaries)
//
// # Algorithm Trade-offs
//
// Pros:
//   - Exact rate limiting: No boundary problem like Fixed Window
//   - True sliding window: Window moves with each request
//   - Precise tracking: Knows exactly when each permit was consumed
//   - Fair: Prevents gaming the system at window boundaries
//
// Cons:
//   - High memory cost: O(limit) timestamps per key
//   - Slower: Must prune timestamps on every request
//   - Not practical for distributed systems: Large state to synchronize
//   - GC pressure: Frequent slice reallocations
//
// # When to Use
//
// Sliding Window Log is appropriate when:
//   - Exact precision is critical (no burst tolerance)
//   - You need to prevent boundary gaming
//   - Memory cost is acceptable (moderate limits)
//   - Single-node deployment (distributed state too expensive)
//
// Example use cases:
//   - High-value operations (payments, resource creation)
//   - Security-critical rate limiting (login attempts, password resets)
//   - Compliance requirements (must enforce exact limits)
//
// Avoid Sliding Window Log when:
//   - Memory is constrained (use Token Bucket or Sliding Window Counter)
//   - Performance is critical (use Fixed Window or Token Bucket)
//   - Limits are very high (e.g., 10,000/second → 10K timestamps)
//   - Distributed deployment needed
//
// # Comparison with Other Algorithms
//
// vs Fixed Window:
//   - More precise (no boundary problem)
//   - Higher memory cost (O(limit) vs O(1))
//   - Slower (must prune vs simple counter)
//
// vs Token Bucket:
//   - No burst tolerance (strict limit enforcement)
//   - Higher memory cost (O(limit) vs O(1))
//   - Different semantics (period-based vs continuous)
//
// vs Sliding Window Counter:
//   - Exact (no approximation)
//   - Higher memory cost (O(limit) vs O(buckets))
//   - Similar complexity
//
// # Concurrency Model
//
// Uses sync.Mutex to protect the event slice. All operations (prune, check, append)
// must be atomic to prevent race conditions.
//
// # Example Usage
//
//	// 100 requests per 60-second window
//	clk := clock.NewSystemClock()
//	swl, err := slidingwindow.New(clk, 60_000_000_000, 100)
//
//	result, err := swl.TryAcquire("user:123", 1)
//	if result.Decision == model.DecisionAllow {
//	    processRequest()
//	} else {
//	    waitTime := time.Duration(result.RetryAfterNanos)
//	    log.Printf("Rate limited, retry after %v", waitTime)
//	}
//
// # Testing with ManualClock
//
//	clk := clock.NewManualClock(0)
//	swl, _ := slidingwindow.New(clk, 10_000_000_000, 5) // 5 per 10 seconds
//
//	// Make 5 requests at t=0
//	for i := 0; i < 5; i++ {
//	    swl.TryAcquire("user1", 1) // ALLOW
//	}
//
//	// 6th request rejected
//	swl.TryAcquire("user1", 1) // REJECT
//
//	// Advance 1 second - still rejected (window is 10 seconds)
//	clk.AdvanceNanos(1_000_000_000)
//	swl.TryAcquire("user1", 1) // REJECT
//
//	// Advance to 10 seconds - oldest event expires
//	clk.SetNanos(10_000_000_001)
//	swl.TryAcquire("user1", 1) // ALLOW (window slides forward)
//
// # Implementation Details
//
// Pruning logic:
//
//	cutoff = now - windowNanos
//	for event in events:
//	    if event <= cutoff: remove
//	    else: break (events are sorted)
//
// Check logic:
//
//	if len(events) + permits <= limit: ALLOW
//	else: REJECT
//
// Retry-after calculation:
//
//	retryAfter = (oldest_event + windowNanos) - now
//
// This tells the client when the oldest event will expire, freeing up capacity.
package slidingwindow

import (
	"fmt"
	"sync"

	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// SlidingWindowLog implements the Sliding Window Log rate limiting algorithm.
//
// Thread-safe: All exported methods are safe for concurrent use by multiple goroutines.
//
// Fields:
//   - clock: Time source (injected for deterministic testing)
//   - windowNanos: Sliding window duration in nanoseconds
//   - limit: Maximum events allowed in the window
//   - events: Slice of event timestamps (sorted, oldest first)
//   - mu: Mutex protecting the events slice
type SlidingWindowLog struct {
	// Immutable fields
	clock       clock.Clock
	windowNanos int64
	limit       int

	// Mutable fields (protected by mu)
	mu     sync.Mutex
	events []int64
}

// New creates a new SlidingWindowLog with the specified configuration.
//
// Parameters:
//   - clk: Clock implementation for time operations
//   - windowNanos: Sliding window duration in nanoseconds (must be > 0)
//   - limit: Maximum events allowed in the window (must be > 0)
//
// Returns:
//   - *SlidingWindowLog: Initialized with empty event log
//   - error: Validation error if parameters are invalid
//
// The event slice is pre-allocated to capacity `limit` to reduce reallocations.
//
// Example:
//
//	// 1000 requests per minute
//	swl, err := slidingwindow.New(clock.NewSystemClock(), 60_000_000_000, 1000)
func New(clk clock.Clock, windowNanos int64, limit int) (*SlidingWindowLog, error) {
	// Validation: window duration
	if windowNanos <= 0 {
		return nil, fmt.Errorf("windowNanos must be > 0, got: %d", windowNanos)
	}

	// Validation: limit
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0, got: %d", limit)
	}

	return &SlidingWindowLog{
		clock:       clk,
		windowNanos: windowNanos,
		limit:       limit,
		events:      make([]int64, 0, limit), // Pre-allocate to limit
	}, nil
}

// TryAcquire attempts to acquire the specified number of permits.
//
// Algorithm:
// 1. Prune expired events (outside sliding window)
// 2. Check if len(events) + permits <= limit
// 3. If yes: append permits to events and return ALLOW
// 4. If no: calculate retry-after and return REJECT
//
// Parameters:
//   - key: Rate limit key (unused in single-instance impl)
//   - permits: Number of permits to acquire (must be > 0)
//
// Returns:
//   - model.RateLimitResult: Decision and retry timing
//   - error: Validation error if permits <= 0
//
// Time complexity: O(permits) for pruning and appending
// Space complexity: O(limit) for storing event timestamps
//
// Note on permits > 1:
//
//	When permits > 1, all permit timestamps are recorded as the current time.
//	This means a single request for 5 permits counts as 5 separate events
//	at the same timestamp.
//
// Retry-after calculation:
//
//	Returns the time until the oldest event expires. Note that waiting this
//	duration does not guarantee success if other requests consume capacity.
//
// Example:
//
//	swl, _ := slidingwindow.New(clock, 10_000_000_000, 100)
//
//	// Request 10 permits
//	result, _ := swl.TryAcquire("user1", 10)
//	// Adds 10 timestamp entries to events slice
func (swl *SlidingWindowLog) TryAcquire(key string, permits int) (model.RateLimitResult, error) {
	// Validation: permits must be positive
	if permits <= 0 {
		return model.RateLimitResult{}, fmt.Errorf("permits must be > 0, got: %d", permits)
	}

	swl.mu.Lock()
	defer swl.mu.Unlock()

	// Step 1: Prune expired events
	now := swl.clock.NowNanos()
	swl.prune(now)

	// Step 2: Check if we can allow this request
	if len(swl.events)+permits <= swl.limit {
		// Success: add event timestamps
		// All permits are recorded with the same timestamp (now)
		for i := 0; i < permits; i++ {
			swl.events = append(swl.events, now)
		}
		return model.Allow(), nil
	}

	// Step 3: Calculate retry-after
	// If events slice is empty (shouldn't happen if len < limit), return 0
	if len(swl.events) == 0 {
		return model.Reject(0), nil
	}

	// Oldest event is at index 0 (events are sorted by construction)
	oldestEvent := swl.events[0]
	retryAfter := (oldestEvent + swl.windowNanos) - now

	// Defensive: ensure non-negative
	if retryAfter < 0 {
		retryAfter = 0
	}

	return model.Reject(retryAfter), nil
}

// prune removes expired events from the log.
//
// This method must be called with swl.mu held.
//
// Events are stored in chronological order (oldest first), so we can
// stop pruning once we find an event that's still valid.
//
// Algorithm:
// 1. Calculate cutoff time: now - windowNanos
// 2. Find index of first valid event (event > cutoff)
// 3. Slice events to remove old entries
//
// Time complexity: O(expired events)
// Space complexity: O(1) (slice reuse)
//
// Example:
//   - Window: 10 seconds
//   - Now: 15 seconds
//   - Cutoff: 5 seconds
//   - Events: [3, 4, 6, 8, 12, 14]
//   - After prune: [6, 8, 12, 14] (removed 3 and 4)
func (swl *SlidingWindowLog) prune(now int64) {
	cutoff := now - swl.windowNanos

	// Find index of first valid event
	validIdx := 0
	for validIdx < len(swl.events) && swl.events[validIdx] <= cutoff {
		validIdx++
	}

	// Remove expired events by slicing
	// This reuses the underlying array, avoiding allocation
	if validIdx > 0 {
		swl.events = swl.events[validIdx:]
	}

	// Note: In Go, slicing doesn't free the underlying array memory.
	// For long-running instances with large bursts, this could lead to
	// memory retention. A production implementation might want to
	// periodically copy to a new slice if cap >> len.
	//
	// For this educational implementation, we keep it simple.
}
