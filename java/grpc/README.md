# gRPC Rate Limiter Service

Thread-safe gRPC service exposing the RateLimiterEngine as a network API.

## Overview

This module provides:
- **Protobuf contract** (`proto/ratelimit.proto`) - Language-agnostic API definition
- **gRPC service** (`RateLimitServiceImpl`) - Thin wrapper over RateLimiterEngine
- **Standalone server** (`RateLimitServer`) - Production-ready server with graceful shutdown
- **Integration tests** (`RateLimitServiceImplTest`) - 7 tests with InProcessServer
- **Stress tests** (`RateLimitServiceStressTest`) - 6 throughput and latency benchmarks

## Architecture

```
┌─────────────────────────────────────────────┐
│  gRPC Client (Java/Go/Python/...)          │
└───────────────────┬─────────────────────────┘
                    │ CheckRateLimit RPC
                    ▼
┌─────────────────────────────────────────────┐
│  RateLimitServiceImpl                       │
│  - Request validation                       │
│  - Protobuf conversion                      │
│  - Error mapping                            │
└───────────────────┬─────────────────────────┘
                    │ tryAcquire(key, permits)
                    ▼
┌─────────────────────────────────────────────┐
│  RateLimiterEngine (Phase 4)                │
│  - Per-key locking (ReentrantLock)          │
│  - LRU cache (maxKeys)                      │
│  - Algorithm factory                        │
└───────────────────┬─────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│  Core Algorithms (Phase 1)                  │
│  - TokenBucket, FixedWindow, etc.           │
└─────────────────────────────────────────────┘
```

## Quick Start

### Running the Server

```bash
# Default port (9090)
bazel run //java/grpc:server

# Custom port
bazel run //java/grpc:server -- 8080
```

Output:
```
RateLimitServer started on port: 9090
```

### Testing

```bash
# Integration tests (InProcessServer)
bazel test //java/grpc:grpc_service_test

# Stress tests (throughput, latency)
bazel test //java/grpc:grpc_stress_test --test_output=all

# All gRPC tests
bazel test //java/grpc:...
```

## API Reference

### CheckRateLimit

Checks if a request is allowed under the rate limit.

**Request**:
```protobuf
message CheckRateLimitRequest {
  string key = 1;      // Rate limit key (e.g., "user:123")
  int32 permits = 2;   // Permits to acquire (must be > 0)
}
```

**Response**:
```protobuf
message CheckRateLimitResponse {
  bool allowed = 1;              // true if allowed, false if rejected
  int64 retry_after_nanos = 2;   // Nanoseconds to wait (0 if allowed)
}
```

**Errors**:
- `INVALID_ARGUMENT`: Empty key or permits <= 0
- `INTERNAL`: Unexpected server error

**Example** (Java client):
```java
ManagedChannel channel = ManagedChannelBuilder
    .forAddress("localhost", 9090)
    .usePlaintext()
    .build();

RateLimitServiceGrpc.RateLimitServiceBlockingStub stub =
    RateLimitServiceGrpc.newBlockingStub(channel);

CheckRateLimitRequest request = CheckRateLimitRequest.newBuilder()
    .setKey("user:alice")
    .setPermits(1)
    .build();

CheckRateLimitResponse response = stub.checkRateLimit(request);

if (response.getAllowed()) {
    // Process request
} else {
    long retryAfterMillis = response.getRetryAfterNanos() / 1_000_000;
    // Return 429 Too Many Requests with Retry-After header
}
```

### HealthCheck

Standard health check endpoint.

**Request**: Empty
**Response**: `SERVING` or `NOT_SERVING`

**Example**:
```java
HealthCheckRequest request = HealthCheckRequest.newBuilder().build();
HealthCheckResponse response = stub.healthCheck(request);
System.out.println("Status: " + response.getStatus()); // SERVING
```

## Configuration

### Server Configuration

Default configuration (Token Bucket):
- **Algorithm**: Token Bucket
- **Capacity**: 100 tokens
- **Refill rate**: 10 tokens/second
- **Max keys**: 10,000 (LRU eviction)

To customize, modify `RateLimitServer.createDefaultEngine()` or inject custom engine:

```java
SystemClock clock = new SystemClock();
RateLimiterConfig config = RateLimiterConfig.fixedWindow(
    50,                    // limit
    60_000_000_000L        // window size (1 minute in nanos)
);
RateLimiterEngine engine = new RateLimiterEngine(clock, config, 5000);

RateLimitServer server = new RateLimitServer(9090, engine);
server.start();
```

## Testing Philosophy

### Integration Tests (`RateLimitServiceImplTest`)

Uses **InProcessServer** for fast, deterministic tests:
- **ManualClock** for time control
- No network overhead
- Predictable results
- **7 tests**: validation, allow/reject, health check, multi-key isolation, refill

### Stress Tests (`RateLimitServiceStressTest`)

Uses **SystemClock** for realistic performance:
- **6 tests**: single/multi-threaded throughput, latency percentiles, contention
- InProcessServer (lower bound - no actual network)
- **Targets**:
  - Single-threaded: >10K req/s
  - Multi-threaded: >50K req/s
  - Latency p99: <1ms

