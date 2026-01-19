package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Command-line flags
	javaFile := flag.String("java", "benchmarks/reports/latest/java_results.json", "Path to JMH JSON results")
	goFile := flag.String("go", "benchmarks/reports/latest/go_bench_raw.txt", "Path to Go benchmark output")
	outputMd := flag.String("output-md", "benchmarks/reports/latest/comparison.md", "Path to markdown output")
	outputJSON := flag.String("output-json", "benchmarks/reports/latest/comparison.json", "Path to JSON output")
	flag.Parse()

	fmt.Println("=================================================")
	fmt.Println("  Phase 8: Benchmark Comparison Tool")
	fmt.Println("=================================================")
	fmt.Println()

	// Load Java results
	fmt.Printf("Loading Java results from: %s\n", *javaFile)
	javaResults, err := loadJavaResults(*javaFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading Java results: %v\n", err)
		fmt.Println("Skipping Java benchmarks (file may not exist yet)")
		javaResults = []JavaBenchmark{}
	} else {
		fmt.Printf("✓ Loaded %d Java benchmarks\n", len(javaResults))
	}

	// Load Go results
	fmt.Printf("Loading Go results from: %s\n", *goFile)
	goResults, err := loadGoResults(*goFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading Go results: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Loaded %d Go benchmarks\n", len(goResults))
	fmt.Println()

	// Match benchmarks
	comparisons := matchBenchmarks(javaResults, goResults)
	fmt.Printf("Matched %d benchmarks between Java and Go\n", len(comparisons))
	fmt.Println()

	if len(comparisons) == 0 {
		fmt.Println("⚠️ No matching benchmarks found.")
		fmt.Println("Java benchmarks:", len(javaResults))
		fmt.Println("Go benchmarks:", len(goResults))
		if len(javaResults) > 0 {
			fmt.Println("\nJava benchmark names:")
			for _, j := range javaResults {
				fmt.Printf("  - %s\n", j.Name)
			}
		}
		if len(goResults) > 0 {
			fmt.Println("\nGo benchmark names:")
			for _, g := range goResults {
				fmt.Printf("  - %s\n", g.Name)
			}
		}
	}

	// Generate markdown report
	markdown := generateMarkdownReport(comparisons)
	if err := os.WriteFile(*outputMd, []byte(markdown), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing markdown report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Markdown report saved to: %s\n", *outputMd)

	// Generate JSON report
	jsonData, err := generateJSONReport(comparisons)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating JSON report: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outputJSON, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ JSON report saved to: %s\n", *outputJSON)

	fmt.Println()
	fmt.Println("=================================================")
	fmt.Println("  Comparison Complete!")
	fmt.Println("=================================================")
	fmt.Println()

	// Print summary to console
	fmt.Println(generateConsoleReport(comparisons))
}
