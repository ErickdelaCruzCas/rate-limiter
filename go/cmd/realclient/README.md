# Real Client - Rate-Limited HTTP Client with Go Concurrency Patterns

A demonstration of production-grade Go concurrency patterns integrated with a gRPC rate limiter, making real HTTP requests to external APIs.

## Overview

This package demonstrates:
- ✅ **Rate limiting integration** - gRPC calls to rate limiter before each request
- ✅ **Real HTTP requests** - Calls to actual public APIs (JSONPlaceholder, HTTPBin, etc.)
- ✅ **Worker pool pattern** - Fixed goroutines processing job queue
- ✅ **Pipeline pattern** - Multi-stage processing with channel chaining
- ✅ **Context cancellation** - Graceful shutdown and timeout handling
- ✅ **Metrics collection** - Tracks allowed, rejected, failed requests
- ✅ **Retry logic** - Automatic retry with exponential backoff on rate limit rejection

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLI Application                            │
│  • Mode selection (sequential, worker pool, pipeline)               │
│  • Configuration (server, key, count, workers)                      │
│  • Metrics reporting                                                │
└────────────────────────┬────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     RateLimitedClient                               │
│  • Check rate limiter (gRPC)                                        │
│  • If ALLOW → Make HTTP request                                     │
│  • If REJECT → Wait retry-after duration, retry (max 3)             │
│  • Track metrics (allowed, rejected, failed)                        │
└────────────────────────┬────────────────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         │                               │
         ▼                               ▼
┌──────────────────┐          ┌──────────────────┐
│  Rate Limiter    │          │   Public APIs    │
│  (gRPC Service)  │          │  • JSONPlaceholder
│  :50051          │          │  • HTTPBin       │
└──────────────────┘          │  • Dog API       │
                              │  • PokeAPI       │
                              │  • Cat Facts     │
                              │  • UUID Generator│
                              │  • Advice Slip   │
                              └──────────────────┘
```

## Quick Start

### Sequential Mode (One at a Time)

```bash
# Start rate limiter server first
bazel run //go/cmd/server

# Run client in sequential mode
bazel run //go/cmd/realclient -- \
  --server=localhost:50051 \
  --mode=sequential \
  --count=10
```

### Worker Pool Mode (Concurrent Workers)

```bash
bazel run //go/cmd/realclient -- \
  --server=localhost:50051 \
  --mode=worker \
  --workers=5 \
  --count=20
```

### Pipeline Mode (Multi-Stage Processing)

```bash
bazel run //go/cmd/realclient -- \
  --server=localhost:50051 \
  --mode=pipeline \
  --workers=5 \
  --count=20
```

### Target Selection

```bash
# Specific target
bazel run //go/cmd/realclient -- \
  --target=jsonplaceholder-posts \
  --count=5

# Random targets (default)
bazel run //go/cmd/realclient -- \
  --count=10
```

## Concurrency Patterns

### 1. Worker Pool Pattern

**Purpose:** Limit concurrency to prevent overwhelming the rate limiter or target APIs.

**How it works:**
```
jobs channel → [Worker 1] → results channel
               [Worker 2] →
               [Worker 3] →
               [Worker N] →
```

**Code example:**
```go
pool := patterns.NewWorkerPool(client, 5) // 5 workers
defer pool.Close()

// Submit jobs
for _, target := range targets {
    pool.Submit(target)
}
pool.CloseJobs()

// Collect results
for result := range pool.Results() {
    if result.Error != nil {
        log.Printf("Failed: %v", result.Error)
    }
}
```

**Key features:**
- Fixed number of goroutines (prevents goroutine explosion)
- Buffered channels for backpressure
- Graceful shutdown with context cancellation
- Per-worker statistics

**When to use:**
- High concurrency without overwhelming downstream systems
- Rate-limited APIs where you want N concurrent requests max
- Long-running background jobs

### 2. Pipeline Pattern

**Purpose:** Multi-stage processing where each stage can run concurrently.

**How it works:**
```
Stage 1: Generate → Stage 2: Fetch → Stage 3: Validate → Results
         (targets)           (HTTP)            (status)
```

**Code example:**
```go
pipeline := patterns.NewPipeline(client, 5) // 5 workers per stage
results := pipeline.Run(ctx, targets)

