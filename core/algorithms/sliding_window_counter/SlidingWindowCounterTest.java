package rl.core.algorithms.sliding_window_counter;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;

import static org.junit.jupiter.api.Assertions.*;

public class SlidingWindowCounterTest {

    @Test
    void enforcesLimit_withBucketRoll() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 100ms buckets, limit 5
        SlidingWindowCounter rl = new SlidingWindowCounter(clock, 1_000_000_000L, 100_000_000L, 5);

        assertEquals("ALLOW", rl.tryAcquire("k", 5).decision().name());
        assertEquals("REJECT", rl.tryAcquire("k", 1).decision().name());

        // avanzar 1s completo: debería vaciar ventana
        clock.advanceNanos(1_000_000_000L);
        assertEquals("ALLOW", rl.tryAcquire("k", 5).decision().name());
    }
}
