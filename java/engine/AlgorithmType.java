package rl.java.engine;

/**
 * Supported rate limiting algorithms.
 *
 * Each enum defines the algorithm characteristics and trade-offs:
 * - TOKEN_BUCKET: Continuous refill, burst-friendly, O(1) time/memory
 * - FIXED_WINDOW: Simple reset, has boundary problem, O(1) time/memory
 * - SLIDING_WINDOW_LOG: Exact precision, no boundary problem, O(permits) memory
 * - SLIDING_WINDOW_COUNTER: Practical approximation, O(buckets) memory
 */
public enum AlgorithmType {
    /**
     * Token Bucket: Continuous refill algorithm.
     * Best for: Most use cases requiring burst capacity.
     * Memory: O(1) per key
     * Precision: Good (continuous refill)
     */
    TOKEN_BUCKET,

    /**
     * Fixed Window: Simple reset at window boundaries.
     * Best for: Simple cases where 2x rate spike is acceptable.
     * Memory: O(1) per key
     * Precision: Poor (boundary problem allows 2x rate at boundaries)
     */
    FIXED_WINDOW,

    /**
     * Sliding Window Log: Exact tracking with timestamp log.
     * Best for: High precision requirements, acceptable memory cost.
     * Memory: O(limit) per key
     * Precision: Exact (no boundary problem)
     */
    SLIDING_WINDOW_LOG,

    /**
     * Sliding Window Counter: Ring buffer approximation.
     * Best for: Balance between precision and memory.
     * Memory: O(buckets) per key
     * Precision: Approximation (better than fixed window)
     */
    SLIDING_WINDOW_COUNTER
}