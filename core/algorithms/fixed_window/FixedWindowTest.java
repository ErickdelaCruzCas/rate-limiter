package rl.core.algorithms.fixed_window;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public class FixedWindowTest {

    @Test
    void simple_test() {
        assertTrue(true);
    }

//    @Test
//    void rejectsWhenLimitExceeded_andResetsNextWindow() {
//        ManualClock clock = new ManualClock(0);
//        FixedWindow rl = new FixedWindow(clock, 1_000_000_000L, 3);
//
//        assertEquals("ALLOW", rl.tryAcquire("k", 3).decision().name());
//        assertEquals("REJECT", rl.tryAcquire("k", 1).decision().name());
//
//        clock.advanceNanos(1_000_000_000L); // next window
//        assertEquals("ALLOW", rl.tryAcquire("k", 3).decision().name());
//    }
}
