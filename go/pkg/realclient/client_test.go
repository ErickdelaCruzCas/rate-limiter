package realclient

import (
	"context"
	"testing"
	"time"

	"github.com/ratelimiter/go/pkg/targets"
)

// TestClient_Fetch_Success tests successful rate-limited HTTP request.
func TestClient_Fetch_Success(t *testing.T) {
	// Skip if server not running (integration test)
	client, err := NewClient("localhost:50051", "test:client:success")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	// Fetch from a fast target
	target := targets.JSONPlaceholderPosts
	resp, err := client.Fetch(target)
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify HTTP response
	if resp.StatusCode != target.ExpectedStatus {
		t.Errorf("expected status %d, got %d", target.ExpectedStatus, resp.StatusCode)
	}

	// Verify metrics
	metrics := client.Metrics()
	if metrics.Allowed == 0 {
		t.Error("expected at least one allowed request")
	}
	if metrics.TotalHTTPRequests == 0 {
		t.Error("expected at least one HTTP request")
	}
}

// TestClient_Fetch_WithContext tests context cancellation.
func TestClient_Fetch_WithContext(t *testing.T) {
	client, err := NewClient("localhost:50051", "test:client:context")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Use delayed target to ensure timeout
	target := targets.HTTPBinDelay
	_, err = client.FetchWithContext(ctx, target)

	// Should fail due to context timeout
	if err == nil {
		t.Error("expected context timeout error, got nil")
	}
}

// TestClient_Metrics tests metrics tracking.
func TestClient_Metrics(t *testing.T) {
	client, err := NewClient("localhost:50051", "test:client:metrics")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	// Reset metrics to start fresh
	client.ResetMetrics()

	// Make several requests
	target := targets.HTTPBinGet
	for i := 0; i < 5; i++ {
		resp, err := client.Fetch(target)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}

	// Verify metrics were updated
	metrics := client.Metrics()
	total := metrics.Allowed + metrics.Rejected + metrics.Failed
	if total != 5 {
		t.Errorf("expected 5 total requests, got %d (allowed=%d, rejected=%d, failed=%d)",
			total, metrics.Allowed, metrics.Rejected, metrics.Failed)
	}

	// Test String() method
	metricsStr := metrics.String()
	if metricsStr == "" {
		t.Error("Metrics.String() returned empty string")
	}
}

// TestClient_ResetMetrics tests metrics reset.
func TestClient_ResetMetrics(t *testing.T) {
	client, err := NewClient("localhost:50051", "test:client:reset")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	// Make a request to populate metrics
	target := targets.UUIDGenerator
	resp, err := client.Fetch(target)
	if err == nil && resp != nil {
		resp.Body.Close()
	}

	// Verify metrics are non-zero
	metrics := client.Metrics()
	if metrics.Allowed == 0 && metrics.Rejected == 0 && metrics.Failed == 0 {
		t.Skip("No metrics recorded, skipping test")
	}

	// Reset and verify
	client.ResetMetrics()
	metricsAfter := client.Metrics()
	if metricsAfter.Allowed != 0 || metricsAfter.Rejected != 0 || metricsAfter.Failed != 0 {
		t.Errorf("expected all metrics to be zero after reset, got %+v", metricsAfter)
	}
}

// TestClient_Close tests closing the client.
func TestClient_Close(t *testing.T) {
	client, err := NewClient("localhost:50051", "test:client:close")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}

	// Close should not error
	if err := client.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Second close may return error (connection already closing) - this is acceptable
	// We just want to ensure it doesn't panic
	_ = client.Close()
}

// TestClient_MultipleTargets tests fetching from different targets.
func TestClient_MultipleTargets(t *testing.T) {
	client, err := NewClient("localhost:50051", "test:client:multitarget")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	allTargets := []*targets.Target{
		targets.JSONPlaceholderPosts,
		targets.HTTPBinGet,
		targets.UUIDGenerator,
	}

	for _, target := range allTargets {
		resp, err := client.Fetch(target)
		if err != nil {
			t.Logf("Fetch(%s) failed: %v", target.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != target.ExpectedStatus {
			t.Errorf("%s: expected status %d, got %d",
				target.Name, target.ExpectedStatus, resp.StatusCode)
		}
	}
}
