package main

import (
	"fmt"
	"math"
	"strings"
)

// generateASCIIChart generates ASCII bar charts for throughput comparison
func generateASCIIChart(comparisons []Comparison) string {
	if len(comparisons) == 0 {
		return "No data to visualize.\n"
	}

	var sb strings.Builder

	sb.WriteString("### Throughput Comparison (ops/sec)\n\n")
	sb.WriteString("```\n")

	// Find max ops/sec for scaling
	maxOps := 0.0
	for _, c := range comparisons {
		if c.JavaOpsPerSec > maxOps {
			maxOps = c.JavaOpsPerSec
		}
		if c.GoOpsPerSec > maxOps {
			maxOps = c.GoOpsPerSec
		}
	}

	// Render bars for each benchmark
	for _, c := range comparisons {
		javaBar := renderBar(c.JavaOpsPerSec, maxOps, 40)
		goBar := renderBar(c.GoOpsPerSec, maxOps, 40)

		// Truncate name if too long
		name := c.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("%-30s\n", name))
		sb.WriteString(fmt.Sprintf("  Java %s (%.1fM ops/s)\n", javaBar, c.JavaOpsPerSec/1_000_000))

		speedupPercent := (c.Speedup - 1) * 100
		speedupSign := "+"
		if speedupPercent < 0 {
			speedupSign = ""
		}
		sb.WriteString(fmt.Sprintf("  Go   %s (%.1fM ops/s) [%s%.0f%%]\n\n",
			goBar, c.GoOpsPerSec/1_000_000, speedupSign, speedupPercent))
	}

	sb.WriteString("```\n")
	return sb.String()
}

// renderBar renders a horizontal bar chart
func renderBar(value, max float64, width int) string {
	if max == 0 {
		return strings.Repeat("░", width)
	}

	filled := int(math.Round((value / max) * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}
