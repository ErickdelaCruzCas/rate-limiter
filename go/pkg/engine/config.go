package engine

// AlgorithmType represents the rate limiting algorithm to use.
//
// The engine supports all four core algorithms, each with different
// characteristics and trade-offs. Choose based on your requirements:
//
// - TokenBucket: Best default choice, allows bursts, continuous refill
// - FixedWindow: Simplest, but has boundary problem
// - SlidingWindowLog: Most precise, but higher memory cost
// - SlidingWindowCounter: Balanced, good for production
type AlgorithmType int

const (
	// AlgorithmTokenBucket uses the Token Bucket algorithm.
	//
	// Characteristics:
	//   - O(1) memory per key
	//   - Allows bursts up to capacity
	//   - Continuous token refill
	//   - Good for most use cases
	//
	// Use when:
	//   - You want burst tolerance
	//   - You need smooth rate limiting
	//   - Memory efficiency is important
	//
	// Configuration required:
	//   - Capacity: Maximum tokens
	//   - RefillRate: Tokens per second
	AlgorithmTokenBucket AlgorithmType = iota

	// AlgorithmFixedWindow uses the Fixed Window algorithm.
	//
	// Characteristics:
	//   - O(1) memory per key
	//   - Simple counter-based
	//   - Has boundary problem (can allow 2x at window edges)
	//   - Resets at window boundaries
	//
	// Use when:
	//   - Simplicity is paramount
	//   - Boundary problem is acceptable
	//   - You need strict per-period quotas
	//
	// Configuration required:
	//   - WindowNanos: Window duration
	//   - Limit: Max permits per window
	AlgorithmFixedWindow

	// AlgorithmSlidingWindowLog uses the Sliding Window Log algorithm.
	//
	// Characteristics:
	//   - O(limit) memory per key
	//   - Exact precision, no approximation
	//   - No boundary problem
	//   - Stores all event timestamps
	//
	// Use when:
	//   - Exact precision is critical
	//   - Memory cost is acceptable
	//   - Limit is moderate (< 10,000)
	//
	// Configuration required:
	//   - WindowNanos: Window duration
	//   - Limit: Max events in window
	AlgorithmSlidingWindowLog

	// AlgorithmSlidingWindowCounter uses the Sliding Window Counter algorithm.
	//
	// Characteristics:
	//   - O(numBuckets) memory per key
	//   - Approximate (bucket granularity)
	//   - Better than Fixed Window, more efficient than Sliding Log
	//   - Good production choice
	//
	// Use when:
	//   - You need balance between precision and efficiency
	//   - Sliding Log's memory cost is too high
	//   - Fixed Window's boundary problem is unacceptable
	//
	// Configuration required:
	//   - WindowNanos: Total window duration
	//   - BucketNanos: Bucket size (windowNanos must be divisible)
	//   - Limit: Max permits in window
	AlgorithmSlidingWindowCounter
)

// String returns the human-readable name of the algorithm.
func (a AlgorithmType) String() string {
	switch a {
	case AlgorithmTokenBucket:
		return "TokenBucket"
	case AlgorithmFixedWindow:
		return "FixedWindow"
	case AlgorithmSlidingWindowLog:
		return "SlidingWindowLog"
	case AlgorithmSlidingWindowCounter:
		return "SlidingWindowCounter"
	default:
		return "Unknown"
	}
}

