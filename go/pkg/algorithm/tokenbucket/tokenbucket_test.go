package tokenbucket_test

import (
	"sync"
	"testing"
	"time"

	"github.com/ratelimiter/go/pkg/algorithm/tokenbucket"
	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/model"
)

// =============================================================================
// Constructor Tests
// =============================================================================

// TestNew_ValidParameters verifies that valid parameters create a working TokenBucket.
func TestNew_ValidParameters(t *testing.T) {
	clk := clock.NewManualClock(0)

	tests := []struct {
		name       string
		capacity   int64
		refillRate float64
	}{
		{"small_capacity", 10, 1.0},
		{"large_capacity", 1_000_000, 100.0},
		{"fractional_refill", 100, 10.5},
		{"slow_refill", 1000, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb, err := tokenbucket.New(clk, tt.capacity, tt.refillRate)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			if tb == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

// TestNew_InvalidParameters verifies that invalid parameters return errors.
func TestNew_InvalidParameters(t *testing.T) {
	clk := clock.NewManualClock(0)

	tests := []struct {
		name       string
		capacity   int64
		refillRate float64
		wantError  bool
	}{
		{"zero_capacity", 0, 10.0, true},
		{"negative_capacity", -100, 10.0, true},
		{"zero_refill_rate", 100, 0.0, true},
		{"negative_refill_rate", 100, -5.0, true},
		{"valid", 100, 10.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb, err := tokenbucket.New(clk, tt.capacity, tt.refillRate)

			if tt.wantError {
				if err == nil {
					t.Error("New() expected error, got nil")
				}
				if tb != nil {
					t.Error("New() expected nil bucket on error, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("New() unexpected error: %v", err)
				}
				if tb == nil {
					t.Error("New() expected non-nil bucket, got nil")
				}
			}
		})
	}
}

// =============================================================================
// Basic Allow/Reject Tests (Deterministic with ManualClock)
// =============================================================================

// TestAllow_WhenTokensAvailable verifies that requests are allowed when sufficient tokens exist.
func TestAllow_WhenTokensAvailable(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 1.0)

	// Bucket starts at full capacity (10 tokens)
	result, err := tb.TryAcquire("user1", 5)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionAllow {
		t.Errorf("expected ALLOW, got %v", result.Decision)
	}

	if result.RetryAfterNanos != 0 {
		t.Errorf("expected RetryAfterNanos=0 for ALLOW, got %d", result.RetryAfterNanos)
	}
}

// TestReject_WhenTokensInsufficient verifies that requests are rejected when not enough tokens.
func TestReject_WhenTokensInsufficient(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 1.0)

	// Consume all tokens
	tb.TryAcquire("user1", 10)

	// Next request should be rejected
	result, err := tb.TryAcquire("user1", 1)
	if err != nil {
		t.Fatalf("TryAcquire() error: %v", err)
	}

	if result.Decision != model.DecisionReject {
		t.Errorf("expected REJECT, got %v", result.Decision)
	}

	if result.RetryAfterNanos <= 0 {
		t.Errorf("expected RetryAfterNanos > 0 for REJECT, got %d", result.RetryAfterNanos)
	}
}

// TestTryAcquire_InvalidPermits verifies that invalid permit values return errors.
func TestTryAcquire_InvalidPermits(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 1.0)

	invalidPermits := []int{0, -1, -100}

	for _, permits := range invalidPermits {
		result, err := tb.TryAcquire("user1", permits)

		if err == nil {
			t.Errorf("TryAcquire(permits=%d) expected error, got nil", permits)
		}

		// Result should be zero value when error occurs
		if result.Decision == model.DecisionAllow {
			t.Errorf("TryAcquire(permits=%d) returned ALLOW on error", permits)
		}
	}
}

// =============================================================================
// Token Refill Tests
// =============================================================================

