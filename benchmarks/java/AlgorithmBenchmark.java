package rl.benchmarks.java;

import org.openjdk.jmh.annotations.*;
import rl.core.algorithms.token_bucket.TokenBucket;
import rl.core.algorithms.fixed_window.FixedWindow;
import rl.core.algorithms.sliding_window.SlidingWindowLog;
import rl.core.algorithms.sliding_window_counter.SlidingWindowCounter;
import rl.core.clock.SystemClock;

import java.util.concurrent.TimeUnit;

/**
 * JMH Benchmarks for core rate limiting algorithms.
 *
 * Measures throughput (ops/sec) for 4 algorithms across 3 scenarios:
 * - allow: Successfully acquire permits (hot path)
 * - reject: Attempt to acquire when exhausted (bucket empty)
 * - parallel: Parallel execution with contention
 *
 * Total: 12 benchmarks (4 algorithms × 3 scenarios)
 *
 * Run all benchmarks:
 *   bazel run //benchmarks/java:benchmarks
 *
 * Run with JSON output:
 *   bazel run //benchmarks/java:benchmarks -- -rf json -rff results.json
 *
 * List all benchmarks:
 *   bazel run //benchmarks/java:benchmarks -- -l
 */
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 3, time = 1)
@Measurement(iterations = 5, time = 1)
@Fork(1)
@State(Scope.Thread)
public class AlgorithmBenchmark {

    private SystemClock clock;

    // Allow scenario: Large capacity, should always succeed
    private TokenBucket tokenBucketAllow;
    private FixedWindow fixedWindowAllow;
    private SlidingWindowLog slidingWindowLogAllow;
    private SlidingWindowCounter slidingWindowCounterAllow;

    // Reject scenario: Exhausted state, should always reject
    private TokenBucket tokenBucketReject;
    private FixedWindow fixedWindowReject;
    private SlidingWindowLog slidingWindowLogReject;
    private SlidingWindowCounter slidingWindowCounterReject;

    @Setup
    public void setup() {
        clock = new SystemClock();

        // Allow scenario configs (high capacity)
        tokenBucketAllow = new TokenBucket(clock, 1_000_000, 1_000_000.0);
        fixedWindowAllow = new FixedWindow(clock, 1_000_000_000L, 1_000_000); // 1 second window
        slidingWindowLogAllow = new SlidingWindowLog(clock, 1_000_000_000L, 1_000_000);
        slidingWindowCounterAllow = new SlidingWindowCounter(clock, 60_000_000_000L, 10, 1_000_000); // 60 second window, 10 buckets

        // Reject scenario configs (low capacity, pre-exhausted)
        tokenBucketReject = new TokenBucket(clock, 0, 0.0); // Zero tokens, zero refill
        fixedWindowReject = new FixedWindow(clock, 1_000_000_000L, 0); // Zero limit
        slidingWindowLogReject = new SlidingWindowLog(clock, 1_000_000_000L, 0);
        slidingWindowCounterReject = new SlidingWindowCounter(clock, 60_000_000_000L, 10, 0);
    }

    // ====== TokenBucket Benchmarks ======

    @Benchmark
    public void tokenBucket_allow() {
        tokenBucketAllow.tryAcquire("user1", 1);
    }

    @Benchmark
    public void tokenBucket_reject() {
        tokenBucketReject.tryAcquire("user1", 1);
    }

    @Benchmark
    @Threads(8)
    public void tokenBucket_parallel() {
        tokenBucketAllow.tryAcquire("user1", 1);
    }

    // ====== FixedWindow Benchmarks ======

    @Benchmark
    public void fixedWindow_allow() {
        fixedWindowAllow.tryAcquire("user1", 1);
    }

    @Benchmark
    public void fixedWindow_reject() {
        fixedWindowReject.tryAcquire("user1", 1);
    }

    @Benchmark
    @Threads(8)
    public void fixedWindow_parallel() {
        fixedWindowAllow.tryAcquire("user1", 1);
    }

    // ====== SlidingWindowLog Benchmarks ======

    @Benchmark
    public void slidingWindowLog_allow() {
        slidingWindowLogAllow.tryAcquire("user1", 1);
    }

    @Benchmark
    public void slidingWindowLog_reject() {
        slidingWindowLogReject.tryAcquire("user1", 1);
    }

    @Benchmark
    @Threads(8)
    public void slidingWindowLog_parallel() {
        slidingWindowLogAllow.tryAcquire("user1", 1);
    }

    // ====== SlidingWindowCounter Benchmarks ======

    @Benchmark
    public void slidingWindowCounter_allow() {
        slidingWindowCounterAllow.tryAcquire("user1", 1);
    }

    @Benchmark
    public void slidingWindowCounter_reject() {
        slidingWindowCounterReject.tryAcquire("user1", 1);
    }

    @Benchmark
    @Threads(8)
    public void slidingWindowCounter_parallel() {
        slidingWindowCounterAllow.tryAcquire("user1", 1);
    }
}
