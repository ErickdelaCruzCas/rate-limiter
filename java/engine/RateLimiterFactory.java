package rl.java.engine;

import rl.core.clock.Clock;
import rl.core.model.RateLimiter;
import rl.core.algorithms.token_bucket.TokenBucket;
import rl.core.algorithms.fixed_window.FixedWindow;
import rl.core.algorithms.sliding_window.SlidingWindowLog;
import rl.core.algorithms.sliding_window_counter.SlidingWindowCounter;

/**
 * Factory for creating RateLimiter instances based on configuration.
 *
 * This factory encapsulates the instantiation logic for all supported
 * rate limiting algorithms. It ensures type-safe creation with proper
 * parameter validation.
 *
 * Thread-safety: This class is stateless and thread-safe.
 */
public final class RateLimiterFactory {

    private RateLimiterFactory() {
        // Utility class, no instantiation
    }

    /**
     * Creates a RateLimiter instance based on the provided configuration.
     *
     * @param clock Clock instance for time control (injected for testability)
     * @param config Configuration specifying algorithm and parameters
     * @return A new RateLimiter instance
     * @throws IllegalArgumentException if configuration is invalid
     * @throws IllegalStateException if algorithm type is not supported
     */
    public static RateLimiter create(Clock clock, RateLimiterConfig config) {
        if (clock == null) throw new IllegalArgumentException("clock cannot be null");
        if (config == null) throw new IllegalArgumentException("config cannot be null");

        return switch (config.algorithmType()) {
            case TOKEN_BUCKET -> new TokenBucket(
                clock,
                config.capacity(),
                config.refillRate()
            );

            case FIXED_WINDOW -> new FixedWindow(
                clock,
                config.windowSizeNanos(),
                (int) config.capacity()
            );

            case SLIDING_WINDOW_LOG -> new SlidingWindowLog(
                clock,
                config.windowSizeNanos(),
                (int) config.capacity()
            );

            case SLIDING_WINDOW_COUNTER -> new SlidingWindowCounter(
                clock,
                config.windowSizeNanos(),
                config.windowSizeNanos() / config.numBuckets(),
                (int) config.capacity()
            );
        };
    }
}