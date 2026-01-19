#!/bin/bash
set -e

OUTPUT_FILE="benchmarks/reports/latest/go_bench_raw.txt"

echo "=== Running Go Benchmarks ==="

# Create reports directory if it doesn't exist
mkdir -p benchmarks/reports/latest

# Run all benchmarks and save output
{
  echo "=== Algorithm Benchmarks ==="
  bazel run //go/pkg/algorithm/tokenbucket:tokenbucket_test -- -test.bench=. -test.benchmem 2>&1 | grep "^Benchmark"
  bazel run //go/pkg/algorithm/fixedwindow:fixedwindow_test -- -test.bench=. -test.benchmem 2>&1 | grep "^Benchmark"
  bazel run //go/pkg/algorithm/slidingwindow:slidingwindow_test -- -test.bench=. -test.benchmem 2>&1 | grep "^Benchmark"
  bazel run //go/pkg/algorithm/slidingwindowcounter:slidingwindowcounter_test -- -test.bench=. -test.benchmem 2>&1 | grep "^Benchmark"

  echo ""
  echo "=== Engine Benchmarks ==="
  bazel run //go/pkg/engine:engine_test -- -test.bench=. -test.benchmem 2>&1 | grep "^Benchmark"

  echo ""
  echo "=== Model Benchmarks ==="
  bazel run //go/pkg/model:model_test -- -test.bench=. -test.benchmem 2>&1 | grep "^Benchmark"
} > "$OUTPUT_FILE"

echo "✓ Go benchmarks saved to $OUTPUT_FILE"
echo ""
echo "Results:"
cat "$OUTPUT_FILE"