// Config holds the configuration for creating a rate limiter.
//
// Different algorithms use different fields from this config.
// Unused fields for a particular algorithm are ignored.
//
// Field usage by algorithm:
//
//	TokenBucket:
//	  - Capacity (required)
//	  - RefillRate (required)
//
//	FixedWindow:
//	  - WindowNanos (required)
//	  - Limit (required)
//
//	SlidingWindowLog:
//	  - WindowNanos (required)
//	  - Limit (required)
//
//	SlidingWindowCounter:
//	  - WindowNanos (required)
//	  - BucketNanos (required)
//	  - Limit (required)
//
// Example configurations:
//
//	// Token Bucket: 100 tokens, refill 10/second
//	config := Config{
//	    Algorithm:  AlgorithmTokenBucket,
//	    Capacity:   100,
//	    RefillRate: 10.0,
//	}
//
//	// Fixed Window: 1000 requests per minute
//	config := Config{
//	    Algorithm:   AlgorithmFixedWindow,
//	    WindowNanos: 60_000_000_000, // 60 seconds
//	    Limit:       1000,
//	}
//
//	// Sliding Window Log: 100 requests per 10 seconds
//	config := Config{
//	    Algorithm:   AlgorithmSlidingWindowLog,
//	    WindowNanos: 10_000_000_000, // 10 seconds
//	    Limit:       100,
//	}
//
//	// Sliding Window Counter: 1000 requests per minute, 60 one-second buckets
//	config := Config{
//	    Algorithm:   AlgorithmSlidingWindowCounter,
//	    WindowNanos: 60_000_000_000,  // 60 seconds
//	    BucketNanos: 1_000_000_000,   // 1 second
//	    Limit:       1000,
//	}
type Config struct {
	// Algorithm specifies which rate limiting algorithm to use.
	Algorithm AlgorithmType

	// Capacity is the maximum tokens for Token Bucket (ignored by other algorithms).
	// Must be > 0 for Token Bucket.
	Capacity int64

	// RefillRate is the tokens per second for Token Bucket (ignored by other algorithms).
	// Must be > 0 for Token Bucket.
	// Fractional values supported (e.g., 0.5 = 1 token every 2 seconds).
	RefillRate float64

	// WindowNanos is the window duration in nanoseconds.
	// Used by: FixedWindow, SlidingWindowLog, SlidingWindowCounter
	// Must be > 0.
	WindowNanos int64

	// BucketNanos is the bucket duration in nanoseconds.
	// Used by: SlidingWindowCounter only
	// Must be > 0 and WindowNanos must be divisible by BucketNanos.
	BucketNanos int64

	// Limit is the maximum permits per window.
	// Used by: FixedWindow, SlidingWindowLog, SlidingWindowCounter
	// Must be > 0.
	Limit int
}

// DefaultTokenBucketConfig returns a sensible default Token Bucket configuration.
//
// Configuration:
//   - Capacity: 100 tokens
//   - RefillRate: 10 tokens/second
//
// This allows bursts of up to 100 requests with a sustained rate of 10 req/s.
//
// Example:
//
//	config := DefaultTokenBucketConfig()
//	engine, _ := NewEngine(clock, config, 10000)
func DefaultTokenBucketConfig() Config {
	return Config{
		Algorithm:  AlgorithmTokenBucket,
		Capacity:   100,
		RefillRate: 10.0,
	}
}

// DefaultFixedWindowConfig returns a sensible default Fixed Window configuration.
//
// Configuration:
//   - WindowNanos: 1 second
//   - Limit: 100 requests/second
//
// Example:
//
//	config := DefaultFixedWindowConfig()
//	engine, _ := NewEngine(clock, config, 10000)
func DefaultFixedWindowConfig() Config {
	return Config{
		Algorithm:   AlgorithmFixedWindow,
		WindowNanos: 1_000_000_000, // 1 second
		Limit:       100,
	}
}

// DefaultSlidingWindowLogConfig returns a sensible default Sliding Window Log configuration.
//
// Configuration:
//   - WindowNanos: 10 seconds
//   - Limit: 100 requests per 10 seconds
//
// Example:
//
//	config := DefaultSlidingWindowLogConfig()
//	engine, _ := NewEngine(clock, config, 10000)
func DefaultSlidingWindowLogConfig() Config {
	return Config{
		Algorithm:   AlgorithmSlidingWindowLog,
		WindowNanos: 10_000_000_000, // 10 seconds
		Limit:       100,
	}
}

// DefaultSlidingWindowCounterConfig returns a sensible default Sliding Window Counter configuration.
//
// Configuration:
//   - WindowNanos: 60 seconds
//   - BucketNanos: 1 second (60 buckets)
//   - Limit: 100 requests per minute
//
// Example:
//
//	config := DefaultSlidingWindowCounterConfig()
//	engine, _ := NewEngine(clock, config, 10000)
func DefaultSlidingWindowCounterConfig() Config {
	return Config{
		Algorithm:   AlgorithmSlidingWindowCounter,
		WindowNanos: 60_000_000_000, // 60 seconds
		BucketNanos: 1_000_000_000,  // 1 second
		Limit:       100,
	}
}
