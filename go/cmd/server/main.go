// Package main implements a production-ready gRPC rate limiter server.
//
// This server provides a complete, runnable rate limiting service with:
//   - CLI configuration (algorithm, capacity, limits, port, etc.)
//   - Graceful shutdown (SIGINT/SIGTERM handling)
//   - Health check endpoint
//   - Structured logging
//   - Multiple algorithm support
//
// # Usage
//
// Run with Token Bucket (default):
//
//	go run main.go \
//	  --port=50051 \
//	  --algorithm=token_bucket \
//	  --capacity=100 \
//	  --refill_rate=10.0 \
//	  --max_keys=10000
//
// Run with Fixed Window:
//
//	go run main.go \
//	  --algorithm=fixed_window \
//	  --window_seconds=60 \
//	  --limit=1000
//
// Run with Sliding Window Log:
//
//	go run main.go \
//	  --algorithm=sliding_window_log \
//	  --window_seconds=10 \
//	  --limit=100
//
// Run with Sliding Window Counter:
//
//	go run main.go \
//	  --algorithm=sliding_window_counter \
//	  --window_seconds=60 \
//	  --bucket_seconds=1 \
//	  --limit=1000
//
// # Bazel Usage
//
//	bazel run //go/cmd/server -- --port=50051 --algorithm=token_bucket
//
// # Graceful Shutdown
//
// The server handles SIGINT (Ctrl+C) and SIGTERM gracefully:
//  1. Stop accepting new connections
//  2. Wait for in-flight requests to complete (up to 30 seconds)
//  3. Shut down cleanly
//
// # Algorithm Configuration
//
// Different algorithms require different flags:
//
// Token Bucket:
//   - --capacity: Maximum tokens (required)
//   - --refill_rate: Tokens per second (required)
//
// Fixed Window:
//   - --window_seconds: Window duration (required)
//   - --limit: Max permits per window (required)
//
// Sliding Window Log:
//   - --window_seconds: Window duration (required)
//   - --limit: Max events in window (required)
//
// Sliding Window Counter:
//   - --window_seconds: Total window duration (required)
//   - --bucket_seconds: Bucket duration (required)
//   - --limit: Max permits in window (required)
//
// # Health Check
//
// The server automatically provides a health check endpoint:
//
//	grpcurl -plaintext localhost:50051 \
//	  rl.proto.RateLimitService/HealthCheck
//
// # Cross-Language Compatibility
//
// This Go server shares the same Protobuf contract as the Java implementation,
// allowing clients written in any language to interact with either server.
//
// Test with Java client:
//
//	bazel run //java/grpc:client -- localhost:50051 user:123 5
//
// Test with Go client:
//
//	bazel run //go/cmd/client -- --server=localhost:50051 --key=user:123
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/ratelimiter/go/api/ratelimit/v1"
	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/engine"
	"github.com/ratelimiter/go/pkg/grpcserver"
	"google.golang.org/grpc"
)

// Configuration flags
var (
	// Server configuration
	port    = flag.Int("port", 50051, "gRPC server port")
	maxKeys = flag.Int("max_keys", 10000, "Maximum number of keys to cache")

	// Algorithm selection
	algorithm = flag.String("algorithm", "token_bucket",
		"Rate limiting algorithm: token_bucket, fixed_window, sliding_window_log, sliding_window_counter")

	// Token Bucket parameters
	capacity   = flag.Int64("capacity", 100, "Token Bucket: maximum tokens")
	refillRate = flag.Float64("refill_rate", 10.0, "Token Bucket: tokens per second")

	// Window-based parameters
	windowSeconds = flag.Float64("window_seconds", 60.0, "Window duration in seconds")
	bucketSeconds = flag.Float64("bucket_seconds", 1.0, "Sliding Window Counter: bucket duration in seconds")
	limit         = flag.Int("limit", 100, "Maximum permits per window")
)

func main() {
	flag.Parse()

	// Create engine configuration
	config, err := createConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create engine
	clk := clock.NewSystemClock()
	eng, err := engine.NewEngine(clk, config, *maxKeys)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	log.Printf("Engine initialized: algorithm=%s, maxKeys=%d", config.Algorithm, *maxKeys)
	logAlgorithmConfig(config)

	// Create gRPC server
	grpcSrv := grpc.NewServer()
	rateLimitSrv := grpcserver.New(eng)
	pb.RegisterRateLimitServiceServer(grpcSrv, rateLimitSrv)

	// Start listener
	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	log.Printf("gRPC server listening on %s", addr)
	log.Printf("Ready to accept rate limit requests")
	log.Printf("Press Ctrl+C to shutdown gracefully")

	// Graceful shutdown handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Printf("Received shutdown signal, initiating graceful shutdown...")
		log.Printf("Stopping acceptance of new connections...")

		// Stop accepting new connections
		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()

		// Wait up to 30 seconds for graceful shutdown
		select {
		case <-stopped:
			log.Printf("Server shut down gracefully")
		case <-time.After(30 * time.Second):
			log.Printf("Graceful shutdown timeout, forcing stop")
			grpcSrv.Stop()
		}

		os.Exit(0)
	}()

	// Start serving (blocks until shutdown)
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// createConfig creates an engine configuration from command-line flags.
func createConfig() (engine.Config, error) {
	switch *algorithm {
	case "token_bucket":
		return createTokenBucketConfig()

	case "fixed_window":
		return createFixedWindowConfig()

	case "sliding_window_log":
		return createSlidingWindowLogConfig()

	case "sliding_window_counter":
		return createSlidingWindowCounterConfig()

	default:
		return engine.Config{}, fmt.Errorf("unknown algorithm: %s (must be one of: token_bucket, fixed_window, sliding_window_log, sliding_window_counter)", *algorithm)
	}
}