for result := range results {
    log.Printf("Result: %+v", result)
}
```

**Key features:**
- Multi-stage channel chaining
- Concurrent workers at each stage
- Context propagation through all stages
- Graceful shutdown on context cancel

**When to use:**
- Multi-step processing (fetch → transform → validate → store)
- CPU-intensive operations between I/O stages
- Need to visualize data flow through system

### 3. Sequential Mode

**Purpose:** Simple mode for debugging or low-rate requests.

**How it works:**
```
Request 1 → Wait → Request 2 → Wait → Request 3 → ...
```

**When to use:**
- Debugging rate limiting behavior
- Low request rates (< 10 req/s)
- Want to see detailed output for each request

## Components

### pkg/realclient

**`Client`** - Rate-limited HTTP client

Methods:
- `NewClient(serverAddr, key)` - Create client
- `Fetch(target)` - Make rate-limited request
- `FetchWithContext(ctx, target)` - With context support
- `Metrics()` - Get statistics
- `Close()` - Cleanup

Features:
- Automatic retry on rate limit rejection (max 3 attempts)
- Configurable retry-after duration from rate limiter
- Context cancellation support
- Thread-safe metrics collection

### pkg/patterns

**`WorkerPool`** - Fixed worker pool

Methods:
- `NewWorkerPool(client, numWorkers)` - Create pool
- `Submit(target)` - Submit job
- `CloseJobs()` - Signal no more jobs
- `Results()` - Get results channel
- `Stats()` - Get statistics
- `Close()` - Graceful shutdown

**`Pipeline`** - Multi-stage processor

Methods:
- `NewPipeline(client, workersPerStage)` - Create pipeline
- `Run(ctx, targets)` - Execute pipeline

### pkg/targets

**`Target`** - HTTP endpoint configuration

Fields:
- `Name` - Human-readable identifier
- `URL` - Full HTTP(S) endpoint
- `Method` - HTTP method (GET, POST, etc.)
- `Description` - What this endpoint does
- `ExpectedStatus` - Expected HTTP status code

Functions:
- `All()` - Get all targets
- `GetTarget(name)` - Get specific target
- `Random()` - Get random target
- `Fast()` - Get targets with low latency

Available targets:
- `jsonplaceholder-posts` - Fake blog posts
- `jsonplaceholder-users` - Fake users
- `httpbin-get` - Echo service
- `httpbin-delay` - Delayed response (1s)
- `randomuser` - Random user data
- `dog-api` - Random dog images
- `pokeapi-pikachu` - Pokemon data
- `cat-facts` - Random cat facts
- `uuid` - UUID generator
- `advice` - Random advice

## Metrics

The client tracks detailed metrics:

```go
type Metrics struct {
    Allowed           int64  // Requests allowed by rate limiter
    Rejected          int64  // Requests rejected by rate limiter
    Failed            int64  // Requests that failed (HTTP errors, etc.)
    TotalHTTPRequests int64  // Successful HTTP requests made
    TotalRetries      int64  // Total retry attempts after rejection
}
```

Example output:
```
=== Final Report ===
Duration: 5.234s
Throughput: 3.82 req/s

HTTP Requests:
  Success: 18
  Errors:  2
  Total:   20

Rate Limiter:
  Allowed:  20
  Rejected: 5 (retries: 5)
  Failed:   0

Rejection Rate: 20.00%
```

## Configuration Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `localhost:50051` | Rate limiter gRPC server address |
| `--key` | `realclient:demo` | Rate limit key (user:123, api:key, etc.) |
| `--mode` | `sequential` | Execution mode: `sequential`, `worker`, `pipeline` |
| `--target` | (random) | Specific target to fetch (see `targets.Names()`) |
| `--count` | `10` | Number of requests to make |
| `--workers` | `5` | Number of concurrent workers (worker/pipeline mode) |
| `--timeout` | `60s` | Overall timeout for all requests |

## Testing

### Unit Tests

```bash
# Test targets package
bazel test //go/pkg/targets:targets_test

# Test realclient package
bazel test //go/pkg/realclient:realclient_test

# Test patterns package
bazel test //go/pkg/patterns:patterns_test
```

### Integration Tests

Integration tests require the gRPC rate limiter server to be running:

```bash
# Terminal 1: Start server
bazel run //go/cmd/server