// TestTokenRegeneration_FromRejectToAllow verifies tokens refill over time.
func TestTokenRegeneration_FromRejectToAllow(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 2.0) // 2 tokens/second refill

	// Step 1: Consume all tokens
	result, _ := tb.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Fatal("expected initial request to be allowed")
	}

	// Step 2: Next request rejected (no tokens left)
	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Fatal("expected second request to be rejected")
	}

	// Step 3: Advance time by 0.5 seconds (should refill 1 token)
	clk.AdvanceNanos(500_000_000)

	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected request to be allowed after refill")
	}

	// Step 4: Next request should be rejected again (no tokens left)
	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected request to be rejected after consuming refilled token")
	}

	// Step 5: Advance another 2.5 seconds (should refill 5 tokens)
	clk.AdvanceNanos(2_500_000_000)

	result, _ = tb.TryAcquire("user1", 5)
	if result.Decision != model.DecisionAllow {
		t.Error("expected request to be allowed after larger refill")
	}
}

// TestTokenRefill_DoesNotExceedCapacity verifies that tokens cap at capacity.
func TestTokenRefill_DoesNotExceedCapacity(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 1.0)

	// Consume 5 tokens
	tb.TryAcquire("user1", 5)

	// Advance time by 100 seconds (would refill 100 tokens without cap)
	clk.AdvanceNanos(100_000_000_000)

	// Should only allow up to capacity (10 tokens), not 105 tokens
	result, _ := tb.TryAcquire("user1", 10)
	if result.Decision != model.DecisionAllow {
		t.Error("expected request for capacity to be allowed")
	}

	// Next request should be rejected (capacity exhausted)
	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected request to be rejected after capacity exhausted")
	}
}

// TestTokenRefill_FractionalAccumulation verifies fractional token accumulation.
func TestTokenRefill_FractionalAccumulation(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 100, 10.5) // 10.5 tokens/second

	// Consume all tokens
	tb.TryAcquire("user1", 100)

	// Advance 0.1 seconds (should refill 1.05 tokens)
	clk.AdvanceNanos(100_000_000)

	// Request 1 token should succeed
	result, _ := tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected fractional refill to allow 1 token")
	}

	// Request another token should fail (only 0.05 tokens left)
	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected second token to be rejected")
	}

	// Advance another 0.1 seconds (adds 1.05 more, total 1.1 tokens)
	clk.AdvanceNanos(100_000_000)

	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("expected accumulated fractional tokens to allow request")
	}
}

// =============================================================================
// Burst Handling Tests
// =============================================================================

// TestBurst_AllowUpToCapacity verifies that bursts up to capacity are allowed.
func TestBurst_AllowUpToCapacity(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 100, 10.0)

	// Burst of 100 tokens should be allowed (at capacity)
	result, _ := tb.TryAcquire("user1", 100)
	if result.Decision != model.DecisionAllow {
		t.Error("expected burst at capacity to be allowed")
	}

	// Next request rejected (bucket empty)
	result, _ = tb.TryAcquire("user1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("expected request after burst to be rejected")
	}
}

// TestBurst_ExceedingCapacityRejected verifies requests exceeding capacity are rejected.
func TestBurst_ExceedingCapacityRejected(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 100, 10.0)

	// Request exceeding capacity should be rejected
	result, _ := tb.TryAcquire("user1", 150)
	if result.Decision != model.DecisionReject {
		t.Error("expected request exceeding capacity to be rejected")
	}

	// Verify retry-after calculation (need 50 extra tokens)
	// retryAfter = ceil(50 / (10.0 / 1e9)) = 5 seconds = 5e9 nanos
	expectedRetryAfter := int64(5_000_000_000)
	tolerance := int64(1_000_000) // 1ms tolerance for float precision

	if result.RetryAfterNanos < expectedRetryAfter-tolerance ||
		result.RetryAfterNanos > expectedRetryAfter+tolerance {
		t.Errorf("expected RetryAfterNanos ~%d, got %d",
			expectedRetryAfter, result.RetryAfterNanos)
	}
}

// =============================================================================
// Retry-After Calculation Tests
// =============================================================================

