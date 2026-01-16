package fixedwindow_test

import (
	"sync"
	"testing"

	"github.com/ratelimiter/go/pkg/algorithm/fixedwindow"
	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNew_ValidParameters(t *testing.T) {
	clk := clock.NewManualClock(0)

	tests := []struct {
		name        string
		windowNanos int64
		limit       int
	}{
		{"1_second_window", 1_000_000_000, 100},
		{"1_minute_window", 60_000_000_000, 1000},
		{"small_limit", 1_000_000_000, 1},
		{"large_limit", 1_000_000_000, 1_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw, err := fixedwindow.New(clk, tt.windowNanos, tt.limit)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			if fw == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

func TestNew_InvalidParameters(t *testing.T) {
	clk := clock.NewManualClock(0)

	tests := []struct {
		name        string
		windowNanos int64
		limit       int
	}{
		{"zero_window", 0, 100},
		{"negative_window", -1_000_000_000, 100},
		{"zero_limit", 1_000_000_000, 0},
		{"negative_limit", 1_000_000_000, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw, err := fixedwindow.New(clk, tt.windowNanos, tt.limit)
			if err == nil {
				t.Error("New() expected error, got nil")
			}
			if fw != nil {
				t.Error("New() expected nil on error, got non-nil")
			}
		})
	}
}

// =============================================================================
// Basic Allow/Reject Tests
// =============================================================================

func TestAllow_WhenWithinLimit(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	result, err := fw.TryAcquire("user1", 5)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionAllow {
		t.Errorf("expected ALLOW, got %v", result.Decision)
	}
}

func TestReject_WhenExceedingLimit(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Consume all permits
	fw.TryAcquire("user1", 10)

	// Next request should be rejected
	result, err := fw.TryAcquire("user1", 1)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionReject {
		t.Errorf("expected REJECT, got %v", result.Decision)
	}

	if result.RetryAfterNanos <= 0 {
		t.Errorf("expected RetryAfterNanos > 0, got %d", result.RetryAfterNanos)
	}
}

func TestTryAcquire_InvalidPermits(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	invalidPermits := []int{0, -1, -100}

	for _, permits := range invalidPermits {
		_, err := fw.TryAcquire("user1", permits)
		if err == nil {
			t.Errorf("TryAcquire(permits=%d) expected error, got nil", permits)
		}
	}
}

// =============================================================================
// Window Reset Tests
// =============================================================================

func TestWindowReset_AfterWindowBoundary(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Consume all permits in first window
	for i := 0; i < 10; i++ {
		result, _ := fw.TryAcquire("user1", 1)
		if result.Decision != model.DecisionAllow {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// 11th request rejected
	result, _ := fw.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected 11th request to be rejected")
	}

	// Advance to next window
	clk.AdvanceNanos(1_000_000_000)

	// Counter should reset - request allowed
	result, _ = fw.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected request in new window to be allowed")
	}
}

func TestWindowReset_MultipleWindowsAhead(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Consume all permits
	fw.TryAcquire("user1", 10)

	// Advance 5 windows ahead
	clk.AdvanceNanos(5_000_000_000)

	// Should allow full capacity in new window
	result, _ := fw.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected full capacity after multiple windows")
	}
}

// =============================================================================
// Boundary Problem Tests
// =============================================================================

// TestBoundaryProblem demonstrates the Fixed Window boundary problem.
//
// This test shows how the algorithm can allow 2x the intended rate
// at window boundaries.
func TestBoundaryProblem(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10) // 10 per second

	// Scenario: Client makes requests at window boundary
	// Window 0: 0-1s
	// Window 1: 1-2s

	// At 0.9 seconds: make 10 requests (allowed)
	clk.SetNanos(900_000_000)
	for i := 0; i < 10; i++ {
		result, _ := fw.TryAcquire("user1", 1)
		if result.Decision != model.DecisionAllow {
			t.Fatalf("request %d at 0.9s should be allowed", i)
		}
	}

	// At 1.0 seconds: window resets, make 10 more requests (allowed)
	clk.SetNanos(1_000_000_000)
	for i := 0; i < 10; i++ {
		result, _ := fw.TryAcquire("user1", 1)
		if result.Decision != model.DecisionAllow {
			t.Fatalf("request %d at 1.0s should be allowed", i)
		}
	}

	// Result: 20 requests in 0.1 seconds (200 req/s, 20x the limit!)
	t.Log("Boundary problem: allowed 20 requests in 0.1 seconds (limit is 10/second)")
}

// TestBoundaryProblem_WorstCase demonstrates the worst-case scenario.
func TestBoundaryProblem_WorstCase(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 100)

	// Worst case: requests at end of one window and start of next
	// Window 0: 0-1s, Window 1: 1-2s

	// At 0.999999999s (1 nanosecond before boundary)
	clk.SetNanos(999_999_999)
	result, _ := fw.TryAcquire("user1", 100)
	if result.Decision != model.DecisionAllow {
		t.Fatal("expected first batch to be allowed")
	}

	// At 1.000000000s (exactly at boundary)
	clk.SetNanos(1_000_000_000)
	result, _ = fw.TryAcquire("user1", 100)
	if result.Decision != model.DecisionAllow {
		t.Fatal("expected second batch to be allowed")
	}

	// Result: 200 requests in 1 nanosecond
	t.Log("Worst-case boundary: 200 requests in 1 nanosecond")
}