## Performance

**Measured with InProcessServer** (lower bound, no network):

| Metric | Target | Typical Result |
|--------|--------|----------------|
| Single-threaded throughput | >10K req/s | ~100K-500K req/s* |
| Multi-threaded throughput (10 threads) | >50K req/s | ~200K-1M req/s* |
| Latency p99 | <1ms | ~50-200 μs* |

_*Results vary by hardware. InProcessServer has minimal overhead._

**Note**: Real network gRPC will have higher latency due to serialization/network overhead. Phase 6 load testing will measure actual network performance.

## Design Decisions

### Why Thin Wrapper?

`RateLimitServiceImpl` contains **zero business logic**:
- Validation happens first (fail-fast)
- All logic delegated to `RateLimiterEngine`
- Easy to test (mock engine)
- Clear separation of concerns

### Why InProcessServer for Tests?

- **Fast**: No network, no serialization overhead
- **Deterministic**: ManualClock control
- **Isolated**: No port conflicts
- **Full gRPC stack**: Tests interceptors, error handling, etc.

### Why No Batch Endpoint?

- **YAGNI**: Start simple, add when needed
- **Complexity**: Batching adds transaction semantics (all-or-nothing?)
- **Performance**: Single RPC already handles >50K req/s

Can add later if needed:
```protobuf
rpc CheckRateLimitBatch(stream CheckRateLimitRequest)
    returns (stream CheckRateLimitResponse);
```

### Error Handling Strategy

Rate limit rejection is **not an error**, it's a valid business response:
- `allowed=false` in response (not gRPC error)
- Only invalid inputs return `INVALID_ARGUMENT`
- Follows gRPC best practices

## Implementation Details

### Thread-Safety

- **Service layer**: Stateless (thread-safe by design)
- **Engine layer**: Per-key ReentrantLocks
- **Concurrency**: Handled entirely by engine, not service

### Graceful Shutdown

Server uses shutdown hook to handle Ctrl+C:
```java
Runtime.getRuntime().addShutdownHook(new Thread(() -> {
    server.shutdown().awaitTermination(5, TimeUnit.SECONDS);
}));
```

### Validation Order

1. Check key (empty check only - protobuf strings never null)
2. Check permits (must be > 0)
3. Execute engine operation
4. Convert result to protobuf
5. Send response

## Future Enhancements

- [ ] Configuration file support (YAML/JSON)
- [ ] Per-key algorithm configuration via RPC
- [ ] Admin endpoints (reset key, change config)
- [ ] Metrics export (Prometheus)
- [ ] Distributed mode with Redis/etcd
- [ ] TLS/mTLS support
- [ ] Circuit breaker pattern

## Future Phases

- **Phase 6**: Load testing tool in Go (stress test with real network)
- **Phase 7**: Go implementation sharing same `.proto`
- **Phase 8**: End-to-end benchmarks (Java gRPC vs Go gRPC)

## Files

```
java/grpc/
├── RateLimitServiceImpl.java       # gRPC service implementation
├── RateLimitServer.java             # Standalone server executable
├── RateLimitServiceImplTest.java   # Integration tests (7 tests)
├── RateLimitServiceStressTest.java # Stress tests (6 tests)
├── BUILD.bazel                      # Bazel targets
└── README.md                        # This file

proto/
├── ratelimit.proto                  # Protobuf contract
└── BUILD.bazel                      # Proto codegen rules
```

## Commands Reference

```bash
# Build
bazel build //java/grpc:grpc_service
bazel build //java/grpc:server

# Test
bazel test //java/grpc:grpc_service_test
bazel test //java/grpc:grpc_stress_test --test_output=all
bazel test //java/grpc:...

# Run
bazel run //java/grpc:server
bazel run //java/grpc:server -- 8080

# Clean
bazel clean
```

## Troubleshooting

### Port already in use
```
Error: Address already in use
```
**Solution**: Kill existing process or use different port:
```bash
lsof -ti:9090 | xargs kill -9
bazel run //java/grpc:server -- 8081
```

### Connection refused
```
UNAVAILABLE: io exception
```
**Solution**: Ensure server is running:
```bash
bazel run //java/grpc:server
```

### Slow tests
If stress tests are slow, they may timeout. Increase Bazel timeout:
```bash
bazel test //java/grpc:grpc_stress_test --test_timeout=300
```

## Related Documentation

- **Phase 4**: `/java/engine/README.md` - RateLimiterEngine design
- **Core Algorithms**: `/Readme.MD` - Algorithm trade-offs
- **Project Guide**: `/CLAUDE.md` - Build system and conventions
- **Roadmap**: `/.claude/PROJECT_ROADMAP.md` - Project status

---

**Phase 5 (gRPC API) - COMPLETE**
- Subfase 5.1: Protobuf contract + Bazel config ✅
- Subfase 5.2: gRPC service implementation ✅
- Subfase 5.3: Server + integration tests (7 tests) ✅
- Subfase 5.4: Stress tests (6 tests) + documentation ✅
