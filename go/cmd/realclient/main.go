// realclient is a CLI tool that demonstrates rate-limited HTTP requests with
// Go concurrency patterns.
//
// This binary shows real-world integration of:
//   - gRPC rate limiter service
//   - Real HTTP requests to public APIs
//   - Worker pool pattern (fan-out/fan-in)
//   - Pipeline pattern (multi-stage processing)
//   - Context cancellation and timeouts
//   - Metrics and reporting
//
// Usage:
//
//	# Sequential mode (one request at a time)
//	bazel run //go/cmd/realclient -- \
//	  --server=localhost:50051 \
//	  --mode=sequential \
//	  --count=10
//
//	# Worker pool mode (concurrent workers)
//	bazel run //go/cmd/realclient -- \
//	  --server=localhost:50051 \
//	  --mode=worker \
//	  --workers=5 \
//	  --count=20
//
//	# Pipeline mode (multi-stage processing)
//	bazel run //go/cmd/realclient -- \
//	  --server=localhost:50051 \
//	  --mode=pipeline \
//	  --workers=5 \
//	  --count=20
//
//	# Specific target
//	bazel run //go/cmd/realclient -- \
//	  --server=localhost:50051 \
//	  --target=jsonplaceholder-posts \
//	  --count=5
//
// Features:
//   - Multiple concurrency modes (sequential, worker pool, pipeline)
//   - Configurable number of workers
//   - Target selection (specific or random)
//   - Metrics reporting (allowed, rejected, failed)
//   - Graceful shutdown (Ctrl+C)
//   - Context timeout support
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ratelimiter/go/pkg/patterns"
	"github.com/ratelimiter/go/pkg/realclient"
	"github.com/ratelimiter/go/pkg/targets"
)

var (
	// Server configuration
	serverAddr   = flag.String("server", "localhost:50051", "Rate limiter gRPC server address")
	rateLimitKey = flag.String("key", "realclient:demo", "Rate limit key to use")

	// Execution mode
	mode = flag.String("mode", "sequential", "Execution mode: sequential, worker, pipeline")

	// Request configuration
	targetName = flag.String("target", "", "Specific target to fetch (empty = random)")
	count      = flag.Int("count", 10, "Number of requests to make")
	workers    = flag.Int("workers", 5, "Number of concurrent workers (for worker/pipeline mode)")

	// Timeout
	timeout = flag.Duration("timeout", 60*time.Second, "Overall timeout for all requests")
)

