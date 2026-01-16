package slidingwindowcounter_test

import (
	"sync"
	"testing"

	"github.com/ratelimiter/go/pkg/algorithm/slidingwindowcounter"
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
		bucketNanos int64
		limit       int
	}{
		{"60s_window_1s_buckets", 60_000_000_000, 1_000_000_000, 100},
		{"10s_window_1s_buckets", 10_000_000_000, 1_000_000_000, 50},
		{"1s_window_100ms_buckets", 1_000_000_000, 100_000_000, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			swc, err := slidingwindowcounter.New(clk, tt.windowNanos, tt.bucketNanos, tt.limit)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			if swc == nil {
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
		bucketNanos int64
		limit       int
	}{
		{"zero_window", 0, 1_000_000_000, 100},
		{"zero_bucket", 10_000_000_000, 0, 100},
		{"negative_window", -10_000_000_000, 1_000_000_000, 100},
		{"negative_bucket", 10_000_000_000, -1_000_000_000, 100},
		{"not_divisible", 10_000_000_000, 3_000_000_000, 100},
		{"zero_limit", 10_000_000_000, 1_000_000_000, 0},
		{"negative_limit", 10_000_000_000, 1_000_000_000, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			swc, err := slidingwindowcounter.New(clk, tt.windowNanos, tt.bucketNanos, tt.limit)
			if err == nil {
				t.Error("New() expected error, got nil")
			}
			if swc != nil {
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
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	result, err := swc.TryAcquire("user1", 5)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionAllow {
		t.Errorf("expected ALLOW, got %v", result.Decision)
	}
}

func TestReject_WhenExceedingLimit(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	// Consume all permits
	swc.TryAcquire("user1", 10)

	// Next request rejected
	result, err := swc.TryAcquire("user1", 1)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionReject {
		t.Errorf("expected REJECT, got %v", result.Decision)
	}
}

func TestTryAcquire_InvalidPermits(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	invalidPermits := []int{0, -1, -100}

	for _, permits := range invalidPermits {
		_, err := swc.TryAcquire("user1", permits)
		if err == nil {
			t.Errorf("TryAcquire(permits=%d) expected error, got nil", permits)
		}
	}
}

// =============================================================================
// Bucket Rolling Tests
// =============================================================================

// TestBucketRolling_SingleBucketStep verifies rolling forward one bucket.
func TestBucketRolling_SingleBucketStep(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10) // 10 buckets

	// Add 5 permits at t=0 (bucket 0)
	swc.TryAcquire("user1", 5)

	// Advance to next bucket (t=1s, bucket 1)
	clk.SetNanos(1_000_000_000)

	// Should have 5 permits available (bucket 0 still in window)
	result, _ := swc.TryAcquire("user1", 6)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection with 5+6=11 > limit")
	}

	result, _ = swc.TryAcquire("user1", 5)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow with 5+5=10 = limit")
	}
}

// TestBucketRolling_MultipleBuckets verifies rolling multiple buckets.
func TestBucketRolling_MultipleBuckets(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 5_000_000_000, 1_000_000_000, 10) // 5 buckets

	// Fill bucket 0
	swc.TryAcquire("user1", 10)

	// Advance 6 seconds (beyond window)
	clk.SetNanos(6_000_000_000)

	// All buckets should be cleared
	result, _ := swc.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected full capacity after rolling beyond window")
	}
}

// TestBucketRolling_ClearAll verifies that rolling >= numBuckets clears everything.
func TestBucketRolling_ClearAll(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 4_000_000_000, 1_000_000_000, 20) // 4 buckets

	// Fill up
	swc.TryAcquire("user1", 20)

	// Advance more than window (way past all buckets)
	clk.SetNanos(100_000_000_000)

	// Should have full capacity
	result, _ := swc.TryAcquire("user1", 20)
	if result.Decision != model.DecisionAllow {
		t.Error("expected full capacity after clearing all buckets")
	}
}

// TestRingBuffer_Wraparound verifies ring buffer index wraparound.
func TestRingBuffer_Wraparound(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 4_000_000_000, 1_000_000_000, 10) // 4 buckets

	// Add to buckets 0, 1, 2, 3
	for i := int64(0); i < 4; i++ {
		clk.SetNanos(i * 1_000_000_000)
		swc.TryAcquire("user1", 1)
	}

	// Move to bucket 4 (wraps to index 0)
	clk.SetNanos(4_000_000_000)
	swc.TryAcquire("user1", 1)

	// Window is [1s, 5s], should have buckets 1,2,3,4 = 4 permits
	result, _ := swc.TryAcquire("user1", 7)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection (4+7=11 > 10)")
	}

	result, _ = swc.TryAcquire("user1", 6)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow (4+6=10 = limit)")
	}
}

// =============================================================================
// Approximation Tests
// =============================================================================

// TestApproximation_BetterThanFixedWindow verifies reduction of boundary problem.
func TestApproximation_BetterThanFixedWindow(t *testing.T) {
	clk := clock.NewManualClock(0)

	// 10 permits per 10 seconds, using 10 buckets (1 second each)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	// At t=0: add 10 permits (fills bucket 0)
	swc.TryAcquire("user1", 10)

	// At t=1s: window=[0s,10s], bucket 0 has 10, new bucket 1 is empty
	clk.SetNanos(1_000_000_000)

	// With Fixed Window, this would allow 10 more (boundary problem)
	// With Sliding Window Counter, bucket 0 is still counted
	result, _ := swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection (better than Fixed Window)")
	}

	// Only after bucket 0 expires (at 10s) will we have capacity
	clk.SetNanos(10_000_000_001)
	result, _ = swc.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow after bucket expires")
	}
}

