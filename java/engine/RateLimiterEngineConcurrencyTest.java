package rl.java.engine;

import org.junit.jupiter.api.Test;
import rl.core.clock.SystemClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Concurrency tests for RateLimiterEngine.
 *
 * Focus:
 * - Thread-safety with CountDownLatch
 * - Race conditions prevention
 * - Lock contention under load
 * - Multi-threaded correctness
 */
class RateLimiterEngineConcurrencyTest {

    @Test
    void testConcurrent_multipleThreadsSameKey() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 10.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int numThreads = 10;
        int permitsPerThread = 10;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        AtomicInteger allowCount = new AtomicInteger(0);
        AtomicInteger rejectCount = new AtomicInteger(0);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await(); // Wait for signal to start
                    RateLimitResult result = engine.tryAcquire("user:123", permitsPerThread);
                    if (result.decision() == Decision.ALLOW) {
                        allowCount.incrementAndGet();
                    } else {
                        rejectCount.incrementAndGet();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown(); // Signal all threads to start
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS), "Test timed out");

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS), "Executor did not terminate");

        // Only 10 threads should succeed (100 capacity / 10 permits each)
        assertEquals(10, allowCount.get());
        assertEquals(0, rejectCount.get());
    }

    @Test
    void testConcurrent_multipleKeysShouldNotInterfere() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(50, 10.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int numThreads = 20; // 10 for key1, 10 for key2
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        AtomicInteger key1Allowed = new AtomicInteger(0);
        AtomicInteger key2Allowed = new AtomicInteger(0);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        // 10 threads for key1
        for (int i = 0; i < 10; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = engine.tryAcquire("key1", 5);
                    if (result.decision() == Decision.ALLOW) {
                        key1Allowed.incrementAndGet();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        // 10 threads for key2
        for (int i = 0; i < 10; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = engine.tryAcquire("key2", 5);
                    if (result.decision() == Decision.ALLOW) {
                        key2Allowed.incrementAndGet();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS));

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));

        // Each key should allow exactly 10 threads (50 capacity / 5 permits each)
        assertEquals(10, key1Allowed.get());
        assertEquals(10, key2Allowed.get());
    }

    @Test
    void testConcurrent_highContentionSameKey() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 50.0); // High refill
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int numThreads = 50;
        int requestsPerThread = 20;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        AtomicInteger totalAllowed = new AtomicInteger(0);
        AtomicInteger totalRejected = new AtomicInteger(0);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int j = 0; j < requestsPerThread; j++) {
                        RateLimitResult result = engine.tryAcquire("hot-key", 1);
                        if (result.decision() == Decision.ALLOW) {
                            totalAllowed.incrementAndGet();
                        } else {
                            totalRejected.incrementAndGet();
                            // Optional: sleep and retry
                            if (result.retryAfterNanos() > 0) {
                                Thread.sleep(result.retryAfterNanos() / 1_000_000);
                            }
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
        assertTrue(doneLatch.await(10, TimeUnit.SECONDS));

        executor.shutdown();
        assertTrue(executor.awaitTermination(10, TimeUnit.SECONDS));

        // Total requests
        int totalRequests = numThreads * requestsPerThread;
        assertEquals(totalRequests, totalAllowed.get() + totalRejected.get());

        // At least 100 should be allowed initially (capacity)
        assertTrue(totalAllowed.get() >= 100, "Expected at least 100 allowed, got " + totalAllowed.get());

        // Some should be rejected due to contention
        assertTrue(totalRejected.get() > 0, "Expected some rejections under high contention");
    }

    @Test
    void testConcurrent_LRUEvictionUnderLoad() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 10); // Max 10 keys

        int numThreads = 20;
        int keysPerThread = 5;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

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
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS));

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));

        // Total keys created: 20 threads * 5 keys = 100 keys
        // But max is 10, so should have evicted down to 10
        assertEquals(10, engine.size());
    }

    @Test
    void testConcurrent_noDeadlockWithMultipleKeys() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 10.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 1000);

        int numThreads = 20;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        // Each thread alternates between key1 and key2
        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int j = 0; j < 100; j++) {
                        String key = (threadId % 2 == 0) ? "key1" : "key2";
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

        // If there's a deadlock, this will timeout
        assertTrue(doneLatch.await(10, TimeUnit.SECONDS), "Deadlock detected - test timed out");

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));
    }

    @Test
    void testConcurrent_correctnessUnderLoad() throws InterruptedException {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(1000, 100.0);
        RateLimiterEngine engine = new RateLimiterEngine(clock, config, 100);

        int numThreads = 100;
        int requestsPerThread = 10;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);

        ConcurrentHashMap<String, AtomicInteger> resultsPerKey = new ConcurrentHashMap<>();

        ExecutorService executor = Executors.newFixedThreadPool(numThreads);

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            executor.submit(() -> {
                try {
                    startLatch.await();
                    String key = "key-" + (threadId % 10); // 10 different keys
                    for (int j = 0; j < requestsPerThread; j++) {
                        RateLimitResult result = engine.tryAcquire(key, 1);
                        if (result.decision() == Decision.ALLOW) {
                            resultsPerKey.computeIfAbsent(key, k -> new AtomicInteger(0))
                                .incrementAndGet();
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
        assertTrue(doneLatch.await(10, TimeUnit.SECONDS));

        executor.shutdown();
        assertTrue(executor.awaitTermination(5, TimeUnit.SECONDS));

        // Verify some requests were allowed for each key
        assertEquals(10, resultsPerKey.size());
        resultsPerKey.forEach((key, count) -> {
            assertTrue(count.get() > 0, "Key " + key + " should have some allowed requests");
        });
    }
}
