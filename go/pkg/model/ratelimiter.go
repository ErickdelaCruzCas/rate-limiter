package model

// RateLimiter is the core interface for all rate limiting algorithms.
//
// This interface defines the contract that all rate limiting implementations must satisfy,
// enabling polymorphism and allowing the engine layer to work with any algorithm implementation.
//
// All implementations must be safe for concurrent use by multiple goroutines.
// Implementations should use the injected Clock interface for all time operations
// to enable deterministic testing with ManualClock.
//
// Design principles:
// - Stateless interface: No shared state across different keys
// - Per-key isolation: Implementations must maintain separate state per key
// - Thread-safe: All methods must be safe for concurrent goroutine access
// - Clock-injectable: All time operations must use the injected Clock
//
// Implementations:
// - Token Bucket: Continuous refill, burst-friendly (pkg/algorithm/tokenbucket)
// - Fixed Window: Simple counter, has boundary problem (pkg/algorithm/fixedwindow)
// - Sliding Window Log: Exact precision, O(limit) memory (pkg/algorithm/slidingwindow)
// - Sliding Window Counter: Approximate, balanced (pkg/algorithm/slidingwindowcounter)
type RateLimiter interface {
	// TryAcquire attempts to acquire the specified number of permits for a key.
	//
	// This method checks if the request is allowed under the rate limit and,
	// if so, consumes the specified number of permits. The method returns
	// immediately without blocking.
	//
	// Parameters:
	//   - key: The rate limit key (e.g., "user:123", "api:key:abc", "ip:192.168.1.1")
	//          Keys are arbitrary strings that identify the entity being rate limited.
	//          Different keys maintain independent rate limit state.
	//
	//   - permits: Number of permits to acquire (must be > 0)
	//              For simple request counting, use permits=1.
	//              For resource-based limiting, use permits=resourceCost (e.g., bytes, tokens).
	//
	// Returns:
	//   - RateLimitResult: Contains the decision (ALLOW/REJECT) and retry timing
	//   - error: Validation error if parameters are invalid (e.g., permits <= 0, empty key)
	//
	// Behavior:
	//   - If allowed: Consumes permits and returns Allow() with RetryAfterNanos=0
	//   - If rejected: Does not consume permits, returns Reject() with RetryAfterNanos>0
	//   - If invalid: Returns error without modifying state
	//
	// Thread-safety:
	//   This method must be safe to call concurrently from multiple goroutines,
	//   even with the same key. Implementations must use appropriate synchronization
	//   (typically sync.Mutex) to protect shared state.
	//
	// Error conditions:
	//   - permits <= 0: Returns error (validation failure)
	//   - key is empty string: Behavior is implementation-defined (may error or allow)
	//   - Capacity/limit issues: Some implementations may error if permits > capacity
	//
	// Example usage:
	//
	//	limiter := tokenbucket.New(clock, 100, 10.0)
	//
	//	// Simple request rate limiting
	//	result, err := limiter.TryAcquire("user:123", 1)
	//	if err != nil {
	//	    return fmt.Errorf("validation error: %w", err)
	//	}
	//	if result.Decision == model.DecisionReject {
	//	    return fmt.Errorf("rate limited, retry after %d ns", result.RetryAfterNanos)
	//	}
	//	// Process request...
	//
	//	// Resource-based rate limiting (e.g., bytes)
	//	bytesToTransfer := 1024
	//	result, err := limiter.TryAcquire("transfer:abc", bytesToTransfer)
	//	if result.Decision == model.DecisionAllow {
	//	    // Transfer bytes...
	//	}
	//
	// Algorithm-specific notes:
	//
	// Token Bucket:
	//   - Allows bursts up to capacity
	//   - Continuous refill (smooth rate limiting)
	//   - RetryAfter = time to refill missing tokens
	//
	// Fixed Window:
	//   - Resets counter at window boundaries
	//   - Boundary problem: Can allow 2x limit at window edges
	//   - RetryAfter = time to next window
	//
	// Sliding Window Log:
	//   - Exact precision, no approximation
	//   - O(limit) memory per key (stores all event timestamps)
	//   - RetryAfter = time until oldest event expires
	//
	// Sliding Window Counter:
	//   - Approximate (bucket granularity)
	//   - O(numBuckets) memory per key
	//   - RetryAfter = time to next bucket boundary
	TryAcquire(key string, permits int) (RateLimitResult, error)
}
