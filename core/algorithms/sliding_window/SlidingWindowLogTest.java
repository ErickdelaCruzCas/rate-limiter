package rl.core.algorithms.sliding_window;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;

import static org.junit.jupiter.api.Assertions.*;

public class SlidingWindowLogTest {

    @Test
    void withinWindow_countsExactly() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog rl = new SlidingWindowLog(clock, 1_000_000_000L, 2);

        assertEquals("ALLOW", rl.tryAcquire("k", 1).decision().name());
        assertEquals("ALLOW", rl.tryAcquire("k", 1).decision().name());
        assertEquals("REJECT", rl.tryAcquire("k", 1).decision().name());

        clock.advanceNanos(1_000_000_000L);
        assertEquals("ALLOW", rl.tryAcquire("k", 1).decision().name());
    }
}
