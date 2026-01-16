// Package grpcserver provides a gRPC service implementation for rate limiting.
//
// This package is a thin wrapper around the engine package, providing:
//   - gRPC service interface implementation
//   - Input validation (key, permits)
//   - Error handling with appropriate gRPC status codes
//   - Health check endpoint
//
// The server is stateless - all rate limiting state lives in the injected engine.
// Multiple server instances can share the same engine (though typically each
// server process has its own engine instance).
//
// # Architecture
//
// The gRPC layer is deliberately thin:
//
//	Client → gRPC Server (validation) → Engine (caching, locking) → Algorithm (rate limiting)
//
// Responsibilities:
//   - Input validation: Ensure key is non-empty, permits > 0
//   - Protocol conversion: Protobuf ↔ Go types
//   - Error mapping: Go errors → gRPC status codes
//   - Service lifecycle: Server startup, shutdown, health checks
//
// NOT responsible for:
//   - Rate limiting logic (lives in algorithm package)
//   - Caching or concurrency (lives in engine package)
//   - Time abstraction (lives in clock package)
//
// # Thread-Safety
//
// The Server struct is safe for concurrent use by multiple goroutines.
// gRPC automatically handles concurrent requests - each request gets its own
// goroutine and calls CheckRateLimit concurrently.
//
// Thread-safety is delegated to the engine layer:
//   - Engine handles concurrent access to the cache
//   - Engine provides per-key locking for rate limiters
//   - No synchronization needed at the gRPC layer
//
// # Example Usage
//
//	// Create engine
//	clk := clock.NewSystemClock()
//	config := engine.DefaultTokenBucketConfig()
//	eng, _ := engine.NewEngine(clk, config, 10000)
//
//	// Create gRPC server
//	srv := grpcserver.New(eng)
//
//	// Start gRPC listener
//	lis, _ := net.Listen("tcp", ":50051")
//	grpcServer := grpc.NewServer()
//	pb.RegisterRateLimitServiceServer(grpcServer, srv)
//	grpcServer.Serve(lis)
//
// # Error Handling
//
// The server maps validation errors to appropriate gRPC status codes:
//
//	Empty key          → INVALID_ARGUMENT
//	Invalid permits    → INVALID_ARGUMENT
//	Algorithm error    → INVALID_ARGUMENT (e.g., permits exceed capacity)
//	Internal errors    → INTERNAL (should never happen in practice)
//
// Successful rate limit checks always return OK status, even if rejected.
// The `allowed` field in the response indicates whether to proceed.
//
// # Protobuf Contract
//
// This server implements the contract defined in proto/ratelimit.proto:
//
//	service RateLimitService {
//	  rpc CheckRateLimit(CheckRateLimitRequest) returns (CheckRateLimitResponse);
//	  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
//	}
//
// The same .proto file is used by both Go and Java implementations,
// ensuring cross-language compatibility.
package grpcserver

import (
	"context"

	pb "github.com/ratelimiter/go/api/ratelimit/v1"
	"github.com/ratelimiter/go/pkg/engine"
	"github.com/ratelimiter/go/pkg/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the RateLimitService gRPC interface.
//
// This is a thin wrapper around the engine package. The server is stateless
// and delegates all rate limiting logic to the injected engine.
//
// Thread-safe: Safe for concurrent use by multiple goroutines.
// gRPC automatically handles concurrent requests.
type Server struct {
	pb.UnimplementedRateLimitServiceServer
	engine *engine.Engine
}

// New creates a new gRPC server wrapping the provided engine.
//
// Parameters:
//   - engine: The rate limiting engine (must not be nil)
//
// Returns:
//   - *Server: Initialized server ready to handle gRPC requests
//
// The engine configuration (algorithm, capacity, limits, etc.) is already
// set when the engine is created. This server simply wraps it with a gRPC interface.
//
// Example:
//
//	clk := clock.NewSystemClock()
//	config := engine.Config{
//	    Algorithm:  engine.AlgorithmTokenBucket,
//	    Capacity:   100,
//	    RefillRate: 10.0,
//	}
//	eng, _ := engine.NewEngine(clk, config, 10000)
//	srv := grpcserver.New(eng)
func New(engine *engine.Engine) *Server {
	return &Server{
		engine: engine,
	}
}

// CheckRateLimit handles rate limit check requests.
//
// This method:
//  1. Validates the request (key not empty, permits > 0)
//  2. Calls the engine to check the rate limit
//  3. Converts the result to a protobuf response
//  4. Maps any errors to appropriate gRPC status codes
//
// Parameters:
//   - ctx: Request context (currently unused, reserved for future use like tracing)
//   - req: The rate limit check request
//
// Returns:
//   - *pb.CheckRateLimitResponse: Response indicating allowed/rejected
//   - error: gRPC status error if validation fails
//
// Error Handling:
//   - Empty key: Returns INVALID_ARGUMENT status
//   - Invalid permits (≤ 0): Returns INVALID_ARGUMENT status
//   - Algorithm error: Returns INVALID_ARGUMENT status
//
// Thread-safety:
//   - Safe for concurrent use (gRPC handles concurrency)
//   - Different keys execute in parallel (engine has per-key locking)
//   - Same key serializes through engine's per-key mutex
//
// Example request/response:
//
//	// Allowed request
//	Request:  {key: "user:123", permits: 5}
//	Response: {allowed: true, retry_after_nanos: 0}
//
//	// Rejected request
//	Request:  {key: "user:456", permits: 10}
//	Response: {allowed: false, retry_after_nanos: 2000000000} // 2 seconds
//
//	// Invalid request
//	Request:  {key: "", permits: 1}
//	Error:    INVALID_ARGUMENT: key must not be empty
func (s *Server) CheckRateLimit(ctx context.Context, req *pb.CheckRateLimitRequest) (*pb.CheckRateLimitResponse, error) {
	// Validation: key must not be empty
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key must not be empty")
	}

	// Validation: permits must be positive
	if req.GetPermits() <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "permits must be > 0, got: %d", req.GetPermits())
	}

	// Call engine to check rate limit
	result, err := s.engine.TryAcquire(req.GetKey(), int(req.GetPermits()))
	if err != nil {
		// Engine returns errors for validation failures (e.g., permits exceed capacity)
		return nil, status.Errorf(codes.InvalidArgument, "rate limit check failed: %v", err)
	}

	// Convert result to protobuf response
	response := &pb.CheckRateLimitResponse{
		Allowed:         result.Decision == model.DecisionAllow,
		RetryAfterNanos: result.RetryAfterNanos,
	}

	return response, nil
}

// HealthCheck handles health check requests.
//
// This endpoint allows monitoring systems to verify the server is healthy
// and accepting requests. Currently returns a simple SERVING status.
//
// Parameters:
//   - ctx: Request context (unused)
//   - req: Health check request (unused, reserved for future extensions)
//
// Returns:
//   - *pb.HealthCheckResponse: Response with SERVING status
//   - error: Always nil (health checks don't fail)
//
// Future enhancements could check:
//   - Engine cache size (warn if near capacity)
//   - Algorithm health (if backends are involved)
//   - System resources (memory, CPU)
//
// Example:
//
//	Request:  {}
//	Response: {status: SERVING}
func (s *Server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status: pb.HealthCheckResponse_SERVING,
	}, nil
}
