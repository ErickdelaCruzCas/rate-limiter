package rl.core.algorithms.fixed_window;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;
import rl.core.clock.SystemClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

public class FixedWindowTest {

    // ========== DETERMINISTIC TESTS (ManualClock) ==========

    @Test
    void testAllow_whenWithinLimit() {
        ManualClock clock = new ManualClock(0);
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 10);

        // Escenario exitoso: requests dentro del límite
        RateLimitResult result = window.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow when within limit");

        result = window.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow second request within limit");
    }

    @Test
    void testReject_whenLimitExceeded() {
        ManualClock clock = new ManualClock(0);
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 5);

        // Consumir el límite
        window.tryAcquire("user1", 5);

        // Escenario fallido: exceder el límite
        RateLimitResult result = window.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit exceeded");
        assertTrue(result.retryAfterNanos() > 0, "Should provide retry-after time");
    }

    @Test
    void testWindowReset_fromRejectToAllow() {
        ManualClock clock = new ManualClock(0);
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 3); // 3 permits per second

        // 1. Consumir todos los permits
        RateLimitResult result = window.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Initial request should be allowed");

        // 2. Intentar consumir más -> REJECT
        result = window.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit reached");

        // 3. Avanzar a la siguiente ventana (1 segundo)
        clock.advanceNanos(1_000_000_000L);

        // 4. Debe permitir porque la ventana se resetea
        result = window.tryAcquire("user1", 3);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after window reset");

        // 5. Verificar que el límite se aplica de nuevo
        result = window.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should enforce limit in new window");
    }

    @Test
    void testPartialWindowReset() {
        ManualClock clock = new ManualClock(0);
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 10);

        // Consumir algunos permits
        window.tryAcquire("user1", 6);

        // Avanzar menos de una ventana (no debe resetear)
        clock.advanceNanos(500_000_000L); // 0.5 segundos

        // Debe seguir contando desde 6
        RateLimitResult result = window.tryAcquire("user1", 4);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow within same window");

        result = window.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when limit exceeded");
    }

    @Test
    void testRetryAfterCalculation() {
        ManualClock clock = new ManualClock(0);
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 5);

        // Consumir el límite
        window.tryAcquire("user1", 5);

        // Avanzar 300ms
        clock.advanceNanos(300_000_000L);

        // Intentar adquirir más
        RateLimitResult result = window.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision());

        // Debe esperar 700ms hasta la siguiente ventana (1s - 300ms)
        long expectedRetryAfter = 700_000_000L;
        assertEquals(expectedRetryAfter, result.retryAfterNanos(),
                "Retry-after should be time until next window");
    }

    @Test
    void testBoundaryProblem() {
        ManualClock clock = new ManualClock(0);
        // Window de 1 segundo, límite de 10
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 10);

        // Al final de la primera ventana (900ms)
        clock.setNanos(900_000_000L);
        RateLimitResult result = window.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow 10 at end of first window");

        // Justo al inicio de la segunda ventana (1000ms)
        clock.setNanos(1_000_000_000L);
        result = window.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow 10 at start of second window");

        // Esto demuestra el boundary problem: 20 permits en 100ms
        // (10 al final de ventana 1, 10 al inicio de ventana 2)
    }

    // ========== CONCURRENT TESTS (SystemClock) ==========

    @Test
    void testConcurrent_multipleThreadsWithinLimit() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 100);

        int numThreads = 20;
        ExecutorService executor = Executors.newFixedThreadPool(numThreads);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);
        AtomicInteger allowCount = new AtomicInteger(0);

        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = window.tryAcquire("user1", 1);
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

        // Todos deberían ser permitidos (20 < 100)
        assertEquals(numThreads, allowCount.get(),
                "All threads should be allowed when within limit");

        executor.shutdown();
    }

    @Test
    void testConcurrent_threadContention() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 30);

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
                    RateLimitResult result = window.tryAcquire("user1", 1);
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

        // Exactamente 30 deberían ser permitidos
        assertEquals(30, allowCount.get(),
                "Exactly limit requests should be allowed");
        assertEquals(20, rejectCount.get(),
                "Remaining requests should be rejected");

        System.out.println("Fixed Window contention: " + allowCount.get() +
                " allowed, " + rejectCount.get() + " rejected");

        executor.shutdown();
    }

    @Test
    void testConcurrent_windowReset() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 20);

        int numRequests = 100;
        ExecutorService executor = Executors.newFixedThreadPool(10);
        AtomicInteger allowCount = new AtomicInteger(0);
        CountDownLatch doneLatch = new CountDownLatch(numRequests);

        long startTime = System.nanoTime();

        // Generar carga continua que atraviesa ventanas
        for (int i = 0; i < numRequests; i++) {
            executor.submit(() -> {
                try {
                    RateLimitResult result = window.tryAcquire("user1", 1);
                    if (result.decision() == Decision.ALLOW) {
                        allowCount.incrementAndGet();
                    } else {
                        // Esperar hasta la siguiente ventana
                        TimeUnit.NANOSECONDS.sleep(result.retryAfterNanos());
                        // Reintentar
                        result = window.tryAcquire("user1", 1);
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

            // Espaciar requests
            Thread.sleep(15);
        }

        assertTrue(doneLatch.await(20, TimeUnit.SECONDS), "All requests should complete");

        long duration = System.nanoTime() - startTime;
        double durationSeconds = duration / 1_000_000_000.0;

        System.out.println("Window reset under load: " + allowCount.get() + "/" + numRequests +
                " allowed over " + String.format("%.2f", durationSeconds) + " seconds");

        // Debería permitir la mayoría debido a los resets de ventana
        assertTrue(allowCount.get() > 80,
                "Should allow most requests with window resets");

        executor.shutdown();
    }

    // ========== EDGE CASES ==========

    @Test
    void testInvalidArguments() {
        ManualClock clock = new ManualClock(0);

        // Window <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new FixedWindow(clock, 0, 10));
        assertThrows(IllegalArgumentException.class,
                () -> new FixedWindow(clock, -1, 10));

        // Limit <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new FixedWindow(clock, 1_000_000_000L, 0));
        assertThrows(IllegalArgumentException.class,
                () -> new FixedWindow(clock, 1_000_000_000L, -1));

        // Permits <= 0
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 10);
        assertThrows(IllegalArgumentException.class,
                () -> window.tryAcquire("user1", 0));
        assertThrows(IllegalArgumentException.class,
                () -> window.tryAcquire("user1", -1));
    }

    @Test
    void testWindowAlignment() {
        ManualClock clock = new ManualClock(500_000_000L); // Start at 0.5 seconds
        FixedWindow window = new FixedWindow(clock, 1_000_000_000L, 5);

        // Primera ventana: [0s, 1s)
        RateLimitResult result = window.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision());

        result = window.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision());

        // Avanzar a la siguiente ventana alineada: [1s, 2s)
        clock.setNanos(1_000_000_000L);

        result = window.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Should reset at aligned window boundary");
    }
}