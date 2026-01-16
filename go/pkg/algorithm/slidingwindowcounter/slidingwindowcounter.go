// Package slidingwindowcounter implements the Sliding Window Counter rate limiting algorithm.
//
// # Algorithm Overview
//
// The Sliding Window Counter divides the sliding window into fixed-size buckets (sub-windows)
// and maintains a count for each bucket in a ring buffer. As time advances, old buckets are
// zeroed out (rolled forward) and new buckets are used.
//
// Key characteristics:
//   - O(numBuckets) memory per instance (fixed-size ring buffer)
//   - O(1) amortized time complexity (bucket rolling is amortized)
//   - Approximate: Precision depends on bucket granularity
//   - Practical: Balances memory, performance, and precision
//
// # Algorithm Trade-offs
//
// Pros:
//   - Bounded memory: O(buckets) regardless of limit
//   - Better than Fixed Window: Reduces boundary problem severity
//   - Practical: Good balance for production use
//   - Predictable: Fixed-size data structure
//
// Cons:
//   - Approximate: Not as precise as Sliding Window Log
//   - Bucket granularity trade-off: More buckets = more precision but more memory
//   - Retry-after is approximate (next bucket boundary)
//   - More complex than simpler algorithms
//
// # Comparison with Other Algorithms
//
// vs Fixed Window:
//   - Better precision (smaller bucket size)
//   - Slightly higher memory (numBuckets vs 1)
//   - Similar performance
//
// vs Sliding Window Log:
//   - Lower memory (O(buckets) vs O(limit))
//   - Approximate vs exact
//   - Similar or better performance
//
// vs Token Bucket:
//   - Different semantics (period-based vs continuous)
//   - Higher memory (O(buckets) vs O(1))
//   - No burst tolerance
//
// # When to Use
//
// Sliding Window Counter is appropriate for:
//   - Production rate limiting (good all-around choice)
//   - When Sliding Window Log's memory cost is too high
//   - When Fixed Window's boundary problem is unacceptable
//   - Moderate to high rate limits (e.g., 1000s/second)
//
// Example use cases:
//   - API rate limiting for production services
//   - Request throttling with bounded memory
//   - Distributed systems (bounded state size)
//
// Avoid when:
//   - Exact precision is critical (use Sliding Window Log)
//   - Memory is extremely constrained (use Fixed Window or Token Bucket)
//   - Burst tolerance is needed (use Token Bucket)
//
// # Bucket Configuration
//
// The number and size of buckets determines precision:
//
//	bucketSize = windowSize / numBuckets
//
// Examples:
//   - 60s window, 60 buckets → 1s bucket size (good precision)
//   - 60s window, 6 buckets → 10s bucket size (moderate precision)
//   - 60s window, 1 bucket → 60s bucket size (degrades to Fixed Window)
//
// Rule of thumb: Use 10-60 buckets for most applications.
//
// # Concurrency Model
//
// Uses sync.Mutex to protect the ring buffer. Bucket rolling and count updates
// must be atomic to prevent race conditions.
//
// # Example Usage
//
//	// 100 requests per minute, using 60 buckets (1-second granularity)
//	clk := clock.NewSystemClock()
//	swc, err := slidingwindowcounter.New(
//	    clk,
//	    60_000_000_000, // 60 second window
//	    1_000_000_000,  // 1 second per bucket
//	    100,            // limit
//	)
//
//	result, err := swc.TryAcquire("user:123", 1)
//	if result.Decision == model.DecisionAllow {
//	    processRequest()
//	} else {
//	    waitTime := time.Duration(result.RetryAfterNanos)
//	    log.Printf("Rate limited, retry after %v", waitTime)
//	}
//
// # Implementation Details
//
// Ring buffer indexing:
//
//	index = (bucketStart / bucketNanos) % numBuckets
//
// Bucket rolling:
//
//	steps = (currentBucket - lastBucket) / bucketNanos
//	for i := 0; i < steps; i++:
//	    roll forward one bucket, zero it out
//
// The ring buffer ensures O(buckets) memory even as time advances indefinitely.
package slidingwindowcounter

