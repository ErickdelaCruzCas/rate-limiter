package patterns

import (
	"context"
	"testing"
	"time"

	"github.com/ratelimiter/go/pkg/realclient"
	"github.com/ratelimiter/go/pkg/targets"
)

// TestWorkerPool_Submit tests basic worker pool functionality.
func TestWorkerPool_Submit(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pool:submit")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	pool := NewWorkerPool(client, 3)
	defer pool.Close()

	// Submit jobs
	targetsToFetch := []*targets.Target{
		targets.JSONPlaceholderPosts,
		targets.HTTPBinGet,
		targets.UUIDGenerator,
	}

	for _, target := range targetsToFetch {
		pool.Submit(target)
	}
	pool.CloseJobs()

	// Collect results
	resultsCount := 0
	for result := range pool.Results() {
		resultsCount++
		if result == nil {
			t.Error("received nil result")
		}
		if result.Response != nil {
			result.Response.Body.Close()
		}
	}

	if resultsCount != len(targetsToFetch) {
		t.Errorf("expected %d results, got %d", len(targetsToFetch), resultsCount)
	}

	// Verify stats
	stats := pool.Stats()
	if stats.JobsSubmitted != len(targetsToFetch) {
		t.Errorf("expected %d jobs submitted, got %d", len(targetsToFetch), stats.JobsSubmitted)
	}
}

// TestWorkerPool_ConcurrentSubmit tests concurrent job submission.
func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pool:concurrent")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	pool := NewWorkerPool(client, 5)
	defer pool.Close()

	// Submit jobs from multiple goroutines
	numGoroutines := 3
	jobsPerGoroutine := 2

	done := make(chan struct{})
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < jobsPerGoroutine; j++ {
				pool.Submit(targets.Random())
			}
		}()
	}

	// Wait for all submissions
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	pool.CloseJobs()

	// Collect results
	resultsCount := 0
	for result := range pool.Results() {
		resultsCount++
		if result.Response != nil {
			result.Response.Body.Close()
		}
	}

	expectedJobs := numGoroutines * jobsPerGoroutine
	if resultsCount != expectedJobs {
		t.Errorf("expected %d results, got %d", expectedJobs, resultsCount)
	}
}

// TestWorkerPool_Close tests graceful shutdown.
func TestWorkerPool_Close(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pool:close")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	pool := NewWorkerPool(client, 2)

	// Submit some jobs
	pool.Submit(targets.JSONPlaceholderPosts)
	pool.Submit(targets.HTTPBinGet)
	pool.CloseJobs()

	// Close should not error
	if err := pool.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Second close should also not error (idempotent)
	if err := pool.Close(); err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}
}

// TestWorkerPool_Stats tests statistics tracking.
func TestWorkerPool_Stats(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pool:stats")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	numWorkers := 4
	pool := NewWorkerPool(client, numWorkers)
	defer pool.Close()

	// Verify initial stats
	stats := pool.Stats()
	if stats.NumWorkers != numWorkers {
		t.Errorf("expected %d workers, got %d", numWorkers, stats.NumWorkers)
	}
	if stats.JobsSubmitted != 0 {
		t.Errorf("expected 0 jobs submitted initially, got %d", stats.JobsSubmitted)
	}

	// Submit jobs and verify count
	pool.Submit(targets.JSONPlaceholderPosts)
	pool.Submit(targets.HTTPBinGet)

	stats = pool.Stats()
	if stats.JobsSubmitted != 2 {
		t.Errorf("expected 2 jobs submitted, got %d", stats.JobsSubmitted)
	}

	// Test String() method
	statsStr := stats.String()
	if statsStr == "" {
		t.Error("Stats.String() returned empty string")
	}
}

// TestPipeline_Run tests basic pipeline functionality.
func TestPipeline_Run(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pipeline:run")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	pipeline := NewPipeline(client, 2)

	targetsToFetch := []*targets.Target{
		targets.JSONPlaceholderPosts,
		targets.HTTPBinGet,
		targets.UUIDGenerator,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := pipeline.Run(ctx, targetsToFetch)

	// Collect results
	resultsCount := 0
	for result := range results {
		resultsCount++
		if result == nil {
			t.Error("received nil result")
		}
		if result.Response != nil {
			result.Response.Body.Close()
		}
	}

	if resultsCount != len(targetsToFetch) {
		t.Errorf("expected %d results, got %d", len(targetsToFetch), resultsCount)
	}
}

// TestPipeline_ContextCancellation tests pipeline cancellation.
func TestPipeline_ContextCancellation(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pipeline:cancel")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	pipeline := NewPipeline(client, 2)

	// Use many targets to ensure pipeline is still running when we cancel
	targetsToFetch := make([]*targets.Target, 20)
	for i := range targetsToFetch {
		targetsToFetch[i] = targets.Random()
	}

	ctx, cancel := context.WithCancel(context.Background())

	results := pipeline.Run(ctx, targetsToFetch)

	// Cancel after receiving first result
	firstResult := <-results
	if firstResult != nil && firstResult.Response != nil {
		firstResult.Response.Body.Close()
	}
	cancel()

	// Drain remaining results
	for result := range results {
		if result != nil && result.Response != nil {
			result.Response.Body.Close()
		}
	}

	// Pipeline should have stopped gracefully
}

// TestPipeline_EmptyTargets tests pipeline with no targets.
func TestPipeline_EmptyTargets(t *testing.T) {
	client, err := realclient.NewClient("localhost:50051", "test:pipeline:empty")
	if err != nil {
		t.Skipf("Skipping integration test: rate limiter not available: %v", err)
	}
	defer client.Close()

	pipeline := NewPipeline(client, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := pipeline.Run(ctx, []*targets.Target{})

	// Should receive no results
	resultsCount := 0
	for range results {
		resultsCount++
	}

	if resultsCount != 0 {
		t.Errorf("expected 0 results for empty targets, got %d", resultsCount)
	}
}
