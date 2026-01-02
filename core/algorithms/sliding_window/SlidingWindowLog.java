package rl.core.algorithms.sliding_window;

import rl.core.clock.Clock;
import rl.core.model.RateLimitResult;
import rl.core.model.RateLimiter;

import java.util.ArrayDeque;

/**
 * Exact sliding window (log):
 * guarda timestamps de cada permit.
 *
 * Pros: precisión real.
 * Contras: memoria/GC y coste por request.
 * En distribuido: estado grande por key.
 *
 * Thread-safety: synchronized for concurrent access.
 */
public final class SlidingWindowLog implements RateLimiter {
    private final Clock clock;
    private final long windowNanos;
    private final int limit;

    private final ArrayDeque<Long> events = new ArrayDeque<>();

    public SlidingWindowLog(Clock clock, long windowNanos, int limit) {
        if (windowNanos <= 0) throw new IllegalArgumentException("window <= 0");
        if (limit <= 0) throw new IllegalArgumentException("limit <= 0");
        this.clock = clock;
        this.windowNanos = windowNanos;
        this.limit = limit;
    }

    @Override
    public synchronized RateLimitResult tryAcquire(String key, int permits) {
        if (permits <= 0) throw new IllegalArgumentException("permits <= 0");
        long now = clock.nowNanos();
        prune(now);

        if (events.size() + permits <= limit) {
            for (int i = 0; i < permits; i++) events.addLast(now);
            return RateLimitResult.allow();
        }

        long oldest = events.peekFirst();
        long retryAfter = (oldest + windowNanos) - now;
        return RateLimitResult.reject(retryAfter);
    }

    private void prune(long now) {
        long cutoff = now - windowNanos;
        while (!events.isEmpty() && events.peekFirst() <= cutoff) {
            events.removeFirst();
        }
    }
}
