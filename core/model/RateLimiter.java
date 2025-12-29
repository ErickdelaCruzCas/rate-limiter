package rl.core.model;

/**
 * Pure core contract: no I/O, no threads.
 * "key" is included for future distributed/tenant designs, but phase 1 keeps it simple.
 */
public interface RateLimiter {
    RateLimitResult tryAcquire(String key, int permits);
}
