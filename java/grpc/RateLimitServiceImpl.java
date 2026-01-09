package rl.java.grpc;

import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import rl.core.model.Decision;
import rl.core.model.RateLimitResult;
import rl.java.engine.RateLimiterEngine;
import rl.proto.CheckRateLimitRequest;
import rl.proto.CheckRateLimitResponse;
import rl.proto.HealthCheckRequest;
import rl.proto.HealthCheckResponse;
import rl.proto.RateLimitServiceGrpc;

/**
 * gRPC service implementation for rate limiting.
 *
 * <p>This is a thin wrapper over RateLimiterEngine with:
 * <ul>
 *   <li>Request validation (fail-fast with INVALID_ARGUMENT)</li>
 *   <li>Error handling (INTERNAL for unexpected errors)</li>
 *   <li>Protobuf conversion (RateLimitResult → CheckRateLimitResponse)</li>
 * </ul>
 *
 * <p>Thread-safety: RateLimiterEngine handles concurrency internally.
 * This service is stateless and can handle concurrent RPCs.
 */
public final class RateLimitServiceImpl extends RateLimitServiceGrpc.RateLimitServiceImplBase {

    private final RateLimiterEngine engine;

    /**
     * Creates a new gRPC service wrapping the given engine.
     *
     * @param engine Rate limiter engine (must be thread-safe)
     * @throws IllegalArgumentException if engine is null
     */
    public RateLimitServiceImpl(RateLimiterEngine engine) {
        if (engine == null) {
            throw new IllegalArgumentException("engine cannot be null");
        }
        this.engine = engine;
    }

    @Override
    public void checkRateLimit(
        CheckRateLimitRequest request,
        StreamObserver<CheckRateLimitResponse> responseObserver
    ) {
        try {
            // Validation: key (protobuf strings are never null, only empty)
            if (request.getKey().isEmpty()) {
                responseObserver.onError(
                    Status.INVALID_ARGUMENT
                        .withDescription("key must not be empty")
                        .asRuntimeException()
                );
                return;
            }

            // Validation: permits
            if (request.getPermits() <= 0) {
                responseObserver.onError(
                    Status.INVALID_ARGUMENT
                        .withDescription("permits must be > 0, got: " + request.getPermits())
                        .asRuntimeException()
                );
                return;
            }

            // Execute rate limit check
            RateLimitResult result = engine.tryAcquire(
                request.getKey(),
                request.getPermits()
            );

            // Build response
            CheckRateLimitResponse response = CheckRateLimitResponse.newBuilder()
                .setAllowed(result.decision() == Decision.ALLOW)
                .setRetryAfterNanos(result.retryAfterNanos())
                .build();

            // Send response
            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (IllegalArgumentException e) {
            // Engine validation failures (should be caught by our validation above)
            responseObserver.onError(
                Status.INVALID_ARGUMENT
                    .withDescription(e.getMessage())
                    .withCause(e)
                    .asRuntimeException()
            );
        } catch (Exception e) {
            // Unexpected errors
            responseObserver.onError(
                Status.INTERNAL
                    .withDescription("Internal error: " + e.getMessage())
                    .withCause(e)
                    .asRuntimeException()
            );
        }
    }

    @Override
    public void healthCheck(
        HealthCheckRequest request,
        StreamObserver<HealthCheckResponse> responseObserver
    ) {
        // Simple health check: if we can respond, we're serving
        HealthCheckResponse response = HealthCheckResponse.newBuilder()
            .setStatus(HealthCheckResponse.Status.SERVING)
            .build();

        responseObserver.onNext(response);
        responseObserver.onCompleted();
    }
}