import (
	"fmt"
	"sync"

	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// SlidingWindowCounter implements the Sliding Window Counter rate limiting algorithm.
//
// Thread-safe: All exported methods are safe for concurrent use by multiple goroutines.
//
// Fields:
//   - clock: Time source (injected for deterministic testing)
//   - windowNanos: Total sliding window duration
//   - bucketNanos: Duration of each bucket (sub-window)
//   - numBuckets: Number of buckets in the ring buffer
//   - limit: Maximum permits allowed in the window
//   - counts: Ring buffer of bucket counts
//   - lastBucketStart: Start timestamp of the most recent bucket
//   - total: Sum of all bucket counts (cached for performance)
//   - mu: Mutex protecting mutable fields
type SlidingWindowCounter struct {
	// Immutable fields
	clock       clock.Clock
	windowNanos int64
	bucketNanos int64
	numBuckets  int
	limit       int

	// Mutable fields (protected by mu)
	mu              sync.Mutex
	counts          []int
	lastBucketStart int64
	total           int
}

// New creates a new SlidingWindowCounter with the specified configuration.
//
// Parameters:
//   - clk: Clock implementation for time operations
//   - windowNanos: Total sliding window duration (must be > 0)
//   - bucketNanos: Duration of each bucket (must be > 0)
//   - limit: Maximum permits allowed in the window (must be > 0)
//
// Validation:
//   - windowNanos must be divisible by bucketNanos
//   - numBuckets = windowNanos / bucketNanos
//
// Returns:
//   - *SlidingWindowCounter: Initialized with empty ring buffer
//   - error: Validation error if parameters are invalid
//
// Example:
//
//	// 100 requests per minute, 60 one-second buckets
//	swc, err := slidingwindowcounter.New(
//	    clock.NewSystemClock(),
//	    60_000_000_000, // 60s window
//	    1_000_000_000,  // 1s buckets
//	    100,            // limit
//	)
func New(clk clock.Clock, windowNanos, bucketNanos int64, limit int) (*SlidingWindowCounter, error) {
	// Validation: window duration
	if windowNanos <= 0 {
		return nil, fmt.Errorf("windowNanos must be > 0, got: %d", windowNanos)
	}

	// Validation: bucket duration
	if bucketNanos <= 0 {
		return nil, fmt.Errorf("bucketNanos must be > 0, got: %d", bucketNanos)
	}

	// Validation: window must be divisible by bucket
	if windowNanos%bucketNanos != 0 {
		return nil, fmt.Errorf("windowNanos (%d) must be divisible by bucketNanos (%d)",
			windowNanos, bucketNanos)
	}

	// Validation: limit
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0, got: %d", limit)
	}

	numBuckets := int(windowNanos / bucketNanos)
	now := clk.NowNanos()
	lastBucketStart := align(now, bucketNanos)

	return &SlidingWindowCounter{
		clock:           clk,
		windowNanos:     windowNanos,
		bucketNanos:     bucketNanos,
		numBuckets:      numBuckets,
		limit:           limit,
		counts:          make([]int, numBuckets),
		lastBucketStart: lastBucketStart,
		total:           0,
	}, nil
}

