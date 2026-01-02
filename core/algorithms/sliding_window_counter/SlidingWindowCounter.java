package rl.core.algorithms.sliding_window_counter;

import rl.core.clock.Clock;
import rl.core.model.RateLimitResult;
import rl.core.model.RateLimiter;

/**
 * Sliding Window Counter (aprox):
 * - divide la ventana en buckets fijos (granularityNanos)
 * - mantiene un ring buffer de counts
 *
 * Pros: estado acotado, buen rendimiento.
 * Contras: aproximación; elegir granularidad es una decisión (precisión vs coste).
 *
 * Thread-safety: synchronized for concurrent access.
 */
public final class SlidingWindowCounter implements RateLimiter {
    private final Clock clock;
    private final long windowNanos;
    private final long bucketNanos;
    private final int buckets;
    private final int limit;

    private final int[] counts;
    private long lastBucketStart;
    private int total;

    public SlidingWindowCounter(Clock clock, long windowNanos, long bucketNanos, int limit) {
        if (windowNanos <= 0) throw new IllegalArgumentException("window <= 0");
        if (bucketNanos <= 0) throw new IllegalArgumentException("bucket <= 0");
        if (windowNanos % bucketNanos != 0) throw new IllegalArgumentException("window % bucket != 0");
        if (limit <= 0) throw new IllegalArgumentException("limit <= 0");

        this.clock = clock;
        this.windowNanos = windowNanos;
        this.bucketNanos = bucketNanos;
        this.buckets = (int) (windowNanos / bucketNanos);
        this.limit = limit;

        this.counts = new int[buckets];
        long now = clock.nowNanos();
        this.lastBucketStart = align(now);
        this.total = 0;
    }

    @Override
    public synchronized RateLimitResult tryAcquire(String key, int permits) {
        if (permits <= 0) throw new IllegalArgumentException("permits <= 0");
        long now = clock.nowNanos();
        roll(now);

        if (total + permits <= limit) {
            int idx = indexOf(align(now));
            counts[idx] += permits;
            total += permits;
            return RateLimitResult.allow();
        }

        // Aproximación simple: retry cuando avance al siguiente bucket
        long nextBucket = align(now) + bucketNanos;
        return RateLimitResult.reject(nextBucket - now);
    }

    private void roll(long now) {
        long current = align(now);
        if (current == lastBucketStart) return;

        long diff = current - lastBucketStart;
        long stepsL = diff / bucketNanos;
        int steps = (int) Math.min(stepsL, buckets);

        // avanzamos bucket a bucket limpiando los que salen de la ventana
        for (int i = 0; i < steps; i++) {
            lastBucketStart += bucketNanos;
            int idx = indexOf(lastBucketStart);
            total -= counts[idx];
            counts[idx] = 0;
        }
    }

    private long align(long now) {
        return (now / bucketNanos) * bucketNanos;
    }

    private int indexOf(long bucketStart) {
        long bucketNumber = bucketStart / bucketNanos;
        return (int) (bucketNumber % buckets);
    }
}
