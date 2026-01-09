package rl.java.grpc;

import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import rl.core.clock.SystemClock;
import rl.java.engine.RateLimiterConfig;
import rl.java.engine.RateLimiterEngine;
import rl.proto.CheckRateLimitRequest;
import rl.proto.CheckRateLimitResponse;
import rl.proto.RateLimitServiceGrpc;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Stress and performance tests for gRPC service.
 *
 * <p>Tests cover:
 * <ul>
 *   <li>Single-threaded throughput</li>
 *   <li>Multi-threaded concurrent requests</li>
 *   <li>Latency percentiles (p50, p95, p99)</li>
 *   <li>High contention on same key</li>
 *   <li>Multi-key concurrent access</li>
 * </ul>
 *
 * <p>Note: These are stress tests, not precise benchmarks.
 * For production benchmarking, use JMH (Phase 8).
 */
class RateLimitServiceStressTest {

    private Server server;
    private ManagedChannel channel;
    private RateLimitServiceGrpc.RateLimitServiceBlockingStub blockingStub;
    private RateLimiterEngine engine;

    @BeforeEach
    void setUp() throws Exception {
        // Use SystemClock for realistic stress testing
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(1000, 100.0);
        engine = new RateLimiterEngine(clock, config, 10_000);

        // In-process server for testing
        String serverName = InProcessServerBuilder.generateName();
        server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(new RateLimitServiceImpl(engine))
            .build()
            .start();

        channel = InProcessChannelBuilder.forName(serverName)
            .directExecutor()
            .build();

        blockingStub = RateLimitServiceGrpc.newBlockingStub(channel);
    }

    @AfterEach
    void tearDown() throws Exception {
        if (channel != null) {
            channel.shutdown();
            channel.awaitTermination(5, TimeUnit.SECONDS);
        }
        if (server != null) {
            server.shutdown();
            server.awaitTermination(5, TimeUnit.SECONDS);
        }
    }

    @Test
    void testSingleThreadedThroughput() {
        int iterations = 100_000;
        long startNanos = System.nanoTime();

        for (int i = 0; i < iterations; i++) {
            CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
                .setKey("user:" + (i % 1000)) // 1000 unique keys
                .setPermits(1)
                .build();

            blockingStub.checkRateLimit(request);
        }

        long durationNanos = System.nanoTime() - startNanos;
        double durationSeconds = durationNanos / 1_000_000_000.0;
        double throughput = iterations / durationSeconds;

        System.out.printf("Single-threaded throughput: %.2f req/s%n", throughput);

        // Sanity check: Should be > 10K req/s (gRPC has overhead vs direct engine)
        assertTrue(throughput > 10_000,
            "Expected > 10K req/s, got: " + throughput);
    }

    @Test
    void testMultiThreadedThroughput() throws Exception {
        int threads = 10;
        int iterationsPerThread = 10_000;
        ExecutorService executor = Executors.newFixedThreadPool(threads);
        CountDownLatch latch = new CountDownLatch(threads);
        AtomicInteger totalRequests = new AtomicInteger(0);

        long startNanos = System.nanoTime();

        for (int t = 0; t < threads; t++) {
            final int threadId = t;
            executor.submit(() -> {
                try {
                    for (int i = 0; i < iterationsPerThread; i++) {
                        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
                            .setKey("user:" + threadId + ":" + (i % 100))
                            .setPermits(1)
                            .build();

                        blockingStub.checkRateLimit(request);
                        totalRequests.incrementAndGet();
                    }
                } finally {
                    latch.countDown();
                }
            });
        }

        latch.await(30, TimeUnit.SECONDS);
        executor.shutdown();

        long durationNanos = System.nanoTime() - startNanos;
        double durationSeconds = durationNanos / 1_000_000_000.0;
        double throughput = totalRequests.get() / durationSeconds;

        System.out.printf("Multi-threaded throughput (%d threads): %.2f req/s%n",
            threads, throughput);

        assertEquals(threads * iterationsPerThread, totalRequests.get());
        assertTrue(throughput > 50_000,
            "Expected > 50K req/s, got: " + throughput);
    }

