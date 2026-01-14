package rl.java.engine;

import rl.core.clock.Clock;
import rl.core.model.RateLimitResult;

import java.util.concurrent.locks.ReentrantLock;

/**
 * Thread-safe rate limiter engine with multi-key support.
 *
 * Features:
 * - ConcurrentHashMap-based storage for multiple keys (users, APIs, etc.)
 * - ReentrantLock per key for fine-grained synchronization
 * - LRU eviction policy to prevent unbounded memory growth
 * - Configurable algorithm per key via factory pattern
 * - High throughput design (target: >100K req/s single-threaded, >500K multi-threaded)
 *
 * Architecture:
 * - LRUCache stores LimiterEntry (RateLimiter + ReentrantLock)
 * - Per-key locks eliminate global bottleneck
 * - Clock injection enables deterministic testing
 * - Factory pattern allows flexible algorithm selection
 *
 * Thread-safety:
 * - LRUCache is internally synchronized for safe get/put
 * - Each limiter has its own ReentrantLock for concurrent access
 * - Eviction callback ensures proper lock cleanup
 *
 * Memory management:
 * - LRU eviction prevents unbounded growth
 * - Least recently used keys are evicted when maxKeys is reached
 * - Evicted entries release locks automatically
 *
 * Usage example:
 * <pre>
 * Clock clock = new SystemClock();
 * RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 10.0);
 * RateLimiterEngine engine = new RateLimiterEngine(clock, config, 10000);
 *
 * RateLimitResult result = engine.tryAcquire("user:123", 1);
 * if (result.decision() == Decision.ALLOW) {
 *     // Process request
 * } else {
 *     // Reject with retry-after: result.retryAfterNanos()
 * }
 * </pre>
 */
public final class RateLimiterEngine {

    private final Clock clock;
    private final RateLimiterConfig defaultConfig;
    private final LRUCache<String, LimiterEntry> limiters;

    /**
     * Creates a new rate limiter engine.
     *
     * @param clock Clock instance for time control (injected for testability)
     * @param defaultConfig Default configuration for new keys
     * @param maxKeys Maximum number of keys to track (LRU eviction beyond this)
     * @throws IllegalArgumentException if any parameter is invalid
     */
    public RateLimiterEngine(Clock clock, RateLimiterConfig defaultConfig, int maxKeys) {
        if (clock == null) {
            throw new IllegalArgumentException("clock cannot be null");
        }
        if (defaultConfig == null) {
            throw new IllegalArgumentException("defaultConfig cannot be null");
        }
        if (maxKeys <= 0) {
            throw new IllegalArgumentException("maxKeys must be > 0");
        }

        this.clock = clock;
        this.defaultConfig = defaultConfig;

        // LRU cache with eviction callback (no special cleanup needed currently)
        this.limiters = new LRUCache<>(maxKeys, (key, entry) -> {
            // Eviction callback: could log, emit metrics, etc.
            // Lock cleanup is automatic when entry is GC'd
        });
    }

    /**
     * Attempts to acquire permits for a key.
     *
     * This method:
     * 1. Retrieves or creates a rate limiter for the key
     * 2. Acquires the per-key lock
     * 3. Executes tryAcquire on the limiter
     * 4. Releases the lock
     *
     * Thread-safety: Safe for concurrent access from multiple threads.
     * Performance: Lock contention only occurs for the same key.
     *
     * @param key The key to rate limit (e.g., user ID, API key)
     * @param permits Number of permits to acquire (must be > 0)
     * @return RateLimitResult indicating ALLOW/REJECT with retry-after
     * @throws IllegalArgumentException if key is null or permits <= 0
     */
    public RateLimitResult tryAcquire(String key, int permits) {
        if (key == null) {
            throw new IllegalArgumentException("key cannot be null");
        }
        if (permits <= 0) {
            throw new IllegalArgumentException("permits must be > 0");
        }

        // Get or create limiter entry
        LimiterEntry entry = getOrCreateLimiter(key);

        // Acquire lock for this key
        ReentrantLock lock = entry.getLock();
        lock.lock();
        try {
            // Execute rate limit check
            return entry.getLimiter().tryAcquire(key, permits);
        } finally {
            lock.unlock();
        }
    }

    /**
     * Retrieves or creates a limiter entry for a key.
     *
     * This method uses putIfAbsent semantics to prevent race conditions:
     * 1. Check if entry exists (fast path)
     * 2. If not, atomically insert new entry (synchronized via LRUCache.putIfAbsent)
     *
     * Thread-safety: The putIfAbsent operation is atomic, ensuring that only one
     * LimiterEntry is created per key, preventing the race condition where multiple
     * threads could create different lock instances for the same key.
     *
     * @param key The key
     * @return LimiterEntry for the key (never null)
     */
    private LimiterEntry getOrCreateLimiter(String key) {
        // Fast path: limiter already exists
        LimiterEntry entry = limiters.get(key);
        if (entry != null) {
            return entry;
        }

        // Slow path: create new limiter and insert atomically
        LimiterEntry newEntry = new LimiterEntry(
            RateLimiterFactory.create(clock, defaultConfig)
        );

        // Atomic putIfAbsent: returns existing if present, or null if inserted
        LimiterEntry existing = limiters.putIfAbsent(key, newEntry);

        // Return existing entry if another thread created it first, otherwise return new entry
        return (existing != null) ? existing : newEntry;
    }

    /**
     * Returns the number of currently tracked keys.
     *
     * @return Number of keys in cache
     */
    public int size() {
        return limiters.size();
    }

    /**
     * Returns the maximum number of keys that can be tracked.
     *
     * @return Maximum capacity
     */
    public int maxSize() {
        return limiters.maxSize();
    }

    /**
     * Clears all rate limiters from the engine.
     * This is primarily useful for testing.
     */
    public void clear() {
        limiters.clear();
    }

    /**
     * Returns the default configuration used for new keys.
     *
     * @return The default configuration
     */
    public RateLimiterConfig getDefaultConfig() {
        return defaultConfig;
    }
}