// TestApproximation_LessPreciseThanLog demonstrates approximation.
func TestApproximation_LessPreciseThanLog(t *testing.T) {
	clk := clock.NewManualClock(0)

	// 3 permits per 10s, using 2 buckets (5s each) - coarse granularity
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 5_000_000_000, 3)

	// At t=0: add 3 permits (bucket 0)
	swc.TryAcquire("user1", 3)

	// At t=4.9s: still in bucket 0, all counts still there
	clk.SetNanos(4_900_000_000)
	result, _ := swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection at 4.9s")
	}

	// At t=5.0s: moved to bucket 1, but bucket 0 still in window
	clk.SetNanos(5_000_000_000)
	result, _ = swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection at 5.0s (bucket 0 still counted)")
	}

	// Only at t=10s+ will bucket 0 expire
	clk.SetNanos(10_000_000_001)
	result, _ = swc.TryAcquire("user1", 3)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow at 10s (bucket 0 expired)")
	}

	t.Log("Approximation: All 3 permits in bucket 0 expire together at 10s")
	t.Log("Sliding Window Log would expire each permit individually")
}

// =============================================================================
// Retry-After Tests
// =============================================================================

func TestRetryAfter_NextBucketBoundary(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	// Fill capacity
	swc.TryAcquire("user1", 10)

	// At t=500ms (mid-bucket)
	clk.SetNanos(500_000_000)

	result, _ := swc.TryAcquire("user1", 1)

	// Should retry after 500ms (next bucket at 1s)
	expected := int64(500_000_000)
	if result.RetryAfterNanos != expected {
		t.Errorf("expected RetryAfterNanos=%d, got %d",
			expected, result.RetryAfterNanos)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestConcurrent_MultipleGoroutines(t *testing.T) {
	clk := clock.NewSystemClock()
	swc, _ := slidingwindowcounter.New(clk, 1_000_000_000, 100_000_000, 1000)

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
				result, _ := swc.TryAcquire("user1", 1)
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

func TestBucketAlignment_OnStartup(t *testing.T) {
	// Test that buckets align correctly regardless of startup time
	clk := clock.NewManualClock(5_700_000_000) // Start at 5.7s

	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	// Should be in bucket starting at 5s
	result, _ := swc.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow on startup")
	}
}

func TestSameBucket_NoRolling(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	// Multiple requests in same bucket
	swc.TryAcquire("user1", 3)
	swc.TryAcquire("user1", 3)
	swc.TryAcquire("user1", 3)

	// Should have consumed 9 total
	result, _ := swc.TryAcquire("user1", 2)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection after 9+2=11")
	}

	result, _ = swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow for 9+1=10")
	}
}

func TestRequestExceedingLimit_Rejected(t *testing.T) {
	clk := clock.NewManualClock(0)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 1_000_000_000, 10)

	result, _ := swc.TryAcquire("user1", 20)
	if result.Decision != model.DecisionReject {
		t.Error("expected request exceeding limit to be rejected")
	}
}

// =============================================================================
// Granularity Tests
// =============================================================================

// TestGranularity_FineBuckets verifies behavior with many small buckets.
func TestGranularity_FineBuckets(t *testing.T) {
	clk := clock.NewManualClock(0)

	// 10s window, 100ms buckets = 100 buckets (fine granularity)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 100_000_000, 10)

	// Add permits gradually
	for i := int64(0); i < 10; i++ {
		clk.SetNanos(i * 100_000_000) // Every 100ms
		swc.TryAcquire("user1", 1)
	}

	// At 10s, all should still be in window
	clk.SetNanos(999_999_999)
	result, _ := swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection (all events still in window)")
	}

	// At 10s + 100ms, first event expires
	clk.SetNanos(10_100_000_000)
	result, _ = swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow (first bucket expired)")
	}
}

// TestGranularity_CoarseBuckets verifies behavior with few large buckets.
func TestGranularity_CoarseBuckets(t *testing.T) {
	clk := clock.NewManualClock(0)

	// 10s window, 10s buckets = 1 bucket (degrades to Fixed Window)
	swc, _ := slidingwindowcounter.New(clk, 10_000_000_000, 10_000_000_000, 10)

	swc.TryAcquire("user1", 10)

	// At 9.9s: still same bucket
	clk.SetNanos(9_900_000_000)
	result, _ := swc.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection")
	}

	// At 10s: bucket resets (like Fixed Window)
	clk.SetNanos(10_000_000_000)
	result, _ = swc.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected allow (bucket reset)")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkTryAcquire_Allow(b *testing.B) {
	clk := clock.NewSystemClock()
	swc, _ := slidingwindowcounter.New(clk, 60_000_000_000, 1_000_000_000, 1_000_000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		swc.TryAcquire("user1", 1)
	}
}

func BenchmarkTryAcquire_Reject(b *testing.B) {
	clk := clock.NewSystemClock()
	swc, _ := slidingwindowcounter.New(clk, 60_000_000_000, 1_000_000_000, 10)

	swc.TryAcquire("user1", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		swc.TryAcquire("user1", 1)
	}
}

func BenchmarkTryAcquire_Parallel(b *testing.B) {
	clk := clock.NewSystemClock()
	swc, _ := slidingwindowcounter.New(clk, 60_000_000_000, 1_000_000_000, 1_000_000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			swc.TryAcquire("user1", 1)
		}
	})
}

func BenchmarkNew(b *testing.B) {
	clk := clock.NewSystemClock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slidingwindowcounter.New(clk, 60_000_000_000, 1_000_000_000, 100)
	}
}
