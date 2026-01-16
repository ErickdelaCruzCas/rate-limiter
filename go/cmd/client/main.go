// Package main implements an example gRPC client for the rate limiter service.
//
// This client demonstrates how to:
//   - Connect to the rate limiter gRPC server
//   - Send rate limit check requests
//   - Handle ALLOW/REJECT responses
//   - Interpret retry-after values
//   - Perform health checks
//
// # Basic Usage
//
// Check a single rate limit:
//
//	go run main.go --server=localhost:50051 --key=user:123 --permits=5
//
// Run multiple requests to demonstrate rate limiting:
//
//	go run main.go --server=localhost:50051 --key=user:123 --permits=1 --count=20
//
// Health check only:
//
//	go run main.go --server=localhost:50051 --health_check
//
// # Bazel Usage
//
//	bazel run //go/cmd/client -- --server=localhost:50051 --key=user:123
//
// # Example Output
//
// Successful request:
//
//	Request #1: ALLOWED (key=user:123, permits=5)
//	Remaining capacity: sufficient
//
// Rejected request:
//
//	Request #11: REJECTED (retry after 2.5 seconds)
//	Rate limit exceeded - please slow down
//
// # Cross-Language Testing
//
// This client can connect to either the Go or Java server:
//
// Test with Go server:
//
//	# Terminal 1: Start Go server
//	bazel run //go/cmd/server
//
//	# Terminal 2: Run client
//	bazel run //go/cmd/client
//
// Test with Java server:
//
//	# Terminal 1: Start Java server
//	bazel run //java/grpc:server -- 50051
//
//	# Terminal 2: Run client (same command)
//	bazel run //go/cmd/client
//
// Both servers implement the same Protobuf contract, so the client works
// identically with either implementation.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	pb "github.com/ratelimiter/go/api/ratelimit/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	// Server connection
	serverAddr = flag.String("server", "localhost:50051", "gRPC server address")
	timeout    = flag.Duration("timeout", 10*time.Second, "Request timeout")

	// Request parameters
	key     = flag.String("key", "user:123", "Rate limit key")
	permits = flag.Int("permits", 1, "Number of permits to acquire")
	count   = flag.Int("count", 1, "Number of requests to send")

	// Actions
	healthCheck = flag.Bool("health_check", false, "Perform health check only")
)

func main() {
	flag.Parse()

	// Connect to server
	log.Printf("Connecting to rate limiter server at %s...", *serverAddr)
	conn, err := grpc.Dial(
		*serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRateLimitServiceClient(conn)
	log.Printf("Connected successfully\n")

	// Perform health check if requested
	if *healthCheck {
		performHealthCheck(client)
		return
	}

	// Send rate limit requests
	log.Printf("Sending %d rate limit request(s)...\n", *count)
	log.Printf("Configuration: key=%s, permits=%d\n", *key, *permits)
	log.Println(strings.Repeat("-", 60))

	allowed := 0
	rejected := 0

	for i := 1; i <= *count; i++ {
		result := sendRateLimitRequest(client, i)
		if result {
			allowed++
		} else {
			rejected++
		}

		// Small delay between requests for readability
		if i < *count {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Summary
	log.Println(strings.Repeat("-", 60))
	log.Printf("Summary: %d allowed, %d rejected (out of %d total)", allowed, rejected, *count)
}

// sendRateLimitRequest sends a single rate limit check request.
// Returns true if allowed, false if rejected.
func sendRateLimitRequest(client pb.RateLimitServiceClient, requestNum int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	req := &pb.CheckRateLimitRequest{
		Key:     *key,
		Permits: int32(*permits),
	}

	resp, err := client.CheckRateLimit(ctx, req)
	if err != nil {
		// Check if it's a gRPC error with details
		if st, ok := status.FromError(err); ok {
			log.Printf("Request #%d: ERROR - %s (code: %v)", requestNum, st.Message(), st.Code())
		} else {
			log.Printf("Request #%d: ERROR - %v", requestNum, err)
		}
		return false
	}

	if resp.GetAllowed() {
		log.Printf("Request #%d: ✓ ALLOWED", requestNum)
		return true
	} else {
		retryAfterNanos := resp.GetRetryAfterNanos()
		retryAfterSec := float64(retryAfterNanos) / 1e9
		log.Printf("Request #%d: ✗ REJECTED (retry after %.2f seconds)", requestNum, retryAfterSec)
		return false
	}
}

// performHealthCheck sends a health check request and displays the result.
func performHealthCheck(client pb.RateLimitServiceClient) {
	log.Println("Performing health check...")

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	req := &pb.HealthCheckRequest{}

	resp, err := client.HealthCheck(ctx, req)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}

	statusStr := resp.GetStatus().String()
	log.Printf("Health check result: %s", statusStr)

	if resp.GetStatus() == pb.HealthCheckResponse_SERVING {
		log.Println("Server is healthy and accepting requests")
	} else {
		log.Println("Server is not serving (status: %s)", statusStr)
	}
}

// strings provides string utilities (temporary until we import "strings")
var strings = struct {
	Repeat func(s string, count int) string
}{
	Repeat: func(s string, count int) string {
		result := ""
		for i := 0; i < count; i++ {
			result += s
		}
		return result
	},
}
