package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// JavaBenchmark represents a JMH benchmark result
type JavaBenchmark struct {
	Name      string
	OpsPerSec float64
	NsPerOp   float64
}

// GoBenchmark represents a Go benchmark result
type GoBenchmark struct {
	Name        string
	OpsPerSec   float64
	NsPerOp     float64
	BytesPerOp  int64
	AllocsPerOp int64
}

// JMHResult represents the structure of JMH JSON output
type JMHResult struct {
	Benchmark     string `json:"benchmark"`
	PrimaryMetric struct {
		Score     float64 `json:"score"`
		ScoreUnit string  `json:"scoreUnit"`
	} `json:"primaryMetric"`
}

// loadJavaResults parses JMH JSON output
func loadJavaResults(path string) ([]JavaBenchmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Java results: %w", err)
	}

	var raw []JMHResult
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JMH JSON: %w", err)
	}

	results := make([]JavaBenchmark, 0, len(raw))
	for _, r := range raw {
		name := normalizeJavaName(r.Benchmark)
		opsPerSec := r.PrimaryMetric.Score
		nsPerOp := 1_000_000_000.0 / opsPerSec

		results = append(results, JavaBenchmark{
			Name:      name,
			OpsPerSec: opsPerSec,
			NsPerOp:   nsPerOp,
		})
	}

	return results, nil
}

// loadGoResults parses Go benchmark text output
func loadGoResults(path string) ([]GoBenchmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Go results: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Parse: BenchmarkName-8  5000000  250 ns/op  0 B/op  0 allocs/op
	re := regexp.MustCompile(`^(Benchmark\w+)-\d+\s+\d+\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)

	results := make([]GoBenchmark, 0)
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := normalizeGoName(matches[1])
		nsPerOp, _ := strconv.ParseFloat(matches[2], 64)
		opsPerSec := 1_000_000_000.0 / nsPerOp
		bytesPerOp, _ := strconv.ParseInt(matches[3], 10, 64)
		allocsPerOp, _ := strconv.ParseInt(matches[4], 10, 64)

		results = append(results, GoBenchmark{
			Name:        name,
			OpsPerSec:   opsPerSec,
			NsPerOp:     nsPerOp,
			BytesPerOp:  bytesPerOp,
			AllocsPerOp: allocsPerOp,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no Go benchmarks found in output")
	}

	return results, nil
}

// normalizeJavaName converts JMH benchmark name to canonical form
// "rl.benchmarks.java.AlgorithmBenchmark.tokenBucket_allow" → "tokenBucket_allow"
func normalizeJavaName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) == 0 {
		return name
	}
	// Return the last part (method name)
	return parts[len(parts)-1]
}

// normalizeGoName converts Go benchmark name to canonical form
// "BenchmarkTryAcquire_Allow" → "TryAcquire_Allow"
// We'll try to match against Java names by converting both to lowercase
func normalizeGoName(name string) string {
	// Remove "Benchmark" prefix
	return strings.TrimPrefix(name, "Benchmark")
}
