package rl.core.algorithms.sliding_window;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;
import rl.core.clock.SystemClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

public class SlidingWindowLogTest {

    // ========== DETERMINISTIC TESTS (ManualClock) ==========

    @Test
    void testAllow_whenWithinLimit() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 10);

        // Escenario exitoso: requests dentro del límite
        RateLimitResult result = log.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow when within limit");

        result = log.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow second request within limit");
    }

    @Test
    void testReject_whenLimitExceeded() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 5);

        // Consumir el límite
        log.tryAcquire("user1", 5);

        // Escenario fallido: exceder el límite
        RateLimitResult result = log.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit exceeded");
        assertTrue(result.retryAfterNanos() > 0, "Should provide retry-after time");
    }

    @Test
    void testSlidingWindow_fromRejectToAllow() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 3); // 3 permits per second

        // 1. Consumir todos los permits
        RateLimitResult result = log.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Initial request should be allowed");

        // 2. Intentar consumir más -> REJECT
        result = log.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit reached");

        // 3. Avanzar el tiempo suficiente para que el primer evento salga de la ventana
        clock.advanceNanos(1_000_000_001L); // Un poco más de 1 segundo

        // 4. Ahora los 3 eventos antiguos están fuera de la ventana, debería permitir
        result = log.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after events slide out");
    }

    @Test
    void testPreciseSlidingWindow() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 5);

        // t=0ms: 2 requests
        log.tryAcquire("user1", 2);

        // t=500ms: 3 requests (total 5, en el límite)
        clock.advanceNanos(500_000_000L);
        RateLimitResult result = log.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision());

        // t=600ms: 1 request (debería ser rechazado, total sería 6)
        clock.advanceNanos(100_000_000L); // total 600ms
        result = log.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject, window still contains all 5");

        // t=1001ms: Las primeras 2 requests salen de la ventana [1ms, 1001ms]
        clock.setNanos(1_000_000_001L);
        result = log.tryAcquire("user1", 2);
        assertEquals(Decision.ALLOW, result.decision(),
                "Should allow 2, since first 2 events are outside window");
    }

    @Test
    void testRetryAfterCalculation() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 3);

        // t=0: Añadir 3 eventos
        log.tryAcquire("user1", 3);

        // t=200ms: Intentar añadir 1 más
        clock.advanceNanos(200_000_000L);
        RateLimitResult result = log.tryAcquire("user1", 1);

        assertEquals(Decision.REJECT, result.decision());

        // El evento más antiguo está en t=0, debe esperar hasta t=1000ms
        // Desde t=200ms, debe esperar 800ms
        long expectedRetryAfter = 800_000_000L;
        assertEquals(expectedRetryAfter, result.retryAfterNanos(),
                "Retry-after should be time until oldest event expires");
    }

    @Test
    void testExactPrecision_noBoundaryProblem() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 10);

        // Al final de una "ventana conceptual" (900ms)
        clock.setNanos(900_000_000L);
        log.tryAcquire("user1", 10);

        // Justo después (901ms), no puede agregar más porque los 10 eventos
        // todavía están dentro de la ventana [901ms - 1s, 901ms]
        clock.setNanos(901_000_000L);
        RateLimitResult result = log.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(),
                "Should reject - no boundary problem in sliding window");

        // Debe esperar hasta que los eventos salgan (1900ms)
        clock.setNanos(1_900_000_001L);
        result = log.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after events expire");
    }

    @Test
    void testGradualEviction() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 5);

        // t=0: 1 evento
        log.tryAcquire("user1", 1);

        // t=100ms: 1 evento
        clock.advanceNanos(100_000_000L);
        log.tryAcquire("user1", 1);

        // t=200ms: 1 evento
        clock.advanceNanos(100_000_000L);
        log.tryAcquire("user1", 1);

        // t=300ms: 2 eventos (total 5)
        clock.advanceNanos(100_000_000L);
        log.tryAcquire("user1", 2);

        // t=400ms: 1 evento debería ser rechazado
        clock.advanceNanos(100_000_000L);
        RateLimitResult result = log.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision());

        // t=1001ms: primer evento (t=0) sale, ahora puede añadir 1
        clock.setNanos(1_000_000_001L);
        result = log.tryAcquire("user1", 1);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after first event expires");
    }

    // ========== CONCURRENT TESTS (SystemClock) ==========

    @Test
    void testConcurrent_multipleThreadsWithinLimit() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 100);

        int numThreads = 20;
        ExecutorService executor = Executors.newFixedThreadPool(numThreads);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);
        AtomicInteger allowCount = new AtomicInteger(0);

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = log.tryAcquire("user1", 1);
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
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 30);

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
                    RateLimitResult result = log.tryAcquire("user1", 1);
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

        // Exactamente 30 permitidos (precisión exacta)
        assertEquals(30, allowCount.get(),
                "Exactly limit requests should be allowed (exact precision)");
        assertEquals(20, rejectCount.get(),
                "Remaining requests should be rejected");

        System.out.println("Sliding Window Log contention: " + allowCount.get() +
                " allowed, " + rejectCount.get() + " rejected");

        executor.shutdown();
    }

    @Test
    void testConcurrent_slidingEviction() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        SlidingWindowLog log = new SlidingWindowLog(clock, 500_000_000L, 20); // 20 per 500ms

        int numRequests = 100;
        ExecutorService executor = Executors.newFixedThreadPool(10);
        AtomicInteger allowCount = new AtomicInteger(0);
        CountDownLatch doneLatch = new CountDownLatch(numRequests);

        long startTime = System.nanoTime();

        for (int i = 0; i < numRequests; i++) {
            executor.submit(() -> {
                try {
                    RateLimitResult result = log.tryAcquire("user1", 1);
                    if (result.decision() == Decision.ALLOW) {
                        allowCount.incrementAndGet();
                    } else {
                        // Esperar y reintentar
                        TimeUnit.NANOSECONDS.sleep(result.retryAfterNanos());
                        result = log.tryAcquire("user1", 1);
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

            Thread.sleep(8); // Espaciar requests
        }

        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "All requests should complete");

        long duration = System.nanoTime() - startTime;
        double durationSeconds = duration / 1_000_000_000.0;

        System.out.println("Sliding window eviction: " + allowCount.get() + "/" + numRequests +
                " allowed over " + String.format("%.2f", durationSeconds) + " seconds");

        assertTrue(allowCount.get() > 40,
                "Should allow significant requests with sliding eviction and retries");

        executor.shutdown();
    }

    // ========== EDGE CASES ==========

    @Test
    void testInvalidArguments() {
        ManualClock clock = new ManualClock(0);

        // Window <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowLog(clock, 0, 10));
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowLog(clock, -1, 10));

        // Limit <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowLog(clock, 1_000_000_000L, 0));
        assertThrows(IllegalArgumentException.class,
                () -> new SlidingWindowLog(clock, 1_000_000_000L, -1));

        // Permits <= 0
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 10);
        assertThrows(IllegalArgumentException.class,
                () -> log.tryAcquire("user1", 0));
        assertThrows(IllegalArgumentException.class,
                () -> log.tryAcquire("user1", -1));
    }

    @Test
    void testEmptyWindow() {
        ManualClock clock = new ManualClock(0);
        SlidingWindowLog log = new SlidingWindowLog(clock, 1_000_000_000L, 5);

        // Ventana vacía - debería permitir hasta el límite
        RateLimitResult result = log.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow when window is empty");
    }
}