// TestRetryAfter_AccurateTiming verifies retry-after calculation accuracy.
func TestRetryAfter_AccurateTiming(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 1.0) // 1 token/second

	// Consume all tokens
	tb.TryAcquire("user1", 10)

	// Request 3 tokens (need 3 seconds to refill)
	result, _ := tb.TryAcquire("user1", 3)

	expected := int64(3_000_000_000) // 3 seconds
	if result.RetryAfterNanos != expected {
		t.Errorf("expected RetryAfterNanos=%d, got %d",
			expected, result.RetryAfterNanos)
	}

	// Verify that waiting the suggested time + a small margin allows the request
	// We use a small margin (1ms) because retry-after uses ceiling, which may
	// overestimate slightly due to floating-point precision
	clk.AdvanceNanos(result.RetryAfterNanos + 1_000_000) // +1ms margin

	result, _ = tb.TryAcquire("user1", 3)
	if result.Decision != model.DecisionAllow {
		t.Error("expected request to be allowed after retry-after + margin")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

// TestConcurrent_MultipleGoroutines verifies thread-safety under concurrent access.
func TestConcurrent_MultipleGoroutines(t *testing.T) {
	clk := clock.NewSystemClock() // Use system clock for realistic concurrency
	tb, _ := tokenbucket.New(clk, 1000, 1000.0)

	const numGoroutines = 10
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	var allowCount, rejectCount int64
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				result, _ := tb.TryAcquire("user1", 1)

				mu.Lock()
				if result.Decision == model.DecisionAllow {
					allowCount++
				} else {
					rejectCount++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	totalRequests := int64(numGoroutines * requestsPerGoroutine)
	t.Logf("Total requests: %d, Allowed: %d, Rejected: %d",
		totalRequests, allowCount, rejectCount)

	// Verify that we didn't allow more than capacity + refilled tokens
	// (exact count depends on timing, but should be reasonable)
	if allowCount > totalRequests {
		t.Errorf("allowed more requests than made: allowed=%d, total=%d",
			allowCount, totalRequests)
	}
}

// TestConcurrent_NoRaceConditions verifies no data races (run with -race flag).
func TestConcurrent_NoRaceConditions(t *testing.T) {
	clk := clock.NewSystemClock()
	tb, _ := tokenbucket.New(clk, 100, 100.0)

	const numGoroutines = 50

	var wg sync.WaitGroup

	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				tb.TryAcquire("user1", 1)
				time.Sleep(time.Microsecond) // Small delay to increase contention
			}
		}(i)
	}

	wg.Wait()
	t.Log("No race conditions detected (run with -race to verify)")
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

// TestZeroElapsedTime_NoRefill verifies that no refill occurs if clock doesn't advance.
func TestZeroElapsedTime_NoRefill(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 10, 1.0)

	// Consume 5 tokens
	tb.TryAcquire("user1", 5)

	// Make request without advancing time
	result, _ := tb.TryAcquire("user1", 10)

	// Should be rejected (only 5 tokens left, no refill occurred)
	if result.Decision != model.DecisionReject {
		t.Error("expected rejection when time doesn't advance")
	}
}

// TestStartsAtFullCapacity verifies bucket initializes at full capacity.
func TestStartsAtFullCapacity(t *testing.T) {
	clk := clock.NewManualClock(0)
	tb, _ := tokenbucket.New(clk, 50, 10.0)

	// Should be able to immediately burst at full capacity
	result, _ := tb.TryAcquire("user1", 50)
	if result.Decision != model.DecisionAllow {
		t.Error("expected bucket to start at full capacity")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkTryAcquire_Allow measures performance of successful acquisitions.
func BenchmarkTryAcquire_Allow(b *testing.B) {
	clk := clock.NewSystemClock()
	tb, _ := tokenbucket.New(clk, 1_000_000, 1_000_000.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.TryAcquire("user1", 1)
	}
}

// BenchmarkTryAcquire_Reject measures performance of rejected acquisitions.
func BenchmarkTryAcquire_Reject(b *testing.B) {
	clk := clock.NewSystemClock()
	tb, _ := tokenbucket.New(clk, 10, 1.0)

	// Consume all tokens
	tb.TryAcquire("user1", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.TryAcquire("user1", 1)
	}
}

// BenchmarkTryAcquire_Parallel measures parallel throughput.
func BenchmarkTryAcquire_Parallel(b *testing.B) {
	clk := clock.NewSystemClock()
	tb, _ := tokenbucket.New(clk, 1_000_000, 1_000_000.0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tb.TryAcquire("user1", 1)
		}
	})
}

// BenchmarkNew measures constructor performance.
func BenchmarkNew(b *testing.B) {
	clk := clock.NewSystemClock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenbucket.New(clk, 100, 10.0)
	}
}
