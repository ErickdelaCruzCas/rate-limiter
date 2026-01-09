package rl.java.engine;

import rl.core.model.RateLimiter;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Entry holding a RateLimiter instance with its associated lock.
 *
 * This class encapsulates:
 * - The rate limiter instance
 * - A ReentrantLock for fine-grained synchronization
 * - Metadata for debugging (optional)
 *
 * Thread-safety:
 * - The lock must be acquired before accessing the rate limiter
 * - This ensures proper synchronization even if the rate limiter itself is not thread-safe
 *
 * Design rationale:
 * - ReentrantLock provides more control than synchronized (tryLock, timeout, fairness)
 * - Per-key locks reduce contention compared to global locking
 * - Bundling lock with limiter simplifies lifecycle management
 */
final class LimiterEntry {

    private final RateLimiter limiter;
    private final ReentrantLock lock;

    /**
     * Creates a new entry.
     *
     * @param limiter The rate limiter instance
     */
    LimiterEntry(RateLimiter limiter) {
        if (limiter == null) {
            throw new IllegalArgumentException("limiter cannot be null");
        }
        this.limiter = limiter;
        this.lock = new ReentrantLock(); // Non-fair for better throughput
    }

    /**
     * Returns the rate limiter instance.
     * MUST be called while holding the lock.
     *
     * @return The rate limiter
     */
    RateLimiter getLimiter() {
        return limiter;
    }

    /**
     * Returns the lock for this entry.
     *
     * @return The lock
     */
    ReentrantLock getLock() {
        return lock;
    }
}
