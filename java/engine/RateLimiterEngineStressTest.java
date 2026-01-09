package rl.java.engine;

import org.junit.jupiter.api.Test;
import rl.core.clock.SystemClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Stress and performance tests for RateLimiterEngine.
 *
 * Performance targets (Phase 4):
 * - > 100K requests/sec (single-threaded)
 * - > 500K requests/sec (multi-threaded)
 * - Latency p99 < 1ms for hot keys
 *
 * Note: These are stress tests, not precise benchmarks.
 * For production benchmarking, use JMH (Phase 8).
 */
class RateLimiterEngineStressTest {

    @Test
    void testStress_singleThreadedThroughput() {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(1_000_000, 1_000_000.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int numRequests = 100_000;
        long startNanos = System.nanoTime();

        for (int i = 0; i < numRequests; i++) {
            engine.tryAcquire("key", 1);
        }

        long elapsedNanos = System.nanoTime() - startNanos;
        double elapsedSeconds = elapsedNanos / 1_000_000_000.0;
        double throughput = numRequests / elapsedSeconds;

        System.out.printf("Single-threaded throughput: %.0f req/s (%.3f ms total)%n",
            throughput, elapsedSeconds * 1000);

        // Target: > 100K req/s
        assertTrue(throughput > 100_000,
            String.format("Expected > 100K req/s, got %.0f req/s", throughput));
    }

    @Test
    void testStress_multiThreadedThroughput() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10_000_000, 10_000_000.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int numThreads = 10;
        int requestsPerThread = 100_000;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        AtomicLong totalRequests = new AtomicLong(0);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        long startNanos = System.nanoTime();

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int j = 0; j < requestsPerThread; j++) {
                        engine.tryAcquire("shared-key", 1);
                        totalRequests.incrementAndGet();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "Test timed out");

        long elapsedNanos = System.nanoTime() - startNanos;
        double elapsedSeconds = elapsedNanos / 1_000_000_000.0;
        double throughput = totalRequests.get() / elapsedSeconds;

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));

        System.out.printf("Multi-threaded throughput (%d threads): %.0f req/s (%.3f ms total)%n",
            numThreads, throughput, elapsedSeconds * 1000);

        // Target: > 500K req/s (relaxed to 250K for CI environments)
        assertTrue(throughput > 250_000,
            String.format("Expected > 250K req/s, got %.0f req/s", throughput));
    }

    @Test
    void testStress_latencyMeasurement() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(1_000_000, 1_000_000.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int warmupRequests = 10_000;
        int measurementRequests = 100_000;

        // Warmup
        for (int i = 0; i < warmupRequests; i++) {
            engine.tryAcquire("key", 1);
        }

        // Measure latencies
        long[] latenciesNanos = new long[measurementRequests];

        for (int i = 0; i < measurementRequests; i++) {
            long start = System.nanoTime();
            engine.tryAcquire("key", 1);
            long elapsed = System.nanoTime() - start;
            latenciesNanos[i] = elapsed;
        }

        // Calculate percentiles
        java.util.Arrays.sort(latenciesNanos);
        long p50 = latenciesNanos[measurementRequests / 2];
        long p95 = latenciesNanos[(int) (measurementRequests * 0.95)];
        long p99 = latenciesNanos[(int) (measurementRequests * 0.99)];
        long max = latenciesNanos[measurementRequests - 1];

        System.out.printf("Latency percentiles (hot key):%n");
        System.out.printf("  p50: %.3f µs%n", p50 / 1000.0);
        System.out.printf("  p95: %.3f µs%n", p95 / 1000.0);
        System.out.printf("  p99: %.3f µs%n", p99 / 1000.0);
        System.out.printf("  max: %.3f µs%n", max / 1000.0);

        // Target: p99 < 1ms = 1,000,000 ns
        // Relaxed to 5ms for CI environments (may not have dedicated resources)
        long p99TargetNanos = 5_000_000; // 5ms
        assertTrue(p99 < p99TargetNanos,
            String.format("Expected p99 < %.3f ms, got %.3f ms",
                p99TargetNanos / 1_000_000.0, p99 / 1_000_000.0));
    }

    @Test
    void testStress_manyKeysMemoryPressure() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 10.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 10_000);

        int numThreads = 10;
        int keysPerThread = 5_000;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        long startNanos = System.nanoTime();

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int k = 0; k < keysPerThread; k++) {
                        String key = "thread-" + threadId + "-key-" + k;
                        engine.tryAcquire(key, 1);
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "Test timed out");

        long elapsedNanos = System.nanoTime() - startNanos;
        double elapsedSeconds = elapsedNanos / 1_000_000_000.0;

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));

        int totalKeysCreated = numThreads * keysPerThread;
        int keysRetained = engine.size();

        System.out.printf("Memory pressure test:%n");
        System.out.printf("  Total keys created: %d%n", totalKeysCreated);
        System.out.printf("  Keys retained (LRU): %d%n", keysRetained);
        System.out.printf("  Eviction rate: %.1f%%%n",
            (totalKeysCreated - keysRetained) * 100.0 / totalKeysCreated);
        System.out.printf("  Time: %.3f seconds%n", elapsedSeconds);

        // Should have evicted down to maxSize
        assertEquals(10_000, keysRetained);
    }

    @Test
    void testStress_sustainedLoadWithRefill() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 1000.0); // 1000 tokens/sec
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        int numThreads = 10;
        int durationSeconds = 3;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        AtomicLong totalAllowed = new AtomicLong(0);
        AtomicLong totalRejected = new AtomicLong(0);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        long startNanos = System.nanoTime();

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    long endTime = System.nanoTime() + (durationSeconds * 1_000_000_000L);

                    while (System.nanoTime() < endTime) {
                        RateLimitResult result = engine.tryAcquire("shared-key", 1);
                        if (result.decision() == Decision.ALLOW) {
                            totalAllowed.incrementAndGet();
                        } else {
                            totalRejected.incrementAndGet();
                        }
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(durationSeconds + 5, TimeUnit.SECONDS));

        long elapsedNanos = System.nanoTime() - startNanos;
        double elapsedSeconds = elapsedNanos / 1_000_000_000.0;

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));

        long totalRequests = totalAllowed.get() + totalRejected.get();
        double requestsPerSecond = totalRequests / elapsedSeconds;
        double allowedPerSecond = totalAllowed.get() / elapsedSeconds;

        System.out.printf("Sustained load test (%.1f seconds):%n", elapsedSeconds);
        System.out.printf("  Total requests: %d (%.0f req/s)%n", totalRequests, requestsPerSecond);
        System.out.printf("  Allowed: %d (%.0f/s)%n", totalAllowed.get(), allowedPerSecond);
        System.out.printf("  Rejected: %d%n", totalRejected.get());
        System.out.printf("  Accept rate: %.1f%%%n", totalAllowed.get() * 100.0 / totalRequests);

        // With 1000 tokens/sec refill, should allow roughly 1000/sec on average
        // Allow some variance due to burst and timing
        assertTrue(allowedPerSecond >= 800 && allowedPerSecond <= 1200,
            String.format("Expected ~1000 allowed/s, got %.0f/s", allowedPerSecond));
    }
}
