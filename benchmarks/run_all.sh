#!/bin/bash
set -e

REPORTS_DIR="benchmarks/reports/latest"
mkdir -p "$REPORTS_DIR"

echo "==================================================="
echo "  Phase 8: Benchmark Comparison (Java vs Go)"
echo "==================================================="
echo ""

# Step 1: Run Java JMH Benchmarks
echo "[1/3] Running Java JMH benchmarks..."
echo "Note: JMH annotation processor setup pending - skipping Java benchmarks for now"
echo "TODO: Configure JMH annotation processor to enable Java benchmarks"
# bazel run //benchmarks/java:benchmarks -- \
#   -rf json \
#   -rff "$REPORTS_DIR/java_results.json" \
#   -f 1 \
#   -wi 3 \
#   -i 5

# Create empty Java results for now
echo '[]' > "$REPORTS_DIR/java_results.json"
echo "⚠️  Java benchmarks skipped (annotation processor configuration pending)"
echo ""

# Step 2: Run Go Benchmarks
echo "[2/3] Running Go benchmarks..."
bazel run //benchmarks/go:run_benchmarks

echo "✓ Go benchmarks saved to $REPORTS_DIR/go_bench_raw.txt"
echo ""

# Step 3: Generate Comparison Report
echo "[3/3] Generating comparison report..."
bazel run //benchmarks/comparison:compare

echo ""
echo "==================================================="
echo "  ✓ Benchmarks Complete!"
echo "==================================================="
echo ""
echo "Reports available at:"
echo "  Markdown: $REPORTS_DIR/comparison.md"
echo "  JSON:     $REPORTS_DIR/comparison.json"
echo ""
echo "View report:"
echo "  cat $REPORTS_DIR/comparison.md"
