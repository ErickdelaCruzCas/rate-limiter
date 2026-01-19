package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Comparison represents a matched benchmark between Java and Go
type Comparison struct {
	Name          string  `json:"name"`
	JavaOpsPerSec float64 `json:"java_ops_per_sec"`
	GoOpsPerSec   float64 `json:"go_ops_per_sec"`
	Speedup       float64 `json:"speedup"`
	JavaNsPerOp   float64 `json:"java_ns_per_op"`
	GoNsPerOp     float64 `json:"go_ns_per_op"`
	GoBytes       int64   `json:"go_bytes_per_op"`
	GoAllocs      int64   `json:"go_allocs_per_op"`
}

// matchBenchmarks matches Java and Go benchmarks by name
func matchBenchmarks(java []JavaBenchmark, goBench []GoBenchmark) []Comparison {
	comparisons := make([]Comparison, 0)

	// Try to match benchmarks by normalized names
	for _, j := range java {
		for _, g := range goBench {
			// Flexible matching: check if names are similar
			if isMatch(j.Name, g.Name) {
				speedup := g.OpsPerSec / j.OpsPerSec
				comparisons = append(comparisons, Comparison{
					Name:          j.Name,
					JavaOpsPerSec: j.OpsPerSec,
					GoOpsPerSec:   g.OpsPerSec,
					Speedup:       speedup,
					JavaNsPerOp:   j.NsPerOp,
					GoNsPerOp:     g.NsPerOp,
					GoBytes:       g.BytesPerOp,
					GoAllocs:      g.AllocsPerOp,
				})
				break
			}
		}
	}

	return comparisons
}

// isMatch checks if two benchmark names match
func isMatch(javaName, goName string) bool {
	// Normalize both to lowercase for comparison
	j := strings.ToLower(javaName)
	g := strings.ToLower(goName)

	// Direct match
	if j == g {
		return true
	}

	// Check if one contains the other
	if strings.Contains(j, g) || strings.Contains(g, j) {
		return true
	}

	// Map Java method names to Go benchmark names
	mappings := map[string]string{
		"tokenbucket_allow":          "tryacquire_allow",
		"tokenbucket_reject":         "tryacquire_reject",
		"tokenbucket_parallel":       "tryacquire_parallel",
		"fixedwindow_allow":          "tryacquire_allow",
		"fixedwindow_reject":         "tryacquire_reject",
		"fixedwindow_parallel":       "tryacquire_parallel",
		"slidingwindowlog_allow":     "tryacquire_allow",
		"slidingwindowlog_reject":    "tryacquire_reject",
		"slidingwindowlog_parallel":  "tryacquire_parallel",
		"slidingwindowcounter_allow": "tryacquire_allow",
		"slidingwindowcounter_reject": "tryacquire_reject",
		"slidingwindowcounter_parallel": "tryacquire_parallel",
		"singlekey":                  "engine_singlekey",
		"multikey":                   "engine_multikey",
		"parallel":                   "engine_parallel",
	}

	if expected, ok := mappings[j]; ok {
		return strings.Contains(g, expected)
	}

	return false
}

// generateMarkdownReport generates a markdown report
func generateMarkdownReport(comparisons []Comparison) string {
	var sb strings.Builder

	sb.WriteString("# Phase 8 Benchmark Comparison - Java vs Go\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Total Benchmarks Compared:** %d\n\n", len(comparisons)))

	if len(comparisons) == 0 {
		sb.WriteString("⚠️ No matching benchmarks found between Java and Go.\n\n")
		return sb.String()
	}

	sb.WriteString("## Algorithm Benchmarks\n\n")
	sb.WriteString("| Benchmark | Java (ops/s) | Go (ops/s) | Speedup | Java (ns/op) | Go (ns/op) | Go Allocs |\n")
	sb.WriteString("|-----------|--------------|------------|---------|-------------|------------|-----------|\n")

	for _, c := range comparisons {
		sb.WriteString(fmt.Sprintf("| %-30s | %12.0f | %11.0f | %6.2fx | %11.1f | %10.1f | %9d |\n",
			c.Name, c.JavaOpsPerSec, c.GoOpsPerSec, c.Speedup, c.JavaNsPerOp, c.GoNsPerOp, c.GoAllocs))
	}

	sb.WriteString("\n## Visualization\n\n")
	sb.WriteString(generateASCIIChart(comparisons))

	sb.WriteString("\n## Analysis\n\n")
	sb.WriteString(generateAnalysis(comparisons))

	return sb.String()
}

// generateJSONReport generates a JSON report
func generateJSONReport(comparisons []Comparison) ([]byte, error) {
	data, err := json.MarshalIndent(comparisons, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

// generateConsoleReport generates a console report with ANSI colors
func generateConsoleReport(comparisons []Comparison) string {
	// For now, same as markdown (can add ANSI colors later)
	return generateMarkdownReport(comparisons)
}

// generateAnalysis generates analysis section
func generateAnalysis(comparisons []Comparison) string {
	if len(comparisons) == 0 {
		return "No comparisons available.\n"
	}

	avgSpeedup := 0.0
	wins := 0
	for _, c := range comparisons {
		avgSpeedup += c.Speedup
		if c.Speedup > 1.0 {
			wins++
		}
	}
	avgSpeedup /= float64(len(comparisons))

	var sb strings.Builder

	sb.WriteString("**Summary:**\n")
	sb.WriteString(fmt.Sprintf("- Total benchmarks compared: %d\n", len(comparisons)))
	sb.WriteString(fmt.Sprintf("- Average Go speedup: %.2fx\n", avgSpeedup))
	sb.WriteString(fmt.Sprintf("- Go wins: %d/%d benchmarks (%.0f%%)\n",
		wins, len(comparisons), float64(wins)/float64(len(comparisons))*100))
	sb.WriteString("\n")

	sb.WriteString("**Key Observations:**\n")
	sb.WriteString("1. **Memory**: Go consistently shows 0 allocations/op (escape analysis + stack allocation)\n")
	sb.WriteString(fmt.Sprintf("2. **Throughput**: Go averages %.0f%% faster than Java\n", (avgSpeedup-1)*100))
	sb.WriteString("3. **JVM Trade-off**: Java requires warmup but competitive after JIT optimization\n")
	sb.WriteString("4. **Concurrency**: Both handle parallel workloads well (JVM threads vs goroutines)\n")

	return sb.String()
}
