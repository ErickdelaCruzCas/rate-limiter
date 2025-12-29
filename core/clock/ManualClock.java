package rl.core.clock;

public final class ManualClock implements Clock {
    private long now;

    public ManualClock(long startNanos) {
        this.now = startNanos;
    }

    @Override
    public long nowNanos() {
        return now;
    }

    public void advanceNanos(long delta) {
        if (delta < 0) throw new IllegalArgumentException("delta < 0");
        now += delta;
    }

    public void setNanos(long value) {
        now = value;
    }
}
