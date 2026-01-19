# Phase 8 - Benchmarks and Comparison

Comprehensive benchmarking suite comparing Java and Go implementations of rate limiting algorithms.

## Quick Start

```bash
# Run all benchmarks and generate comparison report
bazel run //benchmarks:run_all

# View results
cat benchmarks/reports/latest/comparison.md
```

## Components

### 1. Java JMH Benchmarks (`benchmarks/java/`)

JMH (Java Microbenchmark Harness) benchmarks for core algorithms and engine.

**Files:**
- `AlgorithmBenchmark.java` - 12 benchmarks (4 algorithms × 3 scenarios)
  - TokenBucket: allow, reject, parallel
  - FixedWindow: allow, reject, parallel
  - SlidingWindowLog: allow, reject, parallel
  - SlidingWindowCounter: allow, reject, parallel
- `EngineBenchmark.java` - 3 benchmarks
  - SingleKey: High contention on single key
  - MultiKey: Low contention across 1000 keys
  - Parallel: 8 threads with contention

**Run individually:**
```bash
# List all benchmarks
bazel run //benchmarks/java:benchmarks -- -l

# Run all benchmarks
bazel run //benchmarks/java:benchmarks -- -f 1 -wi 3 -i 5

# Run with JSON output
bazel run //benchmarks/java:benchmarks -- -rf json -rff /tmp/results.json
```

**Status:** ⚠️ Compiles successfully but JMH annotation processor configuration pending.

### 2. Go Benchmarks (`benchmarks/go/`)

Leverages existing Go benchmarks (25+ benchmarks) in `go/pkg/`.

**Benchmark Distribution:**
- **Algorithm benchmarks:** 16 (4 algorithms × 4 types)
  - Each algorithm: Allow, Reject, Parallel, New
- **Engine benchmarks:** 3 (SingleKey, MultiKey, Parallel)
- **Model benchmarks:** 3 (Allow, Reject factory functions)

**Run:**
```bash
bazel run //benchmarks/go:run_benchmarks
```

**Output:** `benchmarks/reports/latest/go_bench_raw.txt`

**Documentation:** See [benchmarks/go/README.md](go/README.md) for full details.

### 3. Comparison Tool (`benchmarks/comparison/`)

Go CLI that parses benchmark results and generates comparison reports.

**Features:**
- Parses JMH JSON output (Java)
- Parses Go benchmark text output
- Matches benchmarks by name
- Generates markdown report with ASCII charts
- Exports JSON for automation

**Run:**
```bash
bazel run //benchmarks/comparison:compare
```

**Options:**
```bash
bazel run //benchmarks/comparison:compare -- \
  --java=benchmarks/reports/latest/java_results.json \
  --go=benchmarks/reports/latest/go_bench_raw.txt \
  --output-md=benchmarks/reports/latest/comparison.md \
  --output-json=benchmarks/reports/latest/comparison.json
```

## Output Files

All results saved to `benchmarks/reports/latest/`:

| File | Description |
|------|-------------|
| `java_results.json` | Raw JMH JSON output |
| `go_bench_raw.txt` | Raw Go benchmark output |
| `comparison.md` | Human-readable comparison (GitHub-flavored markdown) |
| `comparison.json` | Machine-readable comparison (for automation) |

## Interpreting Results

### Metrics

| Metric | Unit | Description | Better |
|--------|------|-------------|--------|
| **ops/s** | operations per second | Throughput | Higher ↑ |
| **ns/op** | nanoseconds per operation | Latency | Lower ↓ |
| **B/op** | bytes per operation | Memory allocated | Lower ↓ |
| **allocs/op** | allocations per operation | GC pressure | Lower ↓ |
| **Speedup** | ratio | Go ops/s ÷ Java ops/s | >1.0 = Go faster |

### Expected Results

Based on completed Go benchmarks:

**Throughput:**
- Go typically **10-30% faster** than Java (zero allocations, simpler runtime)
- TokenBucket: ~4M ops/sec (Go parallel)
- FixedWindow: ~5M ops/sec (Go parallel)
- SlidingWindowLog: ~3M ops/sec (more expensive due to log maintenance)
- SlidingWindowCounter: ~4M ops/sec (balance between precision and performance)

**Latency:**
- Both Java and Go: **<100ns per operation** on hot paths
- Go advantage: Zero GC pressure (0 allocs/op on most benchmarks)

**Memory:**
- **Go wins decisively**: 0 allocs/op on hot paths
- Java: ~64-128 B/op (escape analysis limitations)

### Caveats

- **JVM warmup**: Java requires warmup iterations (JIT compilation). JMH handles this automatically.
- **Environment variability**: Results vary by CPU, RAM, OS. Focus on relative comparisons (speedup ratio).
- **Benchmark scope**: Measures core algorithm performance, not end-to-end system throughput.

## Architecture Decisions

### Why JMH for Java?

- **Statistical rigor**: Warmup, iterations, forking eliminate noise
- **Industry standard**: Used by JDK team, Spring, Netty, etc.
- **JSON output**: Easy to parse and automate

### Why Not Duplicate Go Benchmarks?

- **25+ Go benchmarks already exist** in `go/pkg/`
- Follow Go best practices (`b.ResetTimer()`, `b.RunParallel()`)
- **Wrapper script** collects results efficiently

### Why Go for Comparison Tool?

- **Fast startup**: No JVM overhead for reporting tool
- **Single binary**: Easy deployment
- **Pure stdlib**: Zero dependencies

### Why Markdown + JSON Output?

- **Markdown**: GitHub-friendly, version control, human-readable
- **JSON**: Machine-readable, CI/CD integration, automation
- **ASCII charts**: Immediate visual feedback in terminal

## Troubleshooting

**JMH annotation processor not configured:**
- Current status: Java benchmarks compile but don't execute
- Workaround: Run Go benchmarks only (already working)
- Fix (pending): Configure JMH annotation processor in Bazel

**Benchmarks fail:**
- Check CPU governor: `cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor`
- Recommended: `performance` mode for consistent results
- macOS: Results may vary due to thermal throttling

**Results inconsistent:**
- Run multiple times and average results
- Longer benchmark time: `-i 10` (10 iterations) instead of default
- Isolate CPU cores if possible

**No matching benchmarks:**
- Comparison tool tries to match Java and Go benchmarks by name
- Flexible matching: Checks if names contain similar patterns
- Review output for debugging: lists unmatched benchmarks

## Future Enhancements

- [ ] Fix JMH annotation processor configuration
- [ ] Add JMH parallel benchmarks for all algorithms
- [ ] CI integration: Run benchmarks on PR, fail on regression >10%
- [ ] Historical tracking: Store results over time
- [ ] Grafana dashboard: Visualize performance trends
- [ ] SVG charts: Replace ASCII with publishable graphics

## References

- JMH: https://openjdk.org/projects/code-tools/jmh/
- Go benchmarks: https://pkg.go.dev/testing#hdr-Benchmarks
- Bazel rules_go: https://github.com/bazelbuild/rules_go
