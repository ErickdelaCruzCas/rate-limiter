package engine

import (
	"fmt"

	"github.com/ratelimiter/go/pkg/algorithm/fixedwindow"
	"github.com/ratelimiter/go/pkg/algorithm/slidingwindow"
	"github.com/ratelimiter/go/pkg/algorithm/slidingwindowcounter"
	"github.com/ratelimiter/go/pkg/algorithm/tokenbucket"
	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// CreateFromConfig creates a rate limiter instance based on the provided configuration.
//
// This factory function abstracts the creation of different algorithm implementations,
// allowing the engine to work with any algorithm type through the RateLimiter interface.
//
// The function validates the configuration for the specific algorithm and returns
// an appropriate error if the configuration is invalid.
//
// Parameters:
//   - clk: Clock implementation for time operations
//   - config: Configuration specifying algorithm and parameters
//
// Returns:
//   - model.RateLimiter: The created rate limiter instance
//   - error: Validation or creation error
//
// Supported algorithms:
//   - TokenBucket: Requires Capacity and RefillRate
//   - FixedWindow: Requires WindowNanos and Limit
//   - SlidingWindowLog: Requires WindowNanos and Limit
//   - SlidingWindowCounter: Requires WindowNanos, BucketNanos, and Limit
//
// Example:
//
//	config := Config{
//	    Algorithm:  AlgorithmTokenBucket,
//	    Capacity:   100,
//	    RefillRate: 10.0,
//	}
//
//	limiter, err := CreateFromConfig(clock.NewSystemClock(), config)
//	if err != nil {
//	    log.Fatalf("Failed to create limiter: %v", err)
//	}
//
//	result, _ := limiter.TryAcquire("user:123", 1)
func CreateFromConfig(clk clock.Clock, config Config) (model.RateLimiter, error) {
	switch config.Algorithm {
	case AlgorithmTokenBucket:
		return createTokenBucket(clk, config)

	case AlgorithmFixedWindow:
		return createFixedWindow(clk, config)

	case AlgorithmSlidingWindowLog:
		return createSlidingWindowLog(clk, config)

	case AlgorithmSlidingWindowCounter:
		return createSlidingWindowCounter(clk, config)

	default:
		return nil, fmt.Errorf("unknown algorithm type: %d", config.Algorithm)
	}
}

// createTokenBucket creates a Token Bucket rate limiter from config.
//
// Required config fields:
//   - Capacity: Maximum tokens (must be > 0)
//   - RefillRate: Tokens per second (must be > 0)
//
// Returns error if:
//   - Capacity <= 0
//   - RefillRate <= 0
//   - Clock is nil (will panic in algorithm constructor)
func createTokenBucket(clk clock.Clock, config Config) (model.RateLimiter, error) {
	// Validation is delegated to the algorithm constructor
	// This ensures validation logic is centralized in the algorithm package
	limiter, err := tokenbucket.New(clk, config.Capacity, config.RefillRate)
	if err != nil {
		return nil, fmt.Errorf("failed to create TokenBucket: %w", err)
	}
	return limiter, nil
}

// createFixedWindow creates a Fixed Window rate limiter from config.
//
// Required config fields:
//   - WindowNanos: Window duration in nanoseconds (must be > 0)
//   - Limit: Maximum permits per window (must be > 0)
//
// Returns error if:
//   - WindowNanos <= 0
//   - Limit <= 0
func createFixedWindow(clk clock.Clock, config Config) (model.RateLimiter, error) {
	limiter, err := fixedwindow.New(clk, config.WindowNanos, config.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to create FixedWindow: %w", err)
	}
	return limiter, nil
}

// createSlidingWindowLog creates a Sliding Window Log rate limiter from config.
//
// Required config fields:
//   - WindowNanos: Window duration in nanoseconds (must be > 0)
//   - Limit: Maximum events in window (must be > 0)
//
// Returns error if:
//   - WindowNanos <= 0
//   - Limit <= 0
//
// Note: This algorithm uses O(limit) memory per key.
// For high limits (> 10,000), consider SlidingWindowCounter instead.
func createSlidingWindowLog(clk clock.Clock, config Config) (model.RateLimiter, error) {
	limiter, err := slidingwindow.New(clk, config.WindowNanos, config.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to create SlidingWindowLog: %w", err)
	}
	return limiter, nil
}

// createSlidingWindowCounter creates a Sliding Window Counter rate limiter from config.
//
// Required config fields:
//   - WindowNanos: Total window duration (must be > 0)
//   - BucketNanos: Bucket duration (must be > 0)
//   - Limit: Maximum permits in window (must be > 0)
//
// Validation:
//   - WindowNanos must be divisible by BucketNanos
//
// Returns error if:
//   - WindowNanos <= 0
//   - BucketNanos <= 0
//   - WindowNanos % BucketNanos != 0
//   - Limit <= 0
//
// Bucket count = WindowNanos / BucketNanos
// Memory usage: O(bucket count) per key
func createSlidingWindowCounter(clk clock.Clock, config Config) (model.RateLimiter, error) {
	limiter, err := slidingwindowcounter.New(clk, config.WindowNanos, config.BucketNanos, config.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to create SlidingWindowCounter: %w", err)
	}
	return limiter, nil
}
