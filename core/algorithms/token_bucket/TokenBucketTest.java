package rl.core.algorithms.token_bucket;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;

import static org.junit.jupiter.api.Assertions.*;

public class TokenBucketTest {

    @Test
    void allowsUpToCapacity_thenRejects() {
        ManualClock clock = new ManualClock(0);
        TokenBucket rl = new TokenBucket(clock, 5, 1.0); // 1 token/sec

        for (int i = 0; i < 5; i++) assertEquals("ALLOW", rl.tryAcquire("k", 1).decision().name());
        assertEquals("REJECT", rl.tryAcquire("k", 1).decision().name());
    }

    @Test
    void refillsOverTime_deterministic() {
        ManualClock clock = new ManualClock(0);
        TokenBucket rl = new TokenBucket(clock, 2, 2.0); // 2 tokens/sec

        assertEquals("ALLOW", rl.tryAcquire("k", 2).decision().name());
        assertEquals("REJECT", rl.tryAcquire("k", 1).decision().name());

        clock.advanceNanos(500_000_000L); // +0.5s => +1 token
        assertEquals("ALLOW", rl.tryAcquire("k", 1).decision().name());
    }
}
