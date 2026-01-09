package rl.java.engine;

/**
 * Configuration for creating a RateLimiter instance.
 *
 * This record encapsulates all parameters needed to instantiate
 * any of the supported rate limiting algorithms.
 *
 * @param algorithmType The algorithm to use
 * @param capacity Maximum capacity (tokens for TOKEN_BUCKET, limit for windows)
 * @param refillRate Refill rate (tokens/second for TOKEN_BUCKET, N/A for others)
 * @param windowSizeNanos Window size in nanoseconds (for window-based algorithms)
 * @param numBuckets Number of buckets (for SLIDING_WINDOW_COUNTER only)
 */
public record RateLimiterConfig(
    AlgorithmType algorithmType,
    long capacity,
    double refillRate,
    long windowSizeNanos,
    int numBuckets
) {
    /**
     * Creates a Token Bucket configuration.
     *
     * @param capacity Maximum tokens
     * @param refillTokensPerSecond Tokens added per second
     * @return Configuration for Token Bucket
     */
    public static RateLimiterConfig tokenBucket(long capacity, double refillTokensPerSecond) {
        if (capacity <= 0) throw new IllegalArgumentException("capacity must be > 0");
        if (refillTokensPerSecond <= 0) throw new IllegalArgumentException("refillRate must be > 0");

        return new RateLimiterConfig(
            AlgorithmType.TOKEN_BUCKET,
            capacity,
            refillTokensPerSecond,
            0L, // Not used
            0   // Not used
        );
    }

    /**
     * Creates a Fixed Window configuration.
     *
     * @param limit Maximum permits per window
     * @param windowSizeNanos Window size in nanoseconds
     * @return Configuration for Fixed Window
     */
    public static RateLimiterConfig fixedWindow(long limit, long windowSizeNanos) {
        if (limit <= 0) throw new IllegalArgumentException("limit must be > 0");
        if (windowSizeNanos <= 0) throw new IllegalArgumentException("windowSize must be > 0");

        return new RateLimiterConfig(
            AlgorithmType.FIXED_WINDOW,
            limit,
            0.0, // Not used
            windowSizeNanos,
            0    // Not used
        );
    }

    /**
     * Creates a Sliding Window Log configuration.
     *
     * @param limit Maximum permits per window
     * @param windowSizeNanos Window size in nanoseconds
     * @return Configuration for Sliding Window Log
     */
    public static RateLimiterConfig slidingWindowLog(long limit, long windowSizeNanos) {
        if (limit <= 0) throw new IllegalArgumentException("limit must be > 0");
        if (windowSizeNanos <= 0) throw new IllegalArgumentException("windowSize must be > 0");

        return new RateLimiterConfig(
            AlgorithmType.SLIDING_WINDOW_LOG,
            limit,
            0.0, // Not used
            windowSizeNanos,
            0    // Not used
        );
    }

    /**
     * Creates a Sliding Window Counter configuration.
     *
     * @param limit Maximum permits per window
     * @param windowSizeNanos Window size in nanoseconds
     * @param numBuckets Number of sub-buckets (more = better precision, more memory)
     * @return Configuration for Sliding Window Counter
     */
    public static RateLimiterConfig slidingWindowCounter(long limit, long windowSizeNanos, int numBuckets) {
        if (limit <= 0) throw new IllegalArgumentException("limit must be > 0");
        if (windowSizeNanos <= 0) throw new IllegalArgumentException("windowSize must be > 0");
        if (numBuckets <= 0) throw new IllegalArgumentException("numBuckets must be > 0");

        return new RateLimiterConfig(
            AlgorithmType.SLIDING_WINDOW_COUNTER,
            limit,
            0.0, // Not used
            windowSizeNanos,
            numBuckets
        );
    }
}