package rl.core.model;

public record RateLimitResult(
    Decision decision,
    long retryAfterNanos
) {
    public static RateLimitResult allow() {
        return new RateLimitResult(Decision.ALLOW, 0L);
    }

    public static RateLimitResult reject(long retryAfterNanos) {
        return new RateLimitResult(Decision.REJECT, Math.max(0L, retryAfterNanos));
    }
}
