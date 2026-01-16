package slidingwindow_test

import (
	"sync"
	"testing"

	"github.com/ratelimiter/go/pkg/algorithm/slidingwindow"
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
		{"10_second_window", 10_000_000_000, 100},
		{"1_minute_window", 60_000_000_000, 1000},
		{"small_limit", 1_000_000_000, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			swl, err := slidingwindow.New(clk, tt.windowNanos, tt.limit)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			if swl == nil {
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
			swl, err := slidingwindow.New(clk, tt.windowNanos, tt.limit)
			if err == nil {
				t.Error("New() expected error, got nil")
			}
			if swl != nil {
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
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	result, err := swl.TryAcquire("user1", 5)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionAllow {
		t.Errorf("expected ALLOW, got %v", result.Decision)
	}
}

func TestReject_WhenExceedingLimit(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	// Consume all permits
	swl.TryAcquire("user1", 10)

	// Next request rejected
	result, err := swl.TryAcquire("user1", 1)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionReject {
		t.Errorf("expected REJECT, got %v", result.Decision)
	}
}

func TestTryAcquire_InvalidPermits(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	invalidPermits := []int{0, -1, -100}

	for _, permits := range invalidPermits {
		_, err := swl.TryAcquire("user1", permits)
		if err == nil {
			t.Errorf("TryAcquire(permits=%d) expected error, got nil", permits)
		}
	}
}

// =============================================================================
// Sliding Window Behavior Tests
// =============================================================================

// TestSlidingWindow_GradualEviction verifies events expire gradually.
func TestSlidingWindow_GradualEviction(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 5) // 5 per 10 seconds

	// Make 5 requests at different times
	times := []int64{0, 1_000_000_000, 2_000_000_000, 3_000_000_000, 4_000_000_000}

	for _, tm := range times {
		clk.SetNanos(tm)
		result, _ := swl.TryAcquire("user1", 1)
		if result.Decision != model.DecisionAllow {
			t.Fatalf("request at %d should be allowed", tm)
		}
	}

	// At 5s: window full, next request rejected
	clk.SetNanos(5_000_000_000)
	result, _ := swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection at 5s")
	}

	// At 10s + 1ns: first event (at 0s) expires
	clk.SetNanos(10_000_000_001)
	result, _ = swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow at 10s (first event expired)")
	}

	// At 11s + 1ns: second event (at 1s) expires
	clk.SetNanos(11_000_000_001)
	result, _ = swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow at 11s (second event expired)")
	}
}

// TestSlidingWindow_NoBoundaryProblem verifies no boundary problem.
//
// Unlike Fixed Window, Sliding Window should not allow bursts at boundaries.
func TestSlidingWindow_NoBoundaryProblem(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 1_000_000_000, 10) // 10 per second

	// At 0.9s: make 10 requests
	clk.SetNanos(900_000_000)
	for i := 0; i < 10; i++ {
		result, _ := swl.TryAcquire("user1", 1)
		if result.Decision != model.DecisionAllow {
			t.Fatalf("request %d at 0.9s should be allowed", i)
		}
	}

	// At 1.0s: next request should be REJECTED (window still contains 0.9s requests)
	clk.SetNanos(1_000_000_000)
	result, _ := swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection at 1.0s (no boundary problem)")
	}

	// Only after 1.9s will the 0.9s requests expire
	clk.SetNanos(1_900_000_001)
	result, _ = swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow at 1.9s (events expired)")
	}
}

// TestSlidingWindow_TrueSliding verifies the window truly slides with time.
func TestSlidingWindow_TrueSliding(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 5_000_000_000, 3) // 3 per 5 seconds

	// Request pattern:
	// t=0s: 1 request
	// t=2s: 1 request
	// t=4s: 1 request

	clk.SetNanos(0)
	swl.TryAcquire("user1", 1)

	clk.SetNanos(2_000_000_000)
	swl.TryAcquire("user1", 1)

	clk.SetNanos(4_000_000_000)
	swl.TryAcquire("user1", 1)

	// At t=4.5s: window contains [0s, 2s, 4s] = 3 events, reject
	clk.SetNanos(4_500_000_000)
	result, _ := swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection at 4.5s")
	}

	// At t=5s + 1ns: window is [0s+5s, now] = [5s, 5s] - event at 0s expired
	clk.SetNanos(5_000_000_001)
	result, _ = swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow at 5s (window slid forward)")
	}
}

// =============================================================================
// Event Pruning Tests
// =============================================================================

// TestPruning_RemovesExpiredEvents verifies old events are removed.
func TestPruning_RemovesExpiredEvents(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 5_000_000_000, 10)

	// Add 5 events at t=0
	for i := 0; i < 5; i++ {
		swl.TryAcquire("user1", 1)
	}

	// Advance far into future (all events should expire)
	clk.SetNanos(100_000_000_000)

	// Should allow full capacity (all events pruned)
	result, _ := swl.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected full capacity after pruning")
	}
}

