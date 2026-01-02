package rl.core.algorithms.sliding_window_counter;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;
import rl.core.clock.SystemClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

public class SlidingWindowCounterTest {

    // ========== DETERMINISTIC TESTS (ManualClock) ==========

    @Test
    void testAllow_whenWithinLimit() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 100ms buckets (10 buckets), limit 20
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 20);

        // Escenario exitoso: requests dentro del límite
        RateLimitResult result = counter.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow when within limit");

        result = counter.tryAcquire("user1", 8);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow second request within limit");
    }

    @Test
    void testReject_whenLimitExceeded() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 10);

        // Consumir el límite
        counter.tryAcquire("user1", 10);

        // Escenario fallido: exceder el límite
        RateLimitResult result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit exceeded");
        assertTrue(result.retryAfterNanos() > 0, "Should provide retry-after time");
    }

    @Test
    void testBucketRoll_fromRejectToAllow() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 100ms buckets, limit 5
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 5);

        // 1. Consumir todos los permits
        RateLimitResult result = counter.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Initial request should be allowed");

        // 2. Intentar consumir más -> REJECT
        result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit reached");

        // 3. Avanzar 1 segundo completo (todos los buckets deberían rotar)
        clock.advanceNanos(1_000_000_000L);

        // 4. Ahora debe permitir porque los buckets antiguos salieron
        result = counter.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after buckets roll out");

        // 5. Verificar que el límite se aplica de nuevo
        result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should enforce limit in new window");
    }

    @Test
    void testGradualBucketRoll() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 200ms buckets (5 buckets), limit 10
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 200_000_000L, 10);

        // t=0ms: bucket 0, añadir 3
        counter.tryAcquire("user1", 3);

        // t=200ms: bucket 1, añadir 4 (total 7)
        clock.advanceNanos(200_000_000L);
        counter.tryAcquire("user1", 4);

        // t=400ms: bucket 2, añadir 3 (total 10, en el límite)
        clock.advanceNanos(200_000_000L);
        RateLimitResult result = counter.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision());

        // t=500ms: intentar 1 más -> REJECT
        clock.advanceNanos(100_000_000L);
        result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit reached");

        // t=1000ms: bucket 0 sale de la ventana (libera 3 permits)
        clock.setNanos(1_000_000_000L);
        result = counter.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(),
                "Should allow 3 after first bucket rolls out");

        // t=1200ms: bucket 1 sale (libera 4 más)
        clock.setNanos(1_200_000_000L);
        result = counter.tryAcquire("user1", 4);
        assertEquals(Decision.ALLOW, result.decision(),
                "Should allow 4 after second bucket rolls out");
    }

    @Test
    void testRetryAfterCalculation() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 5);

        // Consumir el límite
        counter.tryAcquire("user1", 5);

        // Avanzar a mitad de bucket (50ms)
        clock.advanceNanos(50_000_000L);

        // Intentar adquirir más
        RateLimitResult result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision());

        // Retry-after debería ser hasta el próximo bucket (50ms)
        long expectedRetryAfter = 50_000_000L;
        assertEquals(expectedRetryAfter, result.retryAfterNanos(),
                "Retry-after should be time until next bucket");
    }

    @Test
    void testMultipleBucketsInOneRoll() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 100ms buckets
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 10);

        // t=0: añadir 8
        counter.tryAcquire("user1", 8);

        // Avanzar 5 segundos (mucho más que la ventana)
        clock.advanceNanos(5_000_000_000L);

        // Todos los buckets deberían haber salido, puede añadir 10 de nuevo
        RateLimitResult result = counter.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(),
                "Should allow full capacity after all buckets roll out");
    }

    @Test
    void testBucketAlignment() {
        ManualClock clock = new ManualClock(150_000_000L); // Start at 150ms
        // 1s window, 100ms buckets
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 10);

        // Bucket actual debería estar alineado a 100ms
        counter.tryAcquire("user1", 5);

        // Avanzar dentro del mismo bucket
        clock.setNanos(180_000_000L); // Todavía en bucket [100ms, 200ms)
        RateLimitResult result = counter.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Should accumulate in same bucket");

        // Ahora tenemos 8, intentar 3 más debería fallar
        result = counter.tryAcquire("user1", 3);
        assertEquals(Decision.REJECT, result.decision());
    }

    @Test
    void testRingBufferWrap() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 100ms buckets (10 buckets total en el ring)
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 100);

        // Llenar bucket 0
        counter.tryAcquire("user1", 10);

        // Avanzar 1 segundo exacto (vuelve a bucket 0)
        clock.advanceNanos(1_000_000_000L);
        counter.tryAcquire("user1", 15);

        // Avanzar 1 segundo más (vuelve a bucket 0 de nuevo)
        clock.advanceNanos(1_000_000_000L);
        RateLimitResult result = counter.tryAcquire("user1", 20);
        assertEquals(Decision.ALLOW, result.decision(),
                "Ring buffer should wrap correctly");
    }

    // ========== CONCURRENT TESTS (SystemClock) ==========

    @Test
    void testConcurrent_multipleThreadsWithinLimit() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 50_000_000L, 100);

        int numThreads = 20;
        ExecutorService executor = Executors.newFixedThreadPool(numThreads);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);
        AtomicInteger allowCount = new AtomicInteger(0);

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = counter.tryAcquire("user1", 1);
                    if (result.decision() == Decision.ALLOW) {
                        allowCount.incrementAndGet();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS), "All threads should complete");

        assertEquals(numThreads, allowCount.get(),
                "All threads should be allowed when within limit");

        executor.shutdown();
    }

    @Test
    void testConcurrent_exactLimit() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 50_000_000L, 30);

        int numThreads = 50;
        ExecutorService executor = Executors.newFixedThreadPool(numThreads);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);
        AtomicInteger allowCount = new AtomicInteger(0);
        AtomicInteger rejectCount = new AtomicInteger(0);

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = counter.tryAcquire("user1", 1);
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

        startLatch.countDown();
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS), "All threads should complete");

        assertEquals(numThreads, allowCount.get() + rejectCount.get(),
                "All threads should receive response");

        // Exactamente 30 permitidos
        assertEquals(30, allowCount.get(),
                "Exactly limit requests should be allowed");
        assertEquals(20, rejectCount.get(),
                "Remaining requests should be rejected");

        System.out.println("Sliding Window Counter contention: " + allowCount.get() +
                " allowed, " + rejectCount.get() + " rejected");

        executor.shutdown();
    }

    @Test
    void testConcurrent_bucketRolling() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 500_000_000L, 50_000_000L, 25); // 25 per 500ms

        int numRequests = 100;
        ExecutorService executor = Executors.newFixedThreadPool(10);
        AtomicInteger allowCount = new AtomicInteger(0);
        CountDownLatch doneLatch = new CountDownLatch(numRequests);

        long startTime = System.nanoTime();

        for (int i = 0; i < numRequests; i++) {
            executor.submit(() -> {
                try {
                    RateLimitResult result = counter.tryAcquire("user1", 1);
                    if (result.decision() == Decision.ALLOW) {
                        allowCount.incrementAndGet();
                    } else {
                        // Esperar y reintentar
                        TimeUnit.NANOSECONDS.sleep(result.retryAfterNanos());
                        result = counter.tryAcquire("user1", 1);
                        if (result.decision() == Decision.ALLOW) {
                            allowCount.incrementAndGet();
                        }
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });

            Thread.sleep(8);
        }

        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "All requests should complete");

        long duration = System.nanoTime() - startTime;
        double durationSeconds = duration / 1_000_000_000.0;

        System.out.println("Bucket rolling: " + allowCount.get() + "/" + numRequests +
                " allowed over " + String.format("%.2f", durationSeconds) + " seconds");

        assertTrue(allowCount.get() > 40,
                "Should allow significant requests with bucket rolling and retries");

        executor.shutdown();
    }

    // ========== EDGE CASES ==========

    @Test
    void testInvalidArguments() {
        ManualClock clock = new ManualClock(0);

        // Window <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, 0, 100_000_000L, 10));
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, -1, 100_000_000L, 10));

        // Bucket <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, 1_000_000_000L, 0, 10));
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, 1_000_000_000L, -1, 10));

        // Window % bucket != 0
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, 1_000_000_000L, 300_000_000L, 10),
                "Window must be divisible by bucket size");

        // Limit <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, 1_000_000_000L, 100_000_000L, 0));
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowCounter(clock, 1_000_000_000L, 100_000_000L, -1));

        // Permits <= 0
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 100_000_000L, 10);
        assertThrows(IllegalArgumentException.class,
                () -> counter.tryAcquire("user1", 0));
        assertThrows(IllegalArgumentException.class,
                () -> counter.tryAcquire("user1", -1));
    }

    @Test
    void testSingleBucket() {
        ManualClock clock = new ManualClock(0);
        // 1s window con 1 bucket de 1s = básicamente Fixed Window
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 1_000_000_000L, 10);

        counter.tryAcquire("user1", 10);

        RateLimitResult result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision());

        // Avanzar 1 segundo
        clock.advanceNanos(1_000_000_000L);
        result = counter.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(), "Should behave like fixed window");
    }

    @Test
    void testManySmallBuckets() {
        ManualClock clock = new ManualClock(0);
        // 1s window, 10ms buckets (100 buckets)
        SlidingWindowCounter counter = new SlidingWindowCounter(
                clock, 1_000_000_000L, 10_000_000L, 50);

        // Distribuir eventos a través de varios buckets
        for (int i = 0; i < 10; i++) {
            counter.tryAcquire("user1", 5);
            clock.advanceNanos(15_000_000L); // 15ms
        }

        // Ahora debería estar en el límite
        RateLimitResult result = counter.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(),
                "Should track accurately with many buckets");
    }
}