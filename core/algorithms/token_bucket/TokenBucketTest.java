package rl.core.algorithms.token_bucket;

import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;
import rl.core.clock.SystemClock;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

public class TokenBucketTest {

    // ========== DETERMINISTIC TESTS (ManualClock) ==========

    @Test
    void testAllow_whenTokensAvailable() {
        ManualClock clock = new ManualClock(0);
        TokenBucket bucket = new TokenBucket(clock, 10, 5.0);

        // Escenario exitoso: hay tokens disponibles
        RateLimitResult result = bucket.tryAcquire("user1", 3);

        assertEquals(Decision.ALLOW, result.decision(), "Should allow when tokens available");

        // Siguiente request también debe pasar
        result = bucket.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow second request");
    }

    @Test
    void testReject_whenInsufficientTokens() {
        ManualClock clock = new ManualClock(0);
        TokenBucket bucket = new TokenBucket(clock, 5, 1.0);

        // Consumir todos los tokens
        bucket.tryAcquire("user1", 5);

        // Escenario fallido: no hay tokens suficientes
        RateLimitResult result = bucket.tryAcquire("user1", 1);

        assertEquals(Decision.REJECT, result.decision(), "Should reject when insufficient tokens");
        assertTrue(result.retryAfterNanos() > 0, "Should provide retry-after time");
    }

    @Test
    void testTokenRegeneration_fromRejectToAllow() {
        ManualClock clock = new ManualClock(0);
        TokenBucket bucket = new TokenBucket(clock, 5, 2.0); // 2 tokens/sec

        // 1. Consumir todos los tokens
        RateLimitResult result = bucket.tryAcquire("user1", 5);
        assertEquals(Decision.ALLOW, result.decision(), "Initial request should be allowed");

        // 2. Intentar consumir más -> REJECT
        result = bucket.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject when no tokens left");

        // 3. Avanzar tiempo para regenerar tokens (0.5 seg = 1 token)
        clock.advanceNanos(500_000_000L);

        // 4. Ahora debe permitir porque se regeneró 1 token
        result = bucket.tryAcquire("user1", 1);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after token regeneration");

        // 5. Intentar de nuevo -> REJECT (no hay más tokens)
        result = bucket.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject again after consuming regenerated token");

        // 6. Regenerar más tokens (1 seg = 2 tokens)
        clock.advanceNanos(1_000_000_000L);

        // 7. Ahora debe permitir 2 tokens
        result = bucket.tryAcquire("user1", 2);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow after more regeneration");
    }

    @Test
    void testBurstCapacity() {
        ManualClock clock = new ManualClock(0);
        TokenBucket bucket = new TokenBucket(clock, 100, 10.0);

        // Burst: consumir muchos tokens de golpe
        RateLimitResult result = bucket.tryAcquire("user1", 100);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow burst up to capacity");

        // Siguiente request debe fallar
        result = bucket.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should reject after burst");
    }

    @Test
    void testRetryAfterCalculation() {
        ManualClock clock = new ManualClock(0);
        TokenBucket bucket = new TokenBucket(clock, 10, 5.0); // 5 tokens/sec

        // Consumir todos los tokens
        bucket.tryAcquire("user1", 10);

        // Intentar adquirir 5 tokens cuando no hay ninguno
        RateLimitResult result = bucket.tryAcquire("user1", 5);

        assertEquals(Decision.REJECT, result.decision());

        // Debe tardar 1 segundo en regenerar 5 tokens (5 tokens/sec)
        long retryAfter = result.retryAfterNanos();
        long expectedRetryAfter = 1_000_000_000L; // 1 segundo en nanos

        // Tolerancia de ±1% por aritmética de punto flotante
        assertTrue(Math.abs(retryAfter - expectedRetryAfter) < expectedRetryAfter * 0.01,
                "Retry-after should be approximately 1 second for 5 tokens at 5 tokens/sec");
    }

    @Test
    void testCapacityLimit_doesNotExceedMax() {
        ManualClock clock = new ManualClock(0);
        TokenBucket bucket = new TokenBucket(clock, 10, 5.0);

        // Avanzar mucho tiempo (debería regenerar muchos tokens)
        clock.advanceNanos(10_000_000_000L); // 10 segundos

        // Pero solo debe permitir hasta la capacidad máxima (10 tokens)
        RateLimitResult result = bucket.tryAcquire("user1", 10);
        assertEquals(Decision.ALLOW, result.decision(), "Should allow up to capacity");

        result = bucket.tryAcquire("user1", 1);
        assertEquals(Decision.REJECT, result.decision(), "Should not exceed capacity");
    }

    // ========== CONCURRENT TESTS (SystemClock) ==========