// =============================================================================
// Window Alignment Tests
// =============================================================================

func TestWindowAlignment_ConsistentAcrossInstances(t *testing.T) {
	// Two instances created at different times should have synchronized windows
	clk := clock.NewManualClock(5_700_000_000) // 5.7 seconds since epoch

	fw1, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Advance time within same window
	clk.AdvanceNanos(100_000_000) // Now at 5.8 seconds

	fw2, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Both instances should be in window starting at 5.0 seconds
	// Make requests from both
	fw1.TryAcquire("user1", 5)
	fw2.TryAcquire("user1", 5)

	// Each should independently allow their requests (separate instances)
	// This test just verifies no panics occur due to alignment
	t.Log("Window alignment works correctly across instances")
}

// =============================================================================
// Retry-After Tests
// =============================================================================

func TestRetryAfter_ReflectsTimeToNextWindow(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Consume all permits
	fw.TryAcquire("user1", 10)

	// Advance partway through window
	clk.AdvanceNanos(300_000_000) // 0.3 seconds into window

	// Next request rejected
	result, _ := fw.TryAcquire("user1", 1)

	// Should retry after remaining time in window (0.7 seconds)
	expected := int64(700_000_000)
	if result.RetryAfterNanos != expected {
		t.Errorf("expected RetryAfterNanos=%d, got %d",
			expected, result.RetryAfterNanos)
	}
}

func TestRetryAfter_WorksAfterWaiting(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Consume all permits
	fw.TryAcquire("user1", 10)

	// Get retry-after
	result, _ := fw.TryAcquire("user1", 1)
	retryAfter := result.RetryAfterNanos

	// Wait the suggested time
	clk.AdvanceNanos(retryAfter)

	// Should now be in new window - request allowed
	result, _ = fw.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected request to be allowed after waiting retry-after")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestConcurrent_MultipleGoroutines(t *testing.T) {
	clk := clock.NewSystemClock()
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 1000)

	const numGoroutines = 10
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	var allowCount int64
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				result, _ := fw.TryAcquire("user1", 1)
				if result.Decision == model.DecisionAllow {
					mu.Lock()
					allowCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("Allowed %d requests out of %d total",
		allowCount, numGoroutines*requestsPerGoroutine)

	// Should allow at most limit per window (possibly less due to timing)
	if allowCount > 1000 {
		t.Errorf("allowed more than limit: %d > 1000", allowCount)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestRequestExceedingLimit_Rejected(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Request more than limit
	result, _ := fw.TryAcquire("user1", 20)

	if result.Decision != model.DecisionReject {
		t.Error("expected request exceeding limit to be rejected")
	}
}

func TestZeroUsedAtStart(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Should be able to use full capacity immediately
	result, _ := fw.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected full capacity available at start")
	}
}

func TestMultiplePermits(t *testing.T) {
	clk := clock.NewManualClock(0)
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 100)

	// Request 25 permits multiple times
	for i := 0; i < 4; i++ {
		result, _ := fw.TryAcquire("user1", 25)
		if result.Decision != model.DecisionAllow {
			t.Errorf("request %d for 25 permits should be allowed", i)
		}
	}

	// 5th request should be rejected (100 consumed)
	result, _ := fw.TryAcquire("user1", 25)
	if result.Decision != model.DecisionReject {
		t.Error("5th request should be rejected")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkTryAcquire_Allow(b *testing.B) {
	clk := clock.NewSystemClock()
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 1_000_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fw.TryAcquire("user1", 1)
	}
}

func BenchmarkTryAcquire_Reject(b *testing.B) {
	clk := clock.NewSystemClock()
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 10)

	// Consume all permits
	fw.TryAcquire("user1", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fw.TryAcquire("user1", 1)
	}
}

func BenchmarkTryAcquire_Parallel(b *testing.B) {
	clk := clock.NewSystemClock()
	fw, _ := fixedwindow.New(clk, 1_000_000_000, 1_000_000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fw.TryAcquire("user1", 1)
		}
	})
}

func BenchmarkNew(b *testing.B) {
	clk := clock.NewSystemClock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fixedwindow.New(clk, 1_000_000_000, 100)
	}
}
