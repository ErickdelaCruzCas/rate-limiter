# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **rate limiter cookbook** built as a technical study project for system design interviews. It demonstrates distributed systems concepts, concurrency patterns, and modern build systems using Bazel.

The project implements multiple rate limiting algorithms (Token Bucket, Fixed Window, Sliding Window Log, Sliding Window Counter) with a focus on:
- Clean separation of concerns (algorithm, time, I/O, concurrency)
- Deterministic testing with clock abstraction
- Incremental complexity (single-node → distributed)
- Build reproducibility with Bazel

**Current status**: Phases 1-2 complete (core algorithms + testing infrastructure). Phases 3-8 pending (performance testing, Java engine, gRPC, Go implementation, benchmarks).

## Build System: Bazel Commands

This project uses **Bazel** (not Maven/Gradle) as the build system.

### Running Tests

```bash
# Run all tests in core algorithms
bazel test //core/algorithms/...

# Run specific algorithm tests
bazel test //core/algorithms/token_bucket:token_bucket_test
bazel test //core/algorithms/fixed_window:fixed_window_test
bazel test //core/algorithms/sliding_window:sliding_window_log_test
bazel test //core/algorithms/sliding_window_counter:sliding_window_counter_test

# View detailed test output
bazel test //core/algorithms/... --test_output=all

# Clean build artifacts
bazel clean
```

### Building Targets

```bash
# Build all core libraries
bazel build //core/...

# Build specific algorithm
bazel build //core/algorithms/token_bucket:token_bucket
```

### Important Bazel Notes

- Each module has its own `BUILD.bazel` file defining targets
- Tests use JUnit 5 with custom configuration (`use_testrunner = False`, `main_class = org.junit.platform.console.ConsoleLauncher`)
- Dependencies are explicit - see `MODULE.bazel` for external dependencies
- Bazel caches build artifacts aggressively - only changed targets rebuild
- Java 21 is configured in `.bazelrc`

## Architecture

### Core Design Principles

1. **Pure core algorithms** (`/core/algorithms/`) - no I/O, no threads, no real time
2. **Clock abstraction** (`/core/clock/`) - deterministic testing via `Clock` interface
3. **Clean model layer** (`/core/model/`) - `RateLimiter` interface, `Decision`, `RateLimitResult`
4. **Each algorithm is independent** - separate Bazel target with isolated tests

### Module Structure

```
core/
├── clock/              # Time abstraction (Clock, ManualClock)
├── model/              # Core interfaces and data types
└── algorithms/         # Rate limiting implementations
    ├── token_bucket/        # Continuous refill, burst-friendly
    ├── fixed_window/        # Simple but has boundary problem
    ├── sliding_window/      # Precise but O(permits) memory
    └── sliding_window_counter/  # Practical approximation
```

### Key Interfaces

**RateLimiter** (`core/model/RateLimiter.java`):
```java
public interface RateLimiter {
    RateLimitResult tryAcquire(String key, int permits);
}
```

**Clock** (`core/clock/Clock.java`):
```java
public interface Clock {
    long nowNanos();  // Monotonic time for deterministic testing
}
```

### Algorithm Trade-offs

| Algorithm | Memory | Precision | Complexity | Use Case |
|-----------|--------|-----------|------------|----------|
| **Token Bucket** | O(1) | Good | O(1) | Default choice - burst-friendly, continuous refill |
| **Fixed Window** | O(1) | Poor (boundary problem) | O(1) | Simple but can allow 2x rate at boundaries |
| **Sliding Window Log** | O(limit) | Exact | O(permits) | High precision needed, acceptable memory cost |
| **Sliding Window Counter** | O(buckets) | Approximation | O(1) | Balance between Fixed Window and Sliding Window |

## Testing Philosophy

### Deterministic Testing with ManualClock

All tests use `ManualClock` for time control:

