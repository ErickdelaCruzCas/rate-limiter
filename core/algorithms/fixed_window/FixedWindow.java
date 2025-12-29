package rl.core.algorithms.fixed_window;

import rl.core.clock.Clock;
import rl.core.model.RateLimitResult;
import rl.core.model.RateLimiter;

public final class FixedWindow implements RateLimiter {
    private final Clock clock;
    private final long windowNanos;
    private final int limit;

    private long windowStart;
    private int used;

    public FixedWindow(Clock clock, long windowNanos, int limit) {
        if (windowNanos <= 0) throw new IllegalArgumentException("window <= 0");
        if (limit <= 0) throw new IllegalArgumentException("limit <= 0");
        this.clock = clock;
        this.windowNanos = windowNanos;
        this.limit = limit;

        long now = clock.nowNanos();
        this.windowStart = align(now);
        this.used = 0;
    }

    @Override
    public RateLimitResult tryAcquire(String key, int permits) {
        if (permits <= 0) throw new IllegalArgumentException("permits <= 0");

        long now = clock.nowNanos();
        long start = align(now);
        if (start != windowStart) {
            windowStart = start;
            used = 0;
        }

        if (used + permits <= limit) {
            used += permits;
            return RateLimitResult.allow();
        }

        long retryAfter = (windowStart + windowNanos) - now;
        return RateLimitResult.reject(retryAfter);
    }

    private long align(long now) {
        return (now / windowNanos) * windowNanos;
    }
}
