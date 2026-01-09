package rl.java.engine;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Core functional tests for RateLimiterEngine.
 *
 * Focus:
 * - Basic allow/reject behavior
 * - Multi-key isolation
 * - LRU eviction
 * - Configuration handling
 * - Edge cases
 */
class RateLimiterEngineTest {

    @Test
    void testAllow_whenWithinLimit() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        RateLimitResult result = engine.tryAcquire("user:123", 5);

        assertEquals(Decision.ALLOW, result.decision());
        assertEquals(0L, result.retryAfterNanos());
    }

    @Test
    void testReject_whenExceedingLimit() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        // Consume all tokens
        engine.tryAcquire("user:123", 10);

        // This should reject
        RateLimitResult result = engine.tryAcquire("user:123", 1);

        assertEquals(Decision.REJECT, result.decision());
        assertTrue(result.retryAfterNanos() > 0);
    }

    @Test
    void testMultipleKeys_isolated() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        // User 123 consumes all tokens
        engine.tryAcquire("user:123", 10);
        RateLimitResult result123 = engine.tryAcquire("user:123", 1);
        assertEquals(Decision.REJECT, result123.decision());

        // User 456 should still be allowed (separate limiter)
        RateLimitResult result456 = engine.tryAcquire("user:456", 5);
        assertEquals(Decision.ALLOW, result456.decision());

        // Verify both keys exist
        assertEquals(2, engine.size());
    }

    @Test
    void testLRUEviction_leastRecentlyUsedEvicted() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 3); // Max 3 keys

        // Add 3 keys
        engine.tryAcquire("key1", 1);
        engine.tryAcquire("key2", 1);
        engine.tryAcquire("key3", 1);
        assertEquals(3, engine.size());

        // Add 4th key - should evict key1 (least recently used)
        engine.tryAcquire("key4", 1);
        assertEquals(3, engine.size());

        // Access key2 and key3 to make them recently used
        engine.tryAcquire("key2", 1);
        engine.tryAcquire("key3", 1);

        // Add key5 - should evict key4 now
        engine.tryAcquire("key5", 1);
        assertEquals(3, engine.size());

        // key2, key3, key5 should exist
        // All should have some tokens consumed but not rejected
        assertEquals(Decision.ALLOW, engine.tryAcquire("key2", 1).decision());
        assertEquals(Decision.ALLOW, engine.tryAcquire("key3", 1).decision());
        assertEquals(Decision.ALLOW, engine.tryAcquire("key5", 1).decision());
    }

    @Test
    void testTokenRefill_afterTimeAdvance() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 10.0); // 10 tokens/sec
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        // Consume all tokens
        engine.tryAcquire("user:123", 10);
        assertEquals(Decision.REJECT, engine.tryAcquire("user:123", 1).decision());

        // Advance 1 second - should refill 10 tokens
        clock.advanceNanos(1_000_000_000L);

        // Should allow now
        RateLimitResult result = engine.tryAcquire("user:123", 10);
        assertEquals(Decision.ALLOW, result.decision());
    }

    @Test
    void testDifferentAlgorithms_fixedWindow() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.fixedWindow(10, 1_000_000_000L); // 10/sec
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        // Consume all permits
        engine.tryAcquire("user:123", 10);
        assertEquals(Decision.REJECT, engine.tryAcquire("user:123", 1).decision());

        // Advance to next window
        clock.advanceNanos(1_000_000_000L);

        // Should allow now (window reset)
        assertEquals(Decision.ALLOW, engine.tryAcquire("user:123", 10).decision());
    }

    @Test
    void testClear_removesAllKeys() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        engine.tryAcquire("key1", 1);
        engine.tryAcquire("key2", 1);
        engine.tryAcquire("key3", 1);
        assertEquals(3, engine.size());

        engine.clear();
        assertEquals(0, engine.size());
    }

    @Test
    void testInvalidArguments() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);

        // Null clock
        assertThrows(IllegalArgumentException.class,
            () -> new RateLimiterEngine(null, config, 100));

        // Null config
        assertThrows(IllegalArgumentException.class,
            () -> new RateLimiterEngine(clock, null, 100));

        // Invalid maxKeys
        assertThrows(IllegalArgumentException.class,
            () -> new RateLimiterEngine(clock, config, 0));
        assertThrows(IllegalArgumentException.class,
            () -> new RateLimiterEngine(clock, config, -1));

        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        // Null key
        assertThrows(IllegalArgumentException.class,
            () -> engine.tryAcquire(null, 1));

        // Invalid permits
        assertThrows(IllegalArgumentException.class,
            () -> engine.tryAcquire("key", 0));
        assertThrows(IllegalArgumentException.class,
            () -> engine.tryAcquire("key", -1));
    }

    @Test
    void testMaxSize_returnsConfiguredValue() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 500);

        assertEquals(500, engine.maxSize());
    }

    @Test
    void testGetDefaultConfig_returnsConfiguredValue() {
        ManualClock clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 5.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        assertEquals(config, engine.getDefaultConfig());
        assertEquals(AlgorithmType.TOKEN_BUCKET, engine.getDefaultConfig().algorithmType());
        assertEquals(100, engine.getDefaultConfig().capacity());
        assertEquals(5.0, engine.getDefaultConfig().refillRate());
    }
}
