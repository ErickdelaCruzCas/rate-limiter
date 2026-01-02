package rl.core.algorithms.token_bucket;

import rl.core.clock.Clock;
import rl.core.model.RateLimitResult;
import rl.core.model.RateLimiter;

/**
 * Token Bucket:
 * - capacity: tokens max
 * - refillTokensPerSecond: refill continuo
 *
 * Pros: buen burst + tasa media estable.
 * Contras: estado por key en distribuido; sincronización si hay múltiples nodos.
 *
 * Thread-safety: synchronized para soportar acceso concurrente.
 */
public final class TokenBucket implements RateLimiter {
    private final Clock clock;
    private final long capacity;
    private final double refillPerNanos;

    private double tokens;
    private long lastNanos;

    public TokenBucket(Clock clock, long capacity, double refillTokensPerSecond) {
        if (capacity <= 0) throw new IllegalArgumentException("capacity <= 0");
        if (refillTokensPerSecond <= 0) throw new IllegalArgumentException("refill <= 0");
        this.clock = clock;
        this.capacity = capacity;
        this.refillPerNanos = refillTokensPerSecond / 1_000_000_000d;
        this.tokens = capacity;
        this.lastNanos = clock.nowNanos();
    }

    @Override
    public synchronized RateLimitResult tryAcquire(String key, int permits) {
        if (permits <= 0) throw new IllegalArgumentException("permits <= 0");
        refill();

        if (tokens >= permits) {
            tokens -= permits;
            return RateLimitResult.allow();
        }

        double missing = permits - tokens;
        long retryAfter = (long) Math.ceil(missing / refillPerNanos);
        return RateLimitResult.reject(retryAfter);
    }

    private void refill() {
        long now = clock.nowNanos();
        long elapsed = Math.max(0L, now - lastNanos);
        if (elapsed == 0) return;

        tokens = Math.min(capacity, tokens + elapsed * refillPerNanos);
        lastNanos = now;
    }
}