// TestPruning_PreservesValidEvents verifies valid events are kept.
func TestPruning_PreservesValidEvents(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	// Add events at t=0, 6s, 10s
	swl.TryAcquire("user1", 1) // t=0

	clk.SetNanos(6_000_000_000)
	swl.TryAcquire("user1", 1) // t=6s

	clk.SetNanos(10_000_000_000)
	swl.TryAcquire("user1", 1) // t=10s

	// At t=15s: window=(5s, 15s], should have 2 events (6s and 10s)
	// Event at t=0 is outside window, t=6s and t=10s are inside
	clk.SetNanos(15_000_000_000)

	// Should allow 8 more (10 limit - 2 existing)
	result, _ := swl.TryAcquire("user1", 8)
	if result.Decision != model.DecisionAllow {
		t.Error("expected 8 permits to be allowed")
	}

	// 9th would exceed (2 + 8 + 1 = 11 > 10)
	result, _ = swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected 9th permit to be rejected")
	}
}

// =============================================================================
// Retry-After Tests
// =============================================================================

func TestRetryAfter_ReflectsOldestEvent(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 2)

	// Add 2 events at t=0
	swl.TryAcquire("user1", 2)

	// At t=3s: try to add another (rejected)
	clk.SetNanos(3_000_000_000)
	result, _ := swl.TryAcquire("user1", 1)

	// Should retry after 7 seconds (when first event at 0s expires)
	expected := int64(7_000_000_000)
	if result.RetryAfterNanos != expected {
		t.Errorf("expected RetryAfterNanos=%d, got %d",
			expected, result.RetryAfterNanos)
	}
}

func TestRetryAfter_WorksAfterWaiting(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 5_000_000_000, 1)

	// Add 1 event at t=0
	swl.TryAcquire("user1", 1)

	// Get retry-after
	result, _ := swl.TryAcquire("user1", 1)
	retryAfter := result.RetryAfterNanos

	// Wait the suggested time + 1ns
	clk.AdvanceNanos(retryAfter + 1)

	// Should now be allowed
	result, _ = swl.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow after waiting retry-after")
	}
}

// =============================================================================
// Multiple Permits Tests
// =============================================================================

func TestMultiplePermits_AllSameTimestamp(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	// Request 5 permits at once
	result, _ := swl.TryAcquire("user1", 5)
	if result.Decision != model.DecisionAllow {
		t.Fatal("expected 5 permits to be allowed")
	}

	// Should have consumed 5 slots
	result, _ = swl.TryAcquire("user1", 6)
	if result.Decision != model.DecisionReject {
		t.Error("expected 6 more permits to be rejected")
	}

	result, _ = swl.TryAcquire("user1", 5)
	if result.Decision != model.DecisionAllow {
		t.Error("expected 5 more permits to be allowed (10 total)")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestConcurrent_MultipleGoroutines(t *testing.T) {
	clk := clock.NewSystemClock()
	swl, _ := slidingwindow.New(clk, 1_000_000_000, 1000)

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
				result, _ := swl.TryAcquire("user1", 1)
				if result.Decision == model.DecisionAllow {
					mu.Lock()
					allowCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("Allowed %d requests", allowCount)
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestRequestExceedingLimit_Rejected(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	result, _ := swl.TryAcquire("user1", 20)
	if result.Decision != model.DecisionReject {
		t.Error("expected request exceeding limit to be rejected")
	}
}

func TestEmptyWindow_AllowsRequests(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 10_000_000_000, 10)

	// Fresh window should allow requests
	result, _ := swl.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected empty window to allow requests")
	}
}

func TestAllEventsExpire_ResetsToEmpty(t *testing.T) {
	clk := clock.NewManualClock(0)
	swl, _ := slidingwindow.New(clk, 5_000_000_000, 5)

	// Fill window
	swl.TryAcquire("user1", 5)

	// Advance beyond window
	clk.SetNanos(10_000_000_000)

	// Should have full capacity again
	result, _ := swl.TryAcquire("user1", 5)
	if result.Decision != model.DecisionAllow {
		t.Error("expected full capacity after all events expire")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkTryAcquire_Allow(b *testing.B) {
	clk := clock.NewSystemClock()
	swl, _ := slidingwindow.New(clk, 1_000_000_000, 1_000_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		swl.TryAcquire("user1", 1)
	}
}

func BenchmarkTryAcquire_Reject(b *testing.B) {
	clk := clock.NewSystemClock()
	swl, _ := slidingwindow.New(clk, 1_000_000_000, 10)

	swl.TryAcquire("user1", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		swl.TryAcquire("user1", 1)
	}
}

func BenchmarkTryAcquire_Parallel(b *testing.B) {
	clk := clock.NewSystemClock()
	swl, _ := slidingwindow.New(clk, 1_000_000_000, 1_000_000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			swl.TryAcquire("user1", 1)
		}
	})
}

func BenchmarkNew(b *testing.B) {
	clk := clock.NewSystemClock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slidingwindow.New(clk, 10_000_000_000, 100)
	}
}
