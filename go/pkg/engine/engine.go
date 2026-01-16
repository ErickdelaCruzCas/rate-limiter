package engine

import (
	"fmt"
	"sync"

	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// Engine provides thread-safe multi-key rate limiting with LRU eviction.
//
// The engine manages rate limiters for multiple keys (e.g., different users, API keys)
// and automatically creates limiters on-demand using the configured algorithm.
// It uses an LRU cache to bound memory usage and prevent resource exhaustion.
//
// # Architecture
//
// Each key gets its own rate limiter instance with its own mutex (fine-grained locking).
// This design allows high concurrency: different keys can be rate-limited in parallel
// without lock contention.
//
//	Key "user:123" → limiterEntry{limiter, mutex} → No lock contention with "user:456"
//	Key "user:456" → limiterEntry{limiter, mutex} → Concurrent execution possible
//
// # Concurrency Model
//
// The engine uses two levels of locking:
//
//  1. LRU cache lock: Protects the cache structure (get/put operations)
//     - Short-lived: Just for cache lookups/insertions
//     - No algorithm operations under this lock
//
//  2. Per-key lock: Each limiter entry has its own mutex
//     - Protects the rate limiter's internal state
//     - Held during TryAcquire operations
//     - Different keys never contend with each other
//
// This design provides:
//   - No global bottleneck
//   - Lock contention only for the same key
//   - Maximum parallelism for different keys
//
// # LRU Eviction
//
// When maxKeys is reached, the least recently used limiter is evicted.
// This prevents unbounded memory growth when rate limiting many keys.
//
// Eviction considerations:
//   - A key's rate limit state is lost when evicted
//   - Next access to the same key creates a fresh limiter (reset state)
//   - Set maxKeys high enough to cover your working set
//   - Monitor eviction rate in production
//
// # Example Usage
//
//	// Create engine with Token Bucket algorithm
//	config := DefaultTokenBucketConfig()
//	engine, err := NewEngine(clock.NewSystemClock(), config, 10000)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Rate limit different keys
//	result, err := engine.TryAcquire("user:123", 1)
//	if result.Decision == model.DecisionAllow {
//	    processRequest()
//	}
//
//	result, err = engine.TryAcquire("user:456", 1)
//	// Concurrent with user:123 - different limiters, different locks
//
// # Thread-Safety
//
// All methods are safe for concurrent use by multiple goroutines.
// The engine is designed for high-concurrency production environments.
type Engine struct {
	clock  clock.Clock
	config Config
	cache  *LRUCache
}

// limiterEntry wraps a rate limiter with its own mutex.
//
// Each key in the cache gets its own limiterEntry, providing fine-grained locking.
// The mutex protects the limiter's internal state during TryAcquire calls.
type limiterEntry struct {
	limiter model.RateLimiter
	mu      sync.Mutex
}

// NewEngine creates a new rate limiting engine.
//
// Parameters:
//   - clk: Clock implementation for time operations
//   - config: Rate limiter configuration (algorithm and parameters)
//   - maxKeys: Maximum number of keys to cache (must be > 0)
//
// Returns:
//   - *Engine: The initialized engine
//   - error: Validation error if parameters are invalid
//
// The engine validates the configuration by attempting to create a rate limiter.
// This ensures the config is valid before the engine starts accepting requests.
//
// Validation:
//   - maxKeys must be > 0
//   - config must be valid for the specified algorithm
//
// Example:
//
//	// Token Bucket: 100 tokens, 10/second, cache up to 10,000 users
//	config := Config{
//	    Algorithm:  AlgorithmTokenBucket,
//	    Capacity:   100,
//	    RefillRate: 10.0,
//	}
//	engine, err := NewEngine(clock.NewSystemClock(), config, 10000)
//	if err != nil {
//	    log.Fatalf("Failed to create engine: %v", err)
//	}
func NewEngine(clk clock.Clock, config Config, maxKeys int) (*Engine, error) {
	// Validation: maxKeys
	if maxKeys <= 0 {
		return nil, fmt.Errorf("maxKeys must be > 0, got: %d", maxKeys)
	}

	// Validation: Verify config by creating a test limiter
	// This catches configuration errors early rather than on first request
	_, err := CreateFromConfig(clk, config)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Engine{
		clock:  clk,
		config: config,
		cache:  NewLRUCache(maxKeys),
	}, nil
}

// TryAcquire attempts to acquire permits for a given key.
//
// If this is the first request for the key, a new rate limiter is created
// using the engine's configuration. Subsequent requests for the same key
// reuse the existing limiter.
//
// When the cache reaches maxKeys capacity, the least recently used key
// is evicted before creating a new limiter.
//
// Parameters:
//   - key: The rate limit key (e.g., "user:123", "api:key:abc", "ip:192.168.1.1")
//   - permits: Number of permits to acquire (must be > 0)
//
// Returns:
//   - model.RateLimitResult: Decision (ALLOW/REJECT) and retry timing
//   - error: Validation error if key is empty or permits <= 0
//
// Thread-safety:
//   - Safe for concurrent use by multiple goroutines
//   - Different keys execute in parallel (no lock contention)
//   - Same key serializes through per-key mutex
//
// Error conditions:
//   - Empty key: Returns error
//   - Invalid permits: Delegated to algorithm (returns error)
//
// Example:
//
//	// Single permit acquisition
//	result, err := engine.TryAcquire("user:123", 1)
//	if err != nil {
//	    log.Printf("Validation error: %v", err)
//	    return
//	}
//	if result.Decision == model.DecisionAllow {
//	    processRequest()
//	} else {
//	    rejectRequest(result.RetryAfterNanos)
//	}
//
//	// Resource-based rate limiting (e.g., bytes)
//	bytesToTransfer := 1024
//	result, err := engine.TryAcquire("transfer:abc", bytesToTransfer)
func (e *Engine) TryAcquire(key string, permits int) (model.RateLimitResult, error) {
	// Validation: key must not be empty
	if key == "" {
		return model.RateLimitResult{}, fmt.Errorf("key must not be empty")
	}

	// Get or create limiter entry for this key
	entry := e.getOrCreateLimiter(key)

	// Lock this specific key's limiter
	// Other keys can proceed in parallel
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Delegate to the rate limiter
	return entry.limiter.TryAcquire(key, permits)
}

// getOrCreateLimiter retrieves or creates a limiter entry for the given key.
//
// This method implements the get-or-create pattern using LRU cache's PutIfAbsent.
// It's thread-safe and ensures exactly one limiter per key.
//
// Algorithm:
//  1. Fast path: Check cache for existing limiter
//  2. Slow path: Create new limiter and insert atomically
//  3. If another goroutine inserted first, use their limiter
//
// Thread-safety:
//   The LRU cache's PutIfAbsent provides atomic semantics, preventing races
//   where multiple goroutines try to create a limiter for the same key.
//
// Returns:
//   *limiterEntry: The entry for this key (existing or newly created)
//
// Note: If limiter creation fails (invalid config), this panics.
// This should never happen because NewEngine validates the config.
// The panic indicates a programming error (config was modified after validation).
func (e *Engine) getOrCreateLimiter(key string) *limiterEntry {
	// Fast path: Check if limiter already exists
	if val, ok := e.cache.Get(key); ok {
		return val.(*limiterEntry)
	}

	// Slow path: Create new limiter
	limiter, err := CreateFromConfig(e.clock, e.config)
	if err != nil {
		// This should never happen - config was validated in NewEngine
		// If it does, it indicates a serious bug (e.g., config was mutated)
		panic(fmt.Sprintf("failed to create limiter (should have been caught by NewEngine): %v", err))
	}

	entry := &limiterEntry{
		limiter: limiter,
	}

	// Atomic insert: PutIfAbsent returns nil if we won, existing value if we lost
	existing := e.cache.PutIfAbsent(key, entry)
	if existing != nil {
		// Another goroutine created the limiter first, use theirs
		return existing.(*limiterEntry)
	}

	// We successfully inserted our entry
	return entry
}

// Len returns the current number of cached rate limiters.
//
// This is useful for monitoring cache usage and understanding
// how many unique keys are being actively rate limited.
//
// Thread-safe: Can be called concurrently.
//
// Example:
//
//	cachedKeys := engine.Len()
//	log.Printf("Currently tracking %d keys", cachedKeys)
//
//	if cachedKeys >= maxKeys * 0.9 {
//	    log.Warn("Cache is 90% full, approaching eviction threshold")
//	}
func (e *Engine) Len() int {
	return e.cache.Len()
}
