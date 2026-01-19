# Go Benchmarks

This directory documents and orchestrates the existing Go benchmarks in `go/pkg/`.

## Existing Benchmarks

### Algorithm Benchmarks (16 benchmarks)

Located in `go/pkg/algorithm/`:

**TokenBucket** (`tokenbucket/tokenbucket_test.go`):
- `BenchmarkTryAcquire_Allow` - Happy path (tokens available)
- `BenchmarkTryAcquire_Reject` - Rejected path (no tokens)
- `BenchmarkTryAcquire_Parallel` - Parallel execution with b.RunParallel()
- `BenchmarkNew` - Constructor performance

**FixedWindow** (`fixedwindow/fixedwindow_test.go`):
- `BenchmarkTryAcquire_Allow`
- `BenchmarkTryAcquire_Reject`
- `BenchmarkTryAcquire_Parallel`
- `BenchmarkNew`

**SlidingWindowLog** (`slidingwindow/slidingwindowlog_test.go`):
- `BenchmarkTryAcquire_Allow`
- `BenchmarkTryAcquire_Reject`
- `BenchmarkTryAcquire_Parallel`
- `BenchmarkNew`

**SlidingWindowCounter** (`slidingwindowcounter/slidingwindowcounter_test.go`):
- `BenchmarkTryAcquire_Allow`
- `BenchmarkTryAcquire_Reject`
- `BenchmarkTryAcquire_Parallel`
- `BenchmarkNew`

### Engine Benchmarks (3 benchmarks)

Located in `go/pkg/engine/engine_test.go`:
- `BenchmarkEngine_SingleKey` - Single key throughput
- `BenchmarkEngine_MultiKey` - Multi-key throughput (1000 keys)
- `BenchmarkEngine_Parallel` - Parallel throughput

### LRU Benchmarks (3 benchmarks)

Located in `go/pkg/engine/lru_test.go`:
- `BenchmarkLRU_Put` - Put operation
- `BenchmarkLRU_Get` - Get operation
- `BenchmarkLRU_PutIfAbsent` - Atomic put-if-absent

### Model Benchmarks (3 benchmarks)

Located in `go/pkg/model/model_test.go`:
- `BenchmarkAllow` - ALLOW result factory
- `BenchmarkReject` - REJECT result factory
- `BenchmarkReject_Negative` - REJECT with negative clamping

## Total: 25+ benchmarks

## Running Benchmarks

### Individual benchmark suite:
```bash
bazel run //go/pkg/algorithm/tokenbucket:tokenbucket_test -- -test.bench=. -test.benchmem
```

### All benchmarks via wrapper script:
```bash
bazel run //benchmarks/go:run_benchmarks
```

## Output

The wrapper script saves output to:
- `benchmarks/reports/latest/go_bench_raw.txt` - Raw Go benchmark output

Format:
```
BenchmarkName-8    5000000    250 ns/op    0 B/op    0 allocs/op
```
