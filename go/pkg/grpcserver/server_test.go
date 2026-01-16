package grpcserver_test

import (
	"context"
	"testing"

	pb "github.com/ratelimiter/go/api/ratelimit/v1"
	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/engine"
	"github.com/ratelimiter/go/pkg/grpcserver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestCheckRateLimit_Allow verifies ALLOW decision is returned correctly.
func TestCheckRateLimit_Allow(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   10,
		RefillRate: 1.0,
	}
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 5,
	}

	resp, err := srv.CheckRateLimit(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetAllowed() {
		t.Error("expected request to be allowed")
	}
	if resp.GetRetryAfterNanos() != 0 {
		t.Errorf("expected retry_after_nanos=0 for allowed request, got %d", resp.GetRetryAfterNanos())
	}
}

// TestCheckRateLimit_Reject verifies REJECT decision is returned correctly.
func TestCheckRateLimit_Reject(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   5,
		RefillRate: 1.0,
	}
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	// Exhaust tokens
	for i := 0; i < 5; i++ {
		srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
			Key:     "user:123",
			Permits: 1,
		})
	}

	// Next request should be rejected
	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	}

	resp, err := srv.CheckRateLimit(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetAllowed() {
		t.Error("expected request to be rejected")
	}
	if resp.GetRetryAfterNanos() <= 0 {
		t.Error("expected positive retry_after_nanos for rejected request")
	}
}

// TestCheckRateLimit_EmptyKey verifies empty key is rejected.
func TestCheckRateLimit_EmptyKey(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	req := &pb.CheckRateLimitRequest{
		Key:     "",
		Permits: 1,
	}

	resp, err := srv.CheckRateLimit(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for empty key")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", st.Code())
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
}

// TestCheckRateLimit_ZeroPermits verifies zero permits is rejected.
func TestCheckRateLimit_ZeroPermits(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 0,
	}

	_, err := srv.CheckRateLimit(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for zero permits")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", st.Code())
	}
}

// TestCheckRateLimit_NegativePermits verifies negative permits is rejected.
func TestCheckRateLimit_NegativePermits(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: -5,
	}

	_, err := srv.CheckRateLimit(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for negative permits")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", st.Code())
	}
}

// TestCheckRateLimit_MultipleKeys verifies independent rate limiting.
func TestCheckRateLimit_MultipleKeys(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   5,
		RefillRate: 1.0,
	}
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	// Exhaust user:123's tokens
	for i := 0; i < 5; i++ {
		srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
			Key:     "user:123",
			Permits: 1,
		})
	}

	// user:123 should be rejected
	resp1, _ := srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	})
	if resp1.GetAllowed() {
		t.Error("user:123 should be rejected")
	}

	// user:456 should be allowed (different limiter)
	resp2, _ := srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
		Key:     "user:456",
		Permits: 1,
	})
	if !resp2.GetAllowed() {
		t.Error("user:456 should be allowed")
	}
}

// TestCheckRateLimit_RetryAfterAccuracy verifies retry-after calculation.
func TestCheckRateLimit_RetryAfterAccuracy(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   10,
		RefillRate: 1.0, // 1 token per second
	}
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
			Key:     "user:123",
			Permits: 1,
		})
	}

	// Try to acquire 5 tokens (need 5 seconds)
	resp, _ := srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 5,
	})

	if resp.GetAllowed() {
		t.Fatal("expected request to be rejected")
	}

	// Retry-after should be approximately 5 seconds (with some margin for floating point)
	expected := int64(5_000_000_000) // 5 seconds in nanos
	actual := resp.GetRetryAfterNanos()

	// Allow 100ms margin
	margin := int64(100_000_000)
	if actual < expected-margin || actual > expected+margin {
		t.Errorf("expected retry_after ~%dns, got %dns", expected, actual)
	}
}

// TestCheckRateLimit_DifferentAlgorithms verifies all algorithms work.
func TestCheckRateLimit_DifferentAlgorithms(t *testing.T) {
	clk := clock.NewManualClock(0)

	tests := []struct {
		name   string
		config engine.Config
	}{
		{
			name:   "TokenBucket",
			config: engine.DefaultTokenBucketConfig(),
		},
		{
			name:   "FixedWindow",
			config: engine.DefaultFixedWindowConfig(),
		},
		{
			name:   "SlidingWindowLog",
			config: engine.DefaultSlidingWindowLogConfig(),
		},
		{
			name:   "SlidingWindowCounter",
			config: engine.DefaultSlidingWindowCounterConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng, _ := engine.NewEngine(clk, tt.config, 100)
			srv := grpcserver.New(eng)

			req := &pb.CheckRateLimitRequest{
				Key:     "user:123",
				Permits: 1,
			}

			resp, err := srv.CheckRateLimit(context.Background(), req)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !resp.GetAllowed() {
				t.Error("first request should be allowed")
			}
		})
	}
}

// TestHealthCheck verifies health check endpoint.
func TestHealthCheck(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	req := &pb.HealthCheckRequest{}

	resp, err := srv.HealthCheck(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetStatus() != pb.HealthCheckResponse_SERVING {
		t.Errorf("expected SERVING status, got %v", resp.GetStatus())
	}
}

// TestHealthCheck_AlwaysSucceeds verifies health checks never fail.
func TestHealthCheck_AlwaysSucceeds(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	// Call health check multiple times
	for i := 0; i < 10; i++ {
		resp, err := srv.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if resp.GetStatus() != pb.HealthCheckResponse_SERVING {
			t.Errorf("iteration %d: expected SERVING, got %v", i, resp.GetStatus())
		}
	}
}

// TestServer_DecisionMapping verifies correct mapping between model and protobuf.
func TestServer_DecisionMapping(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   1,
		RefillRate: 1.0,
	}
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	// First request: ALLOW
	resp1, _ := srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	})

	if !resp1.GetAllowed() {
		t.Error("expected first request to be allowed")
	}
	if resp1.GetRetryAfterNanos() != 0 {
		t.Error("expected retry_after=0 for allowed request")
	}

	// Second request: REJECT
	resp2, _ := srv.CheckRateLimit(context.Background(), &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	})

	if resp2.GetAllowed() {
		t.Error("expected second request to be rejected")
	}
	if resp2.GetRetryAfterNanos() <= 0 {
		t.Error("expected positive retry_after for rejected request")
	}
}

// TestServer_ContextPropagation verifies context is accepted (even if unused).
func TestServer_ContextPropagation(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()
	eng, _ := engine.NewEngine(clk, config, 100)
	srv := grpcserver.New(eng)

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	}

	_, err := srv.CheckRateLimit(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// BenchmarkCheckRateLimit_Allow measures throughput for allowed requests.
func BenchmarkCheckRateLimit_Allow(b *testing.B) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   1_000_000,
		RefillRate: 1_000_000.0,
	}
	eng, _ := engine.NewEngine(clk, config, 10000)
	srv := grpcserver.New(eng)

	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.CheckRateLimit(context.Background(), req)
	}
}

// BenchmarkCheckRateLimit_Reject measures throughput for rejected requests.
func BenchmarkCheckRateLimit_Reject(b *testing.B) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   0, // No tokens available
		RefillRate: 0,
	}
	eng, _ := engine.NewEngine(clk, config, 10000)
	srv := grpcserver.New(eng)

	req := &pb.CheckRateLimitRequest{
		Key:     "user:123",
		Permits: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.CheckRateLimit(context.Background(), req)
	}
}