// TryAcquire attempts to acquire the specified number of permits.
//
// Algorithm:
// 1. Roll buckets forward to current time
// 2. Check if total + permits <= limit
// 3. If yes: add permits to current bucket and return ALLOW
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
// Time complexity: O(1) amortized (rolling is amortized over time)
// Space complexity: O(numBuckets) for the ring buffer
//
// Example:
//
//	swc, _ := slidingwindowcounter.New(clock, 60_000_000_000, 1_000_000_000, 100)
//
//	result, _ := swc.TryAcquire("user1", 5)
//	if result.Decision == model.DecisionAllow {
//	    // Success - 5 permits added to current bucket
//	}
func (swc *SlidingWindowCounter) TryAcquire(key string, permits int) (model.RateLimitResult, error) {
	// Validation: permits must be positive
	if permits <= 0 {
		return model.RateLimitResult{}, fmt.Errorf("permits must be > 0, got: %d", permits)
	}

	swc.mu.Lock()
	defer swc.mu.Unlock()

	// Step 1: Roll buckets forward to current time
	now := swc.clock.NowNanos()
	swc.roll(now)

	// Step 2: Check if we can allow this request
	if swc.total+permits <= swc.limit {
		// Success: add permits to current bucket
		currentBucketStart := align(now, swc.bucketNanos)
		idx := swc.indexOf(currentBucketStart)
		swc.counts[idx] += permits
		swc.total += permits
		return model.Allow(), nil
	}

	// Step 3: Calculate retry-after
	// Client should wait until next bucket starts
	currentBucketStart := align(now, swc.bucketNanos)
	nextBucketStart := currentBucketStart + swc.bucketNanos
	retryAfter := nextBucketStart - now

	// Defensive: ensure non-negative
	if retryAfter < 0 {
		retryAfter = 0
	}

	return model.Reject(retryAfter), nil
}

// roll advances the bucket window to the current time.
//
// This method must be called with swc.mu held.
//
// Algorithm:
// 1. Calculate how many buckets to roll forward
// 2. For each bucket step:
//    - Advance lastBucketStart by bucketNanos
//    - Zero out the count at that bucket index
//    - Subtract from total
// 3. Stop when we reach the current bucket
//
// Optimization: If we've advanced more than numBuckets, just zero everything.
//
// Example:
//   - Current lastBucketStart: 10s
//   - Now: 25s
//   - bucketNanos: 1s
//   - Steps to roll: (25-10)/1 = 15 buckets
//   - If numBuckets=60: roll forward 15 buckets
//   - If numBuckets=10: roll forward 10 buckets (all buckets cleared)
func (swc *SlidingWindowCounter) roll(now int64) {
	currentBucketStart := align(now, swc.bucketNanos)

	// No rolling needed if we're still in the same bucket
	if currentBucketStart == swc.lastBucketStart {
		return
	}

	// Calculate how many bucket steps we need to roll forward
	diff := currentBucketStart - swc.lastBucketStart
	stepsL := diff / swc.bucketNanos
	steps := int(stepsL)

	// Optimization: If we've rolled more than numBuckets, clear everything
	if steps >= swc.numBuckets {
		steps = swc.numBuckets
	}

	// Roll forward step by step
	for i := 0; i < steps; i++ {
		swc.lastBucketStart += swc.bucketNanos
		idx := swc.indexOf(swc.lastBucketStart)

		// Subtract old count and zero out bucket
		swc.total -= swc.counts[idx]
		swc.counts[idx] = 0
	}
}

// indexOf returns the ring buffer index for a given bucket start time.
//
// Uses modulo arithmetic to map bucket timestamps to ring buffer indices.
//
// Formula: (bucketStart / bucketNanos) % numBuckets
//
// This ensures the same bucket start time always maps to the same index,
// and the indices cycle through the ring buffer as time advances.
//
// Example with numBuckets=4, bucketNanos=1s:
//   - bucketStart=0s  → index=(0/1)%4=0
//   - bucketStart=1s  → index=(1/1)%4=1
//   - bucketStart=2s  → index=(2/1)%4=2
//   - bucketStart=3s  → index=(3/1)%4=3
//   - bucketStart=4s  → index=(4/1)%4=0 (wraps around)
//   - bucketStart=5s  → index=(5/1)%4=1
func (swc *SlidingWindowCounter) indexOf(bucketStart int64) int {
	bucketNumber := bucketStart / swc.bucketNanos
	return int(bucketNumber % int64(swc.numBuckets))
}

// align returns the start of the bucket containing the given timestamp.
//
// Truncates the timestamp to the nearest bucket boundary.
//
// Formula: (now / bucketNanos) * bucketNanos
//
// Example with bucketNanos=1_000_000_000 (1 second):
//   - align(500_000_000) = 0
//   - align(1_500_000_000) = 1_000_000_000
//   - align(5_700_000_000) = 5_000_000_000
func align(now, bucketNanos int64) int64 {
	return (now / bucketNanos) * bucketNanos
}