func main() {
	flag.Parse()

	// Setup context with timeout and cancellation
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Handle Ctrl+C for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Create rate-limited client
	client, err := realclient.NewClient(*serverAddr, *rateLimitKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	log.Printf("=== Rate-Limited HTTP Client Demo ===")
	log.Printf("Mode: %s", *mode)
	log.Printf("Server: %s", *serverAddr)
	log.Printf("Key: %s", *rateLimitKey)
	log.Printf("Requests: %d", *count)
	if *mode != "sequential" {
		log.Printf("Workers: %d", *workers)
	}
	log.Println()

	// Run selected mode
	startTime := time.Now()
	var successCount, errorCount int

	switch *mode {
	case "sequential":
		successCount, errorCount = runSequential(ctx, client)
	case "worker":
		successCount, errorCount = runWorkerPool(ctx, client)
	case "pipeline":
		successCount, errorCount = runPipeline(ctx, client)
	default:
		log.Fatalf("Unknown mode: %s (valid: sequential, worker, pipeline)", *mode)
	}

	duration := time.Since(startTime)

	// Print final report
	metrics := client.Metrics()
	printReport(metrics, successCount, errorCount, duration)
}

// runSequential executes requests one at a time.
func runSequential(ctx context.Context, client *realclient.Client) (successCount, errorCount int) {
	log.Println("Running in SEQUENTIAL mode...")

	for i := 0; i < *count; i++ {
		select {
		case <-ctx.Done():
			log.Println("Context canceled, stopping...")
			return
		default:
		}

		target := selectTarget()
		log.Printf("[%d/%d] Fetching %s...", i+1, *count, target.Name)

		resp, err := client.FetchWithContext(ctx, target)
		if err != nil {
			log.Printf("  ✗ Error: %v", err)
			errorCount++
			continue
		}

		resp.Body.Close()
		log.Printf("  ✓ Success: %d %s", resp.StatusCode, target.Description)
		successCount++
	}

	return
}

// runWorkerPool executes requests using a worker pool.
func runWorkerPool(ctx context.Context, client *realclient.Client) (successCount, errorCount int) {
	log.Printf("Running in WORKER POOL mode with %d workers...\n", *workers)

	pool := patterns.NewWorkerPool(client, *workers)
	defer pool.Close()

	// Submit jobs
	log.Printf("Submitting %d jobs...", *count)
	for i := 0; i < *count; i++ {
		target := selectTarget()
		pool.Submit(target)
	}
	pool.CloseJobs()

	// Collect results
	log.Println("Collecting results...")
	resultsReceived := 0
	for result := range pool.Results() {
		resultsReceived++

		if result.Error != nil {
			log.Printf("[%d/%d] ✗ %s: %v",
				resultsReceived, *count, result.Target.Name, result.Error)
			errorCount++
		} else {
			log.Printf("[%d/%d] ✓ %s: %d (took %dms)",
				resultsReceived, *count, result.Target.Name,
				result.Response.StatusCode,
				result.DurationNanos/1_000_000)
			result.Response.Body.Close()
			successCount++
		}
	}

	// Print pool stats
	stats := pool.Stats()
	log.Printf("\nPool Stats: %s", stats.String())

	return
}

// runPipeline executes requests using a multi-stage pipeline.
func runPipeline(ctx context.Context, client *realclient.Client) (successCount, errorCount int) {
	log.Printf("Running in PIPELINE mode with %d workers per stage...\n", *workers)

	pipeline := patterns.NewPipeline(client, *workers)

	// Generate targets
	targetsToFetch := make([]*targets.Target, *count)
	for i := 0; i < *count; i++ {
		targetsToFetch[i] = selectTarget()
	}

	log.Printf("Processing %d targets through pipeline...", len(targetsToFetch))
	results := pipeline.Run(ctx, targetsToFetch)

	// Collect results
	log.Println("Collecting results...")
	resultsReceived := 0
	for result := range results {
		resultsReceived++

		if result.Error != nil {
			log.Printf("[%d/%d] ✗ %s: %v",
				resultsReceived, *count, result.Target.Name, result.Error)
			errorCount++
		} else {
			log.Printf("[%d/%d] ✓ %s: %d",
				resultsReceived, *count, result.Target.Name, result.Response.StatusCode)
			result.Response.Body.Close()
			successCount++
		}
	}

	return
}

// selectTarget returns the target to fetch based on CLI flags.
func selectTarget() *targets.Target {
	if *targetName != "" {
		target := targets.GetTarget(*targetName)
		if target == nil {
			log.Fatalf("Unknown target: %s (available: %v)", *targetName, targets.Names())
		}
		return target
	}
	return targets.Random()
}

// printReport prints a final summary report.
func printReport(metrics realclient.Metrics, successCount, errorCount int, duration time.Duration) {
	fmt.Println()
	fmt.Println("=== Final Report ===")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Throughput: %.2f req/s\n", float64(*count)/duration.Seconds())
	fmt.Println()
	fmt.Println("HTTP Requests:")
	fmt.Printf("  Success: %d\n", successCount)
	fmt.Printf("  Errors:  %d\n", errorCount)
	fmt.Printf("  Total:   %d\n", successCount+errorCount)
	fmt.Println()
	fmt.Println("Rate Limiter:")
	fmt.Printf("  Allowed:  %d\n", metrics.Allowed)
	fmt.Printf("  Rejected: %d (retries: %d)\n", metrics.Rejected, metrics.TotalRetries)
	fmt.Printf("  Failed:   %d\n", metrics.Failed)
	fmt.Println()
	if metrics.Rejected > 0 {
		fmt.Printf("Rejection Rate: %.2f%%\n", 100.0*float64(metrics.Rejected)/float64(metrics.Allowed+metrics.Rejected))
	}
}
