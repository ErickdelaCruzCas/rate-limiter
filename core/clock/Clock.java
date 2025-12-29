package rl.core.clock;

public interface Clock {
    /** Monotonic time in nanos (testable). */
    long nowNanos();
}
