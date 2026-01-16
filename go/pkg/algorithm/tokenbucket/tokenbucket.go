// Package tokenbucket implements the Token Bucket rate limiting algorithm.
//
// # Algorithm Overview
//
// The Token Bucket algorithm maintains a bucket that holds tokens. Tokens are added
// to the bucket at a constant rate (refill rate) and accumulate up to a maximum
// capacity. Each request consumes one or more tokens from the bucket.
//
// Key characteristics:
//   - Allows bursts up to the bucket capacity
//   - Smooth, continuous token refill (not discrete)
//   - O(1) memory per instance
//   - O(1) time complexity per request
//   - Floating-point arithmetic for precise refill calculation
//
// # Algorithm Trade-offs
//
// Pros:
//   - Burst-friendly: Allows temporary traffic spikes up to capacity
//   - Smooth rate limiting: Continuous refill provides better distribution
//   - Simple state: Only tracks token count and last refill time
//   - Predictable: Easy to reason about behavior and capacity planning
//
// Cons:
//   - Requires floating-point arithmetic (potential precision issues)
//   - Not suitable for strict per-second quotas (use Fixed Window instead)
//   - Per-instance state makes distribution across nodes complex
//
// # When to Use
//
// Token Bucket is the recommended default algorithm for most use cases:
//   - API rate limiting with burst tolerance
//   - Request throttling for downstream services
//   - Resource consumption limiting (bytes, compute credits, etc.)
//   - Any scenario where occasional bursts are acceptable
//
// Avoid Token Bucket when:
//   - You need strict per-period quotas (use Fixed Window)
//   - You need exact request tracking (use Sliding Window Log)
//   - You cannot tolerate bursts (use Sliding Window Counter with small buckets)
//
// # Concurrency Model
//
// This implementation uses sync.Mutex for thread-safety. All operations
// (refill, check, consume) modify shared state, so a simple mutex provides
// the necessary synchronization without complexity.
//
// Design rationale:
//   - sync.RWMutex not used: All operations are writes (modify tokens)
//   - Channels not used: Protecting shared state, not communication
//   - Atomic operations not used: Multiple fields must update atomically
//
// # Example Usage
//
//	// Production: 100 tokens capacity, 10 tokens/second refill
//	clk := clock.NewSystemClock()
//	tb, err := tokenbucket.New(clk, 100, 10.0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check rate limit
//	result, err := tb.TryAcquire("user:123", 5)
//	if err != nil {
//	    log.Printf("Validation error: %v", err)
//	    return
//	}
//
//	if result.Decision == model.DecisionAllow {
//	    // Process request - tokens have been consumed
//	    processRequest()
//	} else {
//	    // Reject request
//	    retryAfter := time.Duration(result.RetryAfterNanos)
//	    log.Printf("Rate limited, retry after %v", retryAfter)
//	}
//
// # Testing with ManualClock
//
//	// Deterministic testing: control time progression
//	clk := clock.NewManualClock(0)
//	tb, _ := tokenbucket.New(clk, 10, 1.0) // 10 capacity, 1 token/sec
//
//	// Consume all tokens
//	tb.TryAcquire("user1", 10) // ALLOW
//
//	// Next request rejected
//	result, _ := tb.TryAcquire("user1", 1) // REJECT
//	// result.RetryAfterNanos = 1_000_000_000 (1 second)
//
//	// Advance time to refill tokens
//	clk.AdvanceNanos(5_000_000_000) // Advance 5 seconds
//
//	// Now 5 tokens have refilled
//	tb.TryAcquire("user1", 5) // ALLOW
//
// # Implementation Details
//
// Refill calculation:
//
//	elapsed = now - lastNanos
//	tokensToAdd = elapsed * refillPerNanos
//	tokens = min(capacity, tokens + tokensToAdd)
//
// Retry-after calculation:
//
//	missing = permits - tokens
//	retryAfter = ceil(missing / refillPerNanos)
//
// The implementation uses float64 for tokens to handle fractional token accumulation
// between requests. This provides smooth, continuous refill behavior rather than
// discrete token increments.
package tokenbucket

