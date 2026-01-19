package rl.benchmarks.java;

import org.openjdk.jmh.annotations.*;
import rl.core.clock.SystemClock;
import rl.java.engine.RateLimiterEngine;
import rl.java.engine.RateLimiterConfig;
import rl.java.engine.AlgorithmType;

import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;

/**
 * JMH Benchmarks for RateLimiterEngine (thread-safe multi-key wrapper).
 *
 * Measures throughput (ops/sec) across 3 scenarios:
 * - singleKey: All requests to same key (high contention)
 * - multiKey: Rotating through 1000 different keys (low contention)
 * - parallel: 8 threads with high contention on single key
 *
 * Total: 3 benchmarks
 *
 * Run:
 *   bazel run //benchmarks/java:benchmarks -- Engine
 *
 * Run with JSON output:
 *   bazel run //benchmarks/java:benchmarks -- Engine -rf json -rff results.json
 */
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 3, time = 1)
@Measurement(iterations = 5, time = 1)
@Fork(1)
@State(Scope.Thread)
public class EngineBenchmark {

    private RateLimiterEngine engine;

    @Setup
    public void setup() {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(1_000_000, 1_000_000.0);

        engine = new RateLimiterEngine(clock, config, 10_000);
    }

    /**
     * Single key throughput (high contention on same key).
     */
    @Benchmark
    public void singleKey() {
        engine.tryAcquire("user1", 1);
    }

    /**
     * Multi-key throughput (rotating through 1000 keys, low contention).
     */
    @Benchmark
    public void multiKey() {
        String key = "user:" + ThreadLocalRandom.current().nextInt(1000);
        engine.tryAcquire(key, 1);
    }

    /**
     * Parallel throughput with 8 threads on single key (high contention).
     */
    @Benchmark
    @Threads(8)
    public void parallel() {
        engine.tryAcquire("user1", 1);
    }
}