    @Test
    void testLatencyPercentiles() {
        int iterations = 10_000;
        List<Long> latencies = new ArrayList<>(iterations);

        // Warmup
        for (int i = 0; i < 1000; i++) {
            blockingStub.checkRateLimit(
                CheckRateLimitRequest.newBuilder()
                    .setKey("warmup")
                    .setPermits(1)
                    .build()
            );
        }

        // Measure
        for (int i = 0; i < iterations; i++) {
            long start = System.nanoTime();

            blockingStub.checkRateLimit(
                CheckRateLimitRequest.newBuilder()
                    .setKey("user:" + (i % 100))
                    .setPermits(1)
                    .build()
            );

            long latency = System.nanoTime() - start;
            latencies.add(latency);
        }

        // Calculate percentiles
        latencies.sort(Long::compareTo);
        long p50 = latencies.get((int) (iterations * 0.50));
        long p95 = latencies.get((int) (iterations * 0.95));
        long p99 = latencies.get((int) (iterations * 0.99));
        long max = latencies.get(iterations - 1);

        System.out.printf("Latency p50: %d ns (%.2f μs)%n", p50, p50 / 1000.0);
        System.out.printf("Latency p95: %d ns (%.2f μs)%n", p95, p95 / 1000.0);
        System.out.printf("Latency p99: %d ns (%.2f μs)%n", p99, p99 / 1000.0);
        System.out.printf("Latency max: %d ns (%.2f μs)%n", max, max / 1000.0);

        // Sanity: p99 should be < 1ms for in-process gRPC
        assertTrue(p99 < 1_000_000,
            "p99 latency too high: " + p99 + " ns");
    }

    @Test
    void testHighContentionSameKey() throws Exception {
        int threads = 20;
        int iterationsPerThread = 100;
        String sharedKey = "high-contention-key";

        ExecutorService executor = Executors.newFixedThreadPool(threads);
        CountDownLatch latch = new CountDownLatch(threads);
        AtomicInteger allowedCount = new AtomicInteger(0);
        AtomicInteger rejectedCount = new AtomicInteger(0);

        for (int t = 0; t < threads; t++) {
            executor.submit(() -> {
                try {
                    for (int i = 0; i < iterationsPerThread; i++) {
                        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
                            .setKey(sharedKey)
                            .setPermits(1)
                            .build();

                        CheckRateLimitResponse response = blockingStub.checkRateLimit(request);

                        if (response.getAllowed()) {
                            allowedCount.incrementAndGet();
                        } else {
                            rejectedCount.incrementAndGet();
                        }
                    }
                } finally {
                    latch.countDown();
                }
            });
        }

        latch.await(30, TimeUnit.SECONDS);
        executor.shutdown();

        int total = allowedCount.get() + rejectedCount.get();
        assertEquals(threads * iterationsPerThread, total);

        System.out.printf("High contention test - Allowed: %d, Rejected: %d%n",
            allowedCount.get(), rejectedCount.get());

        // Should have rejections due to rate limit
        assertTrue(rejectedCount.get() > 0,
            "Expected some rejections under high contention");
    }

    @Test
    void testMultiKeyNoContention() throws Exception {
        int threads = 10;
        int iterationsPerThread = 1000;

        ExecutorService executor = Executors.newFixedThreadPool(threads);
        CountDownLatch latch = new CountDownLatch(threads);
        AtomicInteger successCount = new AtomicInteger(0);

        for (int t = 0; t < threads; t++) {
            final int threadId = t;
            executor.submit(() -> {
                try {
                    for (int i = 0; i < iterationsPerThread; i++) {
                        // Each thread uses unique keys (no contention)
                        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
                            .setKey("thread:" + threadId + ":key:" + i)
                            .setPermits(1)
                            .build();

                        CheckRateLimitResponse response = blockingStub.checkRateLimit(request);

                        // First request to each key should always succeed
                        if (response.getAllowed()) {
                            successCount.incrementAndGet();
                        }
                    }
                } finally {
                    latch.countDown();
                }
            });
        }

        latch.await(30, TimeUnit.SECONDS);
        executor.shutdown();

        // All should succeed (no contention, first access to each key)
        assertEquals(threads * iterationsPerThread, successCount.get());
    }

    @Test
    void testSustainedLoad() throws Exception {
        int threads = 5;
        int durationSeconds = 3;
        ExecutorService executor = Executors.newFixedThreadPool(threads);
        AtomicInteger totalRequests = new AtomicInteger(0);
        AtomicBoolean running = new AtomicBoolean(true);

        long startNanos = System.nanoTime();

        for (int t = 0; t < threads; t++) {
            final int threadId = t;
            executor.submit(() -> {
                int counter = 0;
                while (running.get()) {
                    CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
                        .setKey("sustained:thread:" + threadId)
                        .setPermits(1)
                        .build();

                    blockingStub.checkRateLimit(request);
                    totalRequests.incrementAndGet();
                    counter++;
                }
            });
        }

        // Run for specified duration
        Thread.sleep(durationSeconds * 1000L);
        running.set(false);
        executor.shutdown();
        executor.awaitTermination(5, TimeUnit.SECONDS);

        long durationNanos = System.nanoTime() - startNanos;
        double actualSeconds = durationNanos / 1_000_000_000.0;
        double throughput = totalRequests.get() / actualSeconds;

        System.out.printf("Sustained load (%d threads, %.1fs): %d total requests, %.2f req/s%n",
            threads, actualSeconds, totalRequests.get(), throughput);

        // Should maintain reasonable throughput
        assertTrue(throughput > 10_000,
            "Sustained throughput too low: " + throughput);
    }
}
