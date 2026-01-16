package model_test

import (
	"testing"

	"github.com/ratelimiter/go/pkg/model"
)

// TestDecision_String verifies that Decision.String() returns correct values.
func TestDecision_String(t *testing.T) {
	tests := []struct {
		name     string
		decision model.Decision
		want     string
	}{
		{"allow", model.DecisionAllow, "ALLOW"},
		{"reject", model.DecisionReject, "REJECT"},
		{"unknown", model.Decision(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.String(); got != tt.want {
				t.Errorf("Decision.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAllow verifies that Allow() creates a correct ALLOW result.
func TestAllow(t *testing.T) {
	result := model.Allow()

	if result.Decision != model.DecisionAllow {
		t.Errorf("Allow().Decision = %v, want DecisionAllow", result.Decision)
	}

	if result.RetryAfterNanos != 0 {
		t.Errorf("Allow().RetryAfterNanos = %d, want 0", result.RetryAfterNanos)
	}
}

// TestReject verifies that Reject() creates correct REJECT results.
func TestReject(t *testing.T) {
	tests := []struct {
		name             string
		retryAfterNanos  int64
		expectedRetryAfter int64
	}{
		{"positive", 1_000_000_000, 1_000_000_000},
		{"zero", 0, 0},
		{"negative_clamped", -500_000_000, 0}, // Negative values should be clamped to 0
		{"large", 9_999_999_999_999, 9_999_999_999_999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.Reject(tt.retryAfterNanos)

			if result.Decision != model.DecisionReject {
				t.Errorf("Reject().Decision = %v, want DecisionReject", result.Decision)
			}

			if result.RetryAfterNanos != tt.expectedRetryAfter {
				t.Errorf("Reject(%d).RetryAfterNanos = %d, want %d",
					tt.retryAfterNanos, result.RetryAfterNanos, tt.expectedRetryAfter)
			}
		})
	}
}

// TestReject_NegativeClamping specifically tests that negative values are clamped.
func TestReject_NegativeClamping(t *testing.T) {
	negativeValues := []int64{
		-1,
		-1_000_000_000,
		-9_999_999_999_999,
	}

	for _, val := range negativeValues {
		result := model.Reject(val)
		if result.RetryAfterNanos != 0 {
			t.Errorf("Reject(%d).RetryAfterNanos = %d, want 0 (clamped)",
				val, result.RetryAfterNanos)
		}
	}
}

// TestRateLimitResult_Invariants verifies that factory functions maintain invariants.
func TestRateLimitResult_Invariants(t *testing.T) {
	// Invariant 1: Allow() always has RetryAfterNanos = 0
	allowResult := model.Allow()
	if allowResult.Decision == model.DecisionAllow && allowResult.RetryAfterNanos != 0 {
		t.Error("Allow() violated invariant: RetryAfterNanos must be 0 for ALLOW decisions")
	}

	// Invariant 2: Reject() never has negative RetryAfterNanos
	rejectResults := []model.RateLimitResult{
		model.Reject(1_000_000_000),
		model.Reject(0),
		model.Reject(-500_000_000), // Should be clamped to 0
	}

	for i, result := range rejectResults {
		if result.Decision == model.DecisionReject && result.RetryAfterNanos < 0 {
			t.Errorf("Reject() #%d violated invariant: RetryAfterNanos must be >= 0, got %d",
				i, result.RetryAfterNanos)
		}
	}
}

// TestDecisionValues verifies the iota values for Decision enum.
//
// While the specific values shouldn't matter for correct code, this test documents
// the expected values and catches unintentional reordering.
func TestDecisionValues(t *testing.T) {
	if model.DecisionReject != 0 {
		t.Errorf("DecisionReject = %d, want 0", model.DecisionReject)
	}
	if model.DecisionAllow != 1 {
		t.Errorf("DecisionAllow = %d, want 1", model.DecisionAllow)
	}
}

// BenchmarkAllow measures the performance of Allow() factory function.
func BenchmarkAllow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = model.Allow()
	}
}

// BenchmarkReject measures the performance of Reject() factory function.
func BenchmarkReject(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = model.Reject(1_000_000_000)
	}
}

// BenchmarkReject_Negative measures the performance of Reject() with clamping.
func BenchmarkReject_Negative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = model.Reject(-500_000_000)
	}
}