// createTokenBucketConfig creates a Token Bucket configuration.
func createTokenBucketConfig() (engine.Config, error) {
	if *capacity <= 0 {
		return engine.Config{}, fmt.Errorf("capacity must be > 0, got: %d", *capacity)
	}
	if *refillRate <= 0 {
		return engine.Config{}, fmt.Errorf("refill_rate must be > 0, got: %f", *refillRate)
	}

	return engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   *capacity,
		RefillRate: *refillRate,
	}, nil
}

// createFixedWindowConfig creates a Fixed Window configuration.
func createFixedWindowConfig() (engine.Config, error) {
	if *windowSeconds <= 0 {
		return engine.Config{}, fmt.Errorf("window_seconds must be > 0, got: %f", *windowSeconds)
	}
	if *limit <= 0 {
		return engine.Config{}, fmt.Errorf("limit must be > 0, got: %d", *limit)
	}

	windowNanos := int64(*windowSeconds * 1e9)

	return engine.Config{
		Algorithm:   engine.AlgorithmFixedWindow,
		WindowNanos: windowNanos,
		Limit:       *limit,
	}, nil
}

// createSlidingWindowLogConfig creates a Sliding Window Log configuration.
func createSlidingWindowLogConfig() (engine.Config, error) {
	if *windowSeconds <= 0 {
		return engine.Config{}, fmt.Errorf("window_seconds must be > 0, got: %f", *windowSeconds)
	}
	if *limit <= 0 {
		return engine.Config{}, fmt.Errorf("limit must be > 0, got: %d", *limit)
	}

	windowNanos := int64(*windowSeconds * 1e9)

	return engine.Config{
		Algorithm:   engine.AlgorithmSlidingWindowLog,
		WindowNanos: windowNanos,
		Limit:       *limit,
	}, nil
}

// createSlidingWindowCounterConfig creates a Sliding Window Counter configuration.
func createSlidingWindowCounterConfig() (engine.Config, error) {
	if *windowSeconds <= 0 {
		return engine.Config{}, fmt.Errorf("window_seconds must be > 0, got: %f", *windowSeconds)
	}
	if *bucketSeconds <= 0 {
		return engine.Config{}, fmt.Errorf("bucket_seconds must be > 0, got: %f", *bucketSeconds)
	}
	if *limit <= 0 {
		return engine.Config{}, fmt.Errorf("limit must be > 0, got: %d", *limit)
	}

	windowNanos := int64(*windowSeconds * 1e9)
	bucketNanos := int64(*bucketSeconds * 1e9)

	// Validate divisibility
	if windowNanos%bucketNanos != 0 {
		return engine.Config{}, fmt.Errorf("window_seconds must be divisible by bucket_seconds (window=%fs, bucket=%fs)",
			*windowSeconds, *bucketSeconds)
	}

	return engine.Config{
		Algorithm:   engine.AlgorithmSlidingWindowCounter,
		WindowNanos: windowNanos,
		BucketNanos: bucketNanos,
		Limit:       *limit,
	}, nil
}

// logAlgorithmConfig logs the algorithm-specific configuration for debugging.
func logAlgorithmConfig(config engine.Config) {
	switch config.Algorithm {
	case engine.AlgorithmTokenBucket:
		log.Printf("  Token Bucket: capacity=%d, refillRate=%.2f tokens/sec",
			config.Capacity, config.RefillRate)

	case engine.AlgorithmFixedWindow:
		windowSec := float64(config.WindowNanos) / 1e9
		log.Printf("  Fixed Window: window=%.2fs, limit=%d/window",
			windowSec, config.Limit)

	case engine.AlgorithmSlidingWindowLog:
		windowSec := float64(config.WindowNanos) / 1e9
		log.Printf("  Sliding Window Log: window=%.2fs, limit=%d/window",
			windowSec, config.Limit)

	case engine.AlgorithmSlidingWindowCounter:
		windowSec := float64(config.WindowNanos) / 1e9
		bucketSec := float64(config.BucketNanos) / 1e9
		numBuckets := config.WindowNanos / config.BucketNanos
		log.Printf("  Sliding Window Counter: window=%.2fs, bucket=%.2fs, numBuckets=%d, limit=%d/window",
			windowSec, bucketSec, numBuckets, config.Limit)
	}
}
