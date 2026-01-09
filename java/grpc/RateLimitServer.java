package rl.java.grpc;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import rl.core.clock.SystemClock;
import rl.java.engine.RateLimiterConfig;
import rl.java.engine.RateLimiterEngine;

import java.io.IOException;
import java.util.concurrent.TimeUnit;

/**
 * Standalone gRPC server for rate limiting service.
 *
 * <p>Features:
 * <ul>
 *   <li>Configurable port (default: 9090)</li>
 *   <li>Graceful shutdown with timeout</li>
 *   <li>Default Token Bucket configuration (100 capacity, 10/sec refill)</li>
 *   <li>SystemClock for production</li>
 * </ul>
 *
 * <p>Usage:
 * <pre>
 * // Run with defaults
 * bazel run //java/grpc:server
 *
 * // Run with custom port
 * bazel run //java/grpc:server -- 8080
 * </pre>
 */
public final class RateLimitServer {

    private final Server server;
    private static final int DEFAULT_PORT = 9090;
    private static final int SHUTDOWN_TIMEOUT_SECONDS = 5;

    /**
     * Creates a server on the specified port.
     *
     * @param port Port to listen on
     */
    public RateLimitServer(int port) {
        this(port, createDefaultEngine());
    }

    /**
     * Creates a server with custom engine (useful for testing).
     *
     * @param port Port to listen on
     * @param engine Rate limiter engine
     */
    public RateLimitServer(int port, RateLimiterEngine engine) {
        this.server = ServerBuilder.forPort(port)
            .addService(new RateLimitServiceImpl(engine))
            .build();
    }

    /**
     * Starts the server.
     *
     * @throws IOException if server fails to start
     */
    public void start() throws IOException {
        server.start();
        System.out.println("RateLimitServer started on port: " + server.getPort());

        // Graceful shutdown hook
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            System.err.println("Shutting down gRPC server (JVM shutdown hook)...");
            try {
                RateLimitServer.this.stop();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                System.err.println("Shutdown interrupted: " + e.getMessage());
            }
        }));
    }

    /**
     * Stops the server gracefully.
     *
     * @throws InterruptedException if shutdown is interrupted
     */
    public void stop() throws InterruptedException {
        if (server != null) {
            server.shutdown().awaitTermination(SHUTDOWN_TIMEOUT_SECONDS, TimeUnit.SECONDS);
            System.out.println("RateLimitServer stopped.");
        }
    }

    /**
     * Blocks until server is terminated.
     *
     * @throws InterruptedException if waiting is interrupted
     */
    public void blockUntilShutdown() throws InterruptedException {
        if (server != null) {
            server.awaitTermination();
        }
    }

    /**
     * Returns the port the server is listening on.
     *
     * @return port number, or -1 if not started
     */
    public int getPort() {
        return server != null ? server.getPort() : -1;
    }

    /**
     * Creates default engine configuration.
     * Token Bucket: 100 capacity, 10 tokens/sec, max 10,000 keys.
     *
     * @return configured engine
     */
    private static RateLimiterEngine createDefaultEngine() {
        SystemClock clock = new SystemClock();
        RateLimiterConfig config = RateLimiterConfig.tokenBucket(100, 10.0);
        return new RateLimiterEngine(clock, config, 10_000);
    }

    /**
     * Main entry point.
     *
     * @param args Optional: port number
     * @throws IOException if server fails to start
     * @throws InterruptedException if server is interrupted
     */
    public static void main(String[] args) throws IOException, InterruptedException {
        int port = DEFAULT_PORT;

        if (args.length > 0) {
            try {
                port = Integer.parseInt(args[0]);
            } catch (NumberFormatException e) {
                System.err.println("Invalid port: " + args[0]);
                System.exit(1);
            }
        }

        RateLimitServer server = new RateLimitServer(port);
        server.start();
        server.blockUntilShutdown();
    }
}