```java
ManualClock clock = new ManualClock();
TokenBucket bucket = new TokenBucket(clock, 10, 1.0);

clock.advanceNanos(1_000_000_000L);  // Advance 1 second
// No sleeps, no flakiness, reproducible 100%
```

**Key points**:
- Never use `System.currentTimeMillis()` or `System.nanoTime()` in core algorithms
- Time is always injected via the `Clock` interface
- Tests are fast and never sleep
- Results are deterministic across runs

### Test Structure

Each algorithm has ~5 functional tests covering:
1. Basic allow/reject behavior
2. Refill/reset mechanics
3. Edge cases (zero permits, boundary conditions)
4. Retry-after calculation
5. Burst handling (for Token Bucket)

## Development Workflow

### Adding a New Algorithm

1. Create directory: `core/algorithms/new_algorithm/`
2. Create `BUILD.bazel` with `java_library` and `java_test` targets
3. Implement algorithm class extending `RateLimiter`
4. Inject `Clock` dependency for time control
5. Write tests using `ManualClock`
6. Add to `//core/algorithms/...` wildcard for test runs

### JUnit 5 Configuration Pattern

All test targets must use this pattern:

```python
java_test(
    name = "test_name",
    srcs = ["TestClass.java"],
    use_testrunner = False,
    main_class = "org.junit.platform.console.ConsoleLauncher",
    args = ["--select-class=rl.core.algorithms.package.TestClass"],
    deps = [
        ":library_under_test",
        "@maven//:org_junit_jupiter_junit_jupiter_api",
        "@maven//:org_junit_jupiter_junit_jupiter_engine",
        "@maven//:org_junit_platform_junit_platform_launcher",
        "@maven//:org_junit_platform_junit_platform_console",
        "@maven//:org_junit_platform_junit_platform_reporting",
    ],
)
```

**Why this is necessary**: Bazel's default test runner is JUnit 4. JUnit 5 requires explicit JUnit Platform Console Launcher configuration.

## Future Phases (Not Yet Implemented)

The roadmap includes:

- **Phase 3**: JMH performance benchmarks for core algorithms
- **Phase 4**: Thread-safe Java engine with `ConcurrentHashMap` and lock-based synchronization
- **Phase 5**: gRPC API with Protobuf codegen
- **Phase 6**: Go implementation sharing same `.proto` contract
- **Phase 7**: Load testing tool in Go
- **Phase 8**: End-to-end benchmarks and Java vs Go comparison

When implementing future phases:
- Each phase adds **one dimension of complexity** (don't mix concerns)
- Performance testing (Phase 3) validates core before building engine (Phase 4)
- Maintain separation: algorithm layer remains pure, concurrency lives in engine layer
- Document trade-offs explicitly in code comments

## Code Style and Conventions

- **Package naming**: `rl.core.*` for core modules
- **Visibility**: Use `visibility = ["//core:__subpackages__"]` to restrict access
- **Clock injection**: Always inject `Clock` in constructor, never use static time methods
- **Error handling**: Throw `IllegalArgumentException` for invalid inputs (permits <= 0, capacity <= 0)
- **Comments**: Document algorithm trade-offs and design decisions directly in code
- **Simplicity over abstraction**: No frameworks, no external libraries beyond JUnit

## Interview Context

This project is designed to demonstrate:

1. **System design thinking**: Trade-offs between algorithms, single-node vs distributed
2. **Concurrency expertise**: Lock granularity, atomics, thread-safety patterns
3. **Build system proficiency**: Bazel targets, hermetic builds, dependency management
4. **Testing rigor**: Deterministic tests, clock injection pattern, edge case coverage
5. **Performance engineering**: (future) JMH benchmarks, profiling, optimization

When discussing this project in interviews, focus on:
- Why each algorithm was chosen and its trade-offs
- How `Clock` abstraction enables deterministic testing
- Why Bazel over Maven (hermetic builds, polyglot support, caching)
- Incremental complexity: core → concurrency → networking → distribution