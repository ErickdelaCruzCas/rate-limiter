package rl.core.clock;

/**
 * Real system clock - uses System.nanoTime().
 * Use this for production or concurrent tests where determinism isn't required.
 */
public final class SystemClock implements Clock {
    private static final SystemClock INSTANCE = new SystemClock();

    public static SystemClock instance() {
        return INSTANCE;
    }

    @Override
    public long nowNanos() {
        return System.nanoTime();
    }
}