    @Test
    void testConcurrent_multipleThreadsAcquireTokens() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        // Bucket con muchos tokens para que todos los threads tengan éxito
        TokenBucket bucket = new TokenBucket(clock, 1000, 1000.0);

        int numThreads = 10;
        ExecutorService executor = Executors.newFixedThreadPool(numThreads);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);
        AtomicInteger allowCount = new AtomicInteger(0);

        // Lanzar threads concurrentes
        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await(); // Esperar señal de inicio
                    RateLimitResult result = bucket.tryAcquire("user1", 1);
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

        // Iniciar todos los threads al mismo tiempo
        startLatch.countDown();

        // Esperar a que terminen
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS), "All threads should complete");

        // Todos deberían haber tenido éxito (hay suficientes tokens)
        assertEquals(numThreads, allowCount.get(),
                "All threads should be allowed when capacity is sufficient");

        executor.shutdown();
    }

    @Test
    void testConcurrent_threadContention() throws InterruptedException {
        SystemClock clock = SystemClock.instance();
        // Bucket con tokens limitados para crear competencia
        TokenBucket bucket = new TokenBucket(clock, 20, 100.0);

        int numThreads = 50;
        ExecutorService executor = Executors.newFixedThreadPool(numThreads);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numThreads);
        AtomicInteger allowCount = new AtomicInteger(0);
        AtomicInteger rejectCount = new AtomicInteger(0);

        // Lanzar threads que compiten por tokens limitados
        for (int i = 0; i < numThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    RateLimitResult result = bucket.tryAcquire("user1", 1);
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

        // Validar que el total sea correcto
        assertEquals(numThreads, allowCount.get() + rejectCount.get(),
                "All threads should have received a response");

        // Debe haber algunos rechazos (50 threads compitiendo por ~20 tokens)
        assertTrue(rejectCount.get() > 0,
                "Should have some rejections when threads exceed capacity");

        // Debe haber algunos permitidos
        assertTrue(allowCount.get() > 0,
                "Should have some allowed requests");

        System.out.println("Concurrent contention: " + allowCount.get() +
                " allowed, " + rejectCount.get() + " rejected");

        executor.shutdown();
    }

    @Test
    void testConcurrent_refillUnderLoad() throws InterruptedException, ExecutionException {
        SystemClock clock = SystemClock.instance();
        // Bucket pequeño con refill rápido
        TokenBucket bucket = new TokenBucket(clock, 5, 50.0); // 50 tokens/sec

        int numRequests = 100;
        ExecutorService executor = Executors.newFixedThreadPool(10);
        AtomicInteger allowCount = new AtomicInteger(0);
        CountDownLatch doneLatch = new CountDownLatch(numRequests);

        long startTime = System.nanoTime();

        // Generar carga continua durante un período
        for (int i = 0; i < numRequests; i++) {
            executor.submit(() -> {
                try {
                    RateLimitResult result = bucket.tryAcquire("user1", 1);
                    if (result.decision() == Decision.ALLOW) {
                        allowCount.incrementAndGet();
                    } else {
                        // Esperar el retry-after sugerido
                        TimeUnit.NANOSECONDS.sleep(result.retryAfterNanos());
                        // Reintentar
                        result = bucket.tryAcquire("user1", 1);
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

            // Espaciar las requests ligeramente para permitir refill
            Thread.sleep(10);
        }

        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "All requests should complete");

        long duration = System.nanoTime() - startTime;
        double durationSeconds = duration / 1_000_000_000.0;

        System.out.println("Refill under load: " + allowCount.get() + "/" + numRequests +
                " allowed over " + String.format("%.2f", durationSeconds) + " seconds");

        // Debería permitir bastantes requests debido al refill continuo
        assertTrue(allowCount.get() > 50,
                "Should allow majority of requests with refill and retries");

        executor.shutdown();
    }

    // ========== EDGE CASES ==========

    @Test
    void testInvalidArguments() {
        ManualClock clock = new ManualClock(0);

        // Capacity <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new TokenBucket(clock, 0, 10.0));
        assertThrows(IllegalArgumentException.class,
                () -> new TokenBucket(clock, -1, 10.0));

        // Refill rate <= 0
        assertThrows(IllegalArgumentException.class,
                () -> new TokenBucket(clock, 10, 0.0));
        assertThrows(IllegalArgumentException.class,
                () -> new TokenBucket(clock, 10, -1.0));

        // Permits <= 0
        TokenBucket bucket = new TokenBucket(clock, 10, 1.0);
        assertThrows(IllegalArgumentException.class,
                () -> bucket.tryAcquire("user1", 0));
        assertThrows(IllegalArgumentException.class,
                () -> bucket.tryAcquire("user1", -1));
    }
}