import (
	"fmt"
	"math"
	"sync"

	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// TokenBucket implements the Token Bucket rate limiting algorithm.
//
// Thread-safe: All exported methods are safe for concurrent use by multiple goroutines.
//
// Fields:
//   - clock: Time source (injected for deterministic testing)
//   - capacity: Maximum tokens that can accumulate
//   - refillPerNanos: Tokens added per nanosecond (computed from refillTokensPerSecond)
//   - tokens: Current token count (float64 for fractional precision)
//   - lastNanos: Last time tokens were refilled (monotonic nanos)
//   - mu: Mutex protecting mutable fields (tokens, lastNanos)
type TokenBucket struct {
	// Immutable fields (set once in constructor, never modified)
	clock          clock.Clock
	capacity       int64
	refillPerNanos float64

	// Mutable fields (protected by mu)
	mu        sync.Mutex
	tokens    float64
	lastNanos int64
}

// New creates a new TokenBucket with the specified configuration.
//
// Parameters:
//
//   - clk: Clock implementation for time operations
//     Use clock.NewSystemClock() for production
//     Use clock.NewManualClock() for deterministic testing
//
//   - capacity: Maximum tokens that can accumulate in the bucket (must be > 0)
//     This also determines the maximum burst size.
//     Example: capacity=100 allows up to 100 requests in a burst.
//
//   - refillTokensPerSecond: Rate at which tokens are added to the bucket (must be > 0)
//     Fractional values are supported for sub-second rates.
//     Example: 10.0 = 10 tokens/second, 0.5 = 1 token every 2 seconds
//
// Returns:
//   - *TokenBucket: Initialized bucket starting at full capacity
//   - error: Validation error if parameters are invalid
//
// Validation:
//   - capacity must be > 0
//   - refillTokensPerSecond must be > 0
//   - clk must not be nil (will panic if nil)
//
// The bucket starts at full capacity, allowing immediate bursts.
// To start with an empty bucket, call TryAcquire() with capacity permits.
//
// Example:
//
//	// 100 tokens max, refill at 10 tokens/second
//	tb, err := tokenbucket.New(clock.NewSystemClock(), 100, 10.0)
//
//	// 1000 tokens max, refill at 0.5 tokens/second (1 token every 2 seconds)
//	tb, err := tokenbucket.New(clock.NewSystemClock(), 1000, 0.5)
func New(clk clock.Clock, capacity int64, refillTokensPerSecond float64) (*TokenBucket, error) {
	// Validation: capacity
	if capacity <= 0 {
		return nil, fmt.Errorf("capacity must be > 0, got: %d", capacity)
	}

	// Validation: refill rate
	if refillTokensPerSecond <= 0 {
		return nil, fmt.Errorf("refillTokensPerSecond must be > 0, got: %f", refillTokensPerSecond)
	}

	// Convert refill rate from tokens/second to tokens/nanosecond
	// This avoids repeated division in the hot path (TryAcquire)
	refillPerNanos := refillTokensPerSecond / 1_000_000_000.0

	return &TokenBucket{
		clock:          clk,
		capacity:       capacity,
		refillPerNanos: refillPerNanos,
		tokens:         float64(capacity), // Start at full capacity
		lastNanos:      clk.NowNanos(),
	}, nil
}

// TryAcquire attempts to acquire the specified number of permits.
//
// This method is the core rate limiting operation:
// 1. Refill tokens based on elapsed time since last refill
// 2. Check if enough tokens are available
// 3. If yes: consume tokens and return ALLOW
// 4. If no: calculate retry-after time and return REJECT
//
// Parameters:
//   - key: Rate limit key (currently unused in single-instance impl, used by engine layer)
//   - permits: Number of tokens to acquire (must be > 0)
//
// Returns:
//   - model.RateLimitResult: Decision and retry timing
//   - error: Validation error if permits <= 0
//
// Thread-safety:
//
//	Protected by internal mutex. Multiple goroutines can call this concurrently.
//
// Behavior:
//   - If tokens >= permits: Consumes permits, returns Allow()
//   - If tokens < permits: Returns Reject() with retry-after time
//   - Does not block or wait for tokens to refill
//
// Retry-after calculation:
//
//	The retry-after value indicates when enough tokens will refill to satisfy
//	the request, assuming no other requests consume tokens in the meantime.
//
//	Formula: retryAfter = ceil((permits - tokens) / refillPerNanos)
//
//	Note: Waiting exactly retryAfter does not guarantee success, as other
//	concurrent requests may consume the refilled tokens.
//
// Example:
//
//	tb, _ := tokenbucket.New(clock, 100, 10.0)
//
//	// Request 5 tokens
//	result, err := tb.TryAcquire("user:123", 5)
//	if result.Decision == model.DecisionAllow {
//	    // Success: 5 tokens consumed, 95 remaining
//	}
//
//	// Request more than available
//	result, err = tb.TryAcquire("user:123", 200)
//	if result.Decision == model.DecisionReject {
//	    // Rejected: retry after result.RetryAfterNanos
//	    // retryAfter = ceil((200 - 95) / (10.0 / 1e9)) = 10.5 seconds
//	}
func (tb *TokenBucket) TryAcquire(key string, permits int) (model.RateLimitResult, error) {
	// Validation: permits must be positive
	if permits <= 0 {
		return model.RateLimitResult{}, fmt.Errorf("permits must be > 0, got: %d", permits)
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Step 1: Refill tokens based on elapsed time
	tb.refill()

	// Step 2: Check if enough tokens are available
	if tb.tokens >= float64(permits) {
		// Success: consume tokens and allow request
		tb.tokens -= float64(permits)
		return model.Allow(), nil
	}

	// Step 3: Insufficient tokens - calculate retry-after time
	// How many tokens are we short?
	missing := float64(permits) - tb.tokens

	// How long until those tokens refill?
	// Use ceiling to ensure we wait long enough (err on the side of caution)
	retryAfter := int64(math.Ceil(missing / tb.refillPerNanos))

	return model.Reject(retryAfter), nil
}

// refill recalculates the current token count based on elapsed time.
//
// This method must be called with tb.mu held (enforced by caller, not checked).
//
// Algorithm:
// 1. Calculate elapsed time since last refill
// 2. Calculate tokens to add: elapsed * refillPerNanos
// 3. Add tokens, capping at capacity
// 4. Update lastNanos to current time
//
// Edge cases:
//   - elapsed <= 0: No-op (clock hasn't advanced or went backward)
//   - tokens + refilled > capacity: Cap at capacity
//   - Fractional tokens: Preserved in float64 for smooth refill
//
// Why float64 for tokens?
//
//	Using float64 allows fractional token accumulation, providing smooth
//	continuous refill behavior. For example, with refillRate=10.5 tokens/sec:
//	  - After 0.1 seconds: +1.05 tokens
//	  - After 0.2 seconds: +2.10 tokens
//	If we used int64, we'd lose the fractional accumulation and introduce jitter.
//
// Monotonicity assumption:
//
//	This implementation assumes the Clock is monotonic (time never goes backward).
//	If elapsed < 0 (non-monotonic clock), we treat it as 0 (no refill).
//	This defensive approach prevents negative token counts from clock skew.
func (tb *TokenBucket) refill() {
	now := tb.clock.NowNanos()
	elapsed := now - tb.lastNanos

	// Handle non-monotonic clocks or zero elapsed time
	if elapsed <= 0 {
		return
	}

	// Calculate tokens to add based on elapsed time
	tokensToAdd := float64(elapsed) * tb.refillPerNanos

	// Add tokens, capping at capacity
	// Using math.Min ensures we never exceed capacity, even with floating-point errors
	tb.tokens = math.Min(float64(tb.capacity), tb.tokens+tokensToAdd)

	// Update last refill time to current time
	tb.lastNanos = now
}
