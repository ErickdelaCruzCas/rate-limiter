package rl.java.grpc;

import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import rl.core.clock.ManualClock;
import rl.java.engine.RateLimiterConfig;
import rl.java.engine.RateLimiterEngine;
import rl.proto.CheckRateLimitRequest;
import rl.proto.CheckRateLimitResponse;
import rl.proto.HealthCheckRequest;
import rl.proto.HealthCheckResponse;
import rl.proto.RateLimitServiceGrpc;

import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Integration tests for RateLimitServiceImpl using InProcessServer.
 *
 * <p>Tests cover:
 * <ul>
 *   <li>Allow/reject behavior</li>
 *   <li>Validation errors (empty key, invalid permits)</li>
 *   <li>Health check endpoint</li>
 *   <li>Multi-key isolation</li>
 *   <li>Token refill mechanics</li>
 * </ul>
 */
class RateLimitServiceImplTest {

    private Server server;
    private ManagedChannel channel;
    private ManualClock clock;
    private RateLimiterEngine engine;
    private RateLimitServiceGrpc.RateLimitServiceBlockingStub blockingStub;

    @BeforeEach
    void setUp() throws Exception {
        // Create engine with ManualClock for deterministic tests
        clock = new ManualClock(0L);
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(10, 1.0); // 10 tokens, 1/sec
        engine = new RateLimiterEngine(clock, config, 100);

        // Create in-process server
        String serverName = InProcessServerBuilder.generateName();
        server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(new RateLimitServiceImpl(engine))
            .build()
            .start();

        // Create client channel
        channel = InProcessChannelBuilder.forName(serverName)
            .directExecutor()
            .build();

        // Create blocking stub
        blockingStub = RateLimitServiceGrpc.newBlockingStub(channel);
    }

    @AfterEach
    void tearDown() throws Exception {
        // Shutdown channel
        if (channel != null) {
            channel.shutdown();
            channel.awaitTermination(5, TimeUnit.SECONDS);
        }

        // Shutdown server
        if (server != null) {
            server.shutdown();
            server.awaitTermination(5, TimeUnit.SECONDS);
        }
    }

    @Test
    void testAllow_whenWithinLimit() {
        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
            .setKey("user:123")
            .setPermits(5)
            .build();

        CheckRateLimitResponse response = blockingStub.checkRateLimit(request);

        assertTrue(response.getAllowed());
        assertEquals(0L, response.getRetryAfterNanos());
    }

    @Test
    void testReject_whenExceedingLimit() {
        // Consume all tokens
        blockingStub.checkRateLimit(
            CheckRateLimitRequest.newBuilder()
                .setKey("user:123")
                .setPermits(10)
                .build()
        );

        // This should reject
        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
            .setKey("user:123")
            .setPermits(1)
            .build();

        CheckRateLimitResponse response = blockingStub.checkRateLimit(request);

        assertFalse(response.getAllowed());
        assertTrue(response.getRetryAfterNanos() > 0);
    }

    @Test
    void testValidation_emptyKey() {
        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
            .setKey("")
            .setPermits(1)
            .build();

        StatusRuntimeException exception = assertThrows(
            StatusRuntimeException.class,
            () -> blockingStub.checkRateLimit(request)
        );

        assertEquals(Status.INVALID_ARGUMENT.getCode(), exception.getStatus().getCode());
        assertTrue(exception.getMessage().contains("key must not be empty"));
    }

    @Test
    void testValidation_invalidPermits() {
        CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
            .setKey("user:123")
            .setPermits(0)
            .build();

        StatusRuntimeException exception = assertThrows(
            StatusRuntimeException.class,
            () -> blockingStub.checkRateLimit(request)
        );

        assertEquals(Status.INVALID_ARGUMENT.getCode(), exception.getStatus().getCode());
        assertTrue(exception.getMessage().contains("permits must be > 0"));
    }

    @Test
    void testHealthCheck() {
        HealthCheckRequest request = HealthCheckRequest.newBuilder().build();

        HealthCheckResponse response = blockingStub.healthCheck(request);

        assertEquals(HealthCheckResponse.Status.SERVING, response.getStatus());
    }

    @Test
    void testMultiKeyIsolation() {
        // User 1 consumes tokens
        blockingStub.checkRateLimit(
            CheckRateLimitRequest.newBuilder()
                .setKey("user:1")
                .setPermits(10)
                .build()
        );

        // User 2 should still have full capacity
        CheckRateLimitResponse response = blockingStub.checkRateLimit(
            CheckRateLimitRequest.newBuilder()
                .setKey("user:2")
                .setPermits(5)
                .build()
        );

        assertTrue(response.getAllowed());
    }

    @Test
    void testTokenRefill_fromRejectToAllow() {
        // Consume all tokens
        blockingStub.checkRateLimit(
            CheckRateLimitRequest.newBuilder()
                .setKey("user:123")
                .setPermits(10)
                .build()
        );

        // Should reject
        CheckRateLimitResponse response1 = blockingStub.checkRateLimit(
            CheckRateLimitRequest.newBuilder()
                .setKey("user:123")
                .setPermits(1)
                .build()
        );
        assertFalse(response1.getAllowed());

        // Advance time by 2 seconds (refill 2 tokens at 1 token/sec)
        clock.advanceNanos(2_000_000_000L);

        // Should allow now
        CheckRateLimitResponse response2 = blockingStub.checkRateLimit(
            CheckRateLimitRequest.newBuilder()
                .setKey("user:123")
                .setPermits(2)
                .build()
        );
        assertTrue(response2.getAllowed());
    }
}