# Terminal 2: Run integration tests
bazel test //go/pkg/realclient:realclient_test
bazel test //go/pkg/patterns:patterns_test
```

Tests will skip if server is not available.

### E2E Test

```bash
# Terminal 1: Start server with low rate limit
bazel run //go/cmd/server -- \
  --algorithm=token_bucket \
  --capacity=10 \
  --refill_rate=2.0

# Terminal 2: Run client with high request rate
bazel run //go/cmd/realclient -- \
  --mode=worker \
  --workers=10 \
  --count=50

# Observe rate limiting in action
# You should see rejected requests with automatic retry
```

## Real-World Use Cases

### 1. API Gateway Rate Limiting

```bash
# Simulate API gateway with 100 req/s limit
bazel run //go/cmd/server -- \
  --algorithm=token_bucket \
  --capacity=100 \
  --refill_rate=100.0

# Client with bursts
bazel run //go/cmd/realclient -- \
  --mode=worker \
  --workers=20 \
  --count=200
```

### 2. Scraper with Politeness

```bash
# Scraper that respects rate limits
bazel run //go/cmd/server -- \
  --algorithm=fixed_window \
  --window_seconds=60 \
  --limit=30

# Scraper client
bazel run //go/cmd/realclient -- \
  --mode=sequential \
  --count=100 \
  --target=jsonplaceholder-posts
```

### 3. Load Testing

```bash
# High-capacity server
bazel run //go/cmd/server -- \
  --algorithm=sliding_window_counter \
  --window_seconds=10 \
  --bucket_seconds=1 \
  --limit=1000

# Load test
bazel run //go/cmd/realclient -- \
  --mode=worker \
  --workers=50 \
  --count=2000
```

## Performance Characteristics

**Sequential Mode:**
- Throughput: ~1-5 req/s (depends on target API latency)
- Memory: O(1)
- CPU: Very low
- Best for: Debugging, low-rate requests

**Worker Pool Mode:**
- Throughput: ~50-200 req/s (depends on workers and rate limit)
- Memory: O(workers)
- CPU: Medium
- Best for: Production, controlled concurrency

**Pipeline Mode:**
- Throughput: ~100-300 req/s (depends on workers per stage)
- Memory: O(workers * stages)
- CPU: Medium-High
- Best for: Multi-stage processing, transformations

## Graceful Shutdown

All modes support graceful shutdown:

```bash
# Press Ctrl+C to trigger shutdown
^C
Received interrupt signal, shutting down gracefully...
Context canceled, stopping...

=== Final Report ===
# ... partial results reported
```

Features:
- Context cancellation propagates to all goroutines
- In-flight HTTP requests complete or timeout
- Worker pools drain remaining jobs
- Final metrics always printed

## Error Handling

The client handles various error scenarios:

**Rate limit rejection:**
- Waits `retry-after` duration
- Retries up to 3 times
- Increments `Rejected` and `TotalRetries` metrics

**HTTP errors:**
- Network errors, timeouts, DNS failures
- Increments `Failed` metric
- Logs error details

**Context cancellation:**
- User interrupt (Ctrl+C)
- Timeout exceeded
- Graceful shutdown, partial results reported

## Best Practices

1. **Choose appropriate worker count:**
   - Too few: Underutilize rate limit capacity
   - Too many: Overhead from goroutine context switching
   - Recommended: 5-20 for most use cases

2. **Use context timeouts:**
   - Always set `--timeout` flag
   - Prevents infinite hangs on network issues
   - Recommended: 2x expected duration

3. **Monitor rejection rate:**
   - High rejection rate (>20%) = workers > rate limit capacity
   - Low rejection rate (<5%) = may be able to increase workers
   - Optimal: 5-10% rejection rate

4. **Target selection:**
   - Use `--target` for specific API testing
   - Use random targets for diverse latency patterns
   - Avoid `httpbin-delay` in benchmarks (intentional 1s delay)

## Future Enhancements

- [ ] Prometheus metrics export
- [ ] Circuit breaker pattern integration
- [ ] Exponential backoff configuration
- [ ] Request payload support (POST/PUT)
- [ ] Response body validation
- [ ] Distributed tracing (OpenTelemetry)
- [ ] Custom target configuration file

## License

Study material for educational purposes.

---

**Questions?** Check the godoc: `godoc -http=:6060`

**Issues?** This is a demonstration project showcasing Go concurrency patterns with real-world rate limiting.
