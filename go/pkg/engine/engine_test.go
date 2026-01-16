package engine_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ratelimiter/go/pkg/clock"
	"github.com/ratelimiter/go/pkg/engine"
	"github.com/ratelimiter/go/pkg/model"
)

// TestNewEngine_Validation verifies constructor validation.
func TestNewEngine_Validation(t *testing.T) {
	clk := clock.NewSystemClock()

	tests := []struct {
		name    string
		config  engine.Config
		maxKeys int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid token bucket config",
			config:  engine.DefaultTokenBucketConfig(),
			maxKeys: 100,
			wantErr: false,
		},
		{
			name:    "maxKeys must be positive",
			config:  engine.DefaultTokenBucketConfig(),
			maxKeys: 0,
			wantErr: true,
			errMsg:  "maxKeys must be > 0",
		},
		{
			name:    "maxKeys cannot be negative",
			config:  engine.DefaultTokenBucketConfig(),
			maxKeys: -1,
			wantErr: true,
			errMsg:  "maxKeys must be > 0",
		},
		{
			name: "invalid config - zero capacity",
			config: engine.Config{
				Algorithm:  engine.AlgorithmTokenBucket,
				Capacity:   0,
				RefillRate: 10.0,
			},
			maxKeys: 100,
			wantErr: true,
			errMsg:  "invalid config",
		},
		{
			name: "invalid config - zero refill rate",
			config: engine.Config{
				Algorithm:  engine.AlgorithmTokenBucket,
				Capacity:   100,
				RefillRate: 0,
			},
			maxKeys: 100,
			wantErr: true,
			errMsg:  "invalid config",
		},
		{
			name: "unknown algorithm",
			config: engine.Config{
				Algorithm: engine.AlgorithmType(999),
			},
			maxKeys: 100,
			wantErr: true,
			errMsg:  "unknown algorithm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := engine.NewEngine(clk, tt.config, tt.maxKeys)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if err.Error() == "" {
					t.Errorf("expected error message, got empty error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if e == nil {
					t.Error("expected non-nil engine")
				}
			}
		})
	}
}

// TestEngine_SingleKey verifies basic single-key rate limiting.
func TestEngine_SingleKey(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   10,
		RefillRate: 1.0, // 1 token/second
	}

	e, err := engine.NewEngine(clk, config, 100)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// First 10 requests should succeed (initial capacity)
	for i := 1; i <= 10; i++ {
		result, err := e.TryAcquire("user:123", 1)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if result.Decision != model.DecisionAllow {
			t.Errorf("request %d: expected ALLOW, got REJECT", i)
		}
	}

	// 11th request should fail (out of tokens)
	result, err := e.TryAcquire("user:123", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != model.DecisionReject {
		t.Error("expected REJECT after capacity exhausted")
	}

	// Advance time by 5 seconds (5 tokens refilled)
	clk.AdvanceNanos(5_000_000_000)

	// Next 5 requests should succeed
	for i := 1; i <= 5; i++ {
		result, err := e.TryAcquire("user:123", 1)
		if err != nil {
			t.Fatalf("request %d after refill: unexpected error: %v", i, err)
		}
		if result.Decision != model.DecisionAllow {
			t.Errorf("request %d after refill: expected ALLOW, got REJECT", i)
		}
	}

	// Next request should fail
	result, err = e.TryAcquire("user:123", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != model.DecisionReject {
		t.Error("expected REJECT after refilled tokens exhausted")
	}
}

// TestEngine_MultipleKeys verifies independent rate limiting for different keys.
func TestEngine_MultipleKeys(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   5,
		RefillRate: 1.0,
	}

	e, err := engine.NewEngine(clk, config, 100)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Exhaust user:123's tokens
	for i := 0; i < 5; i++ {
		e.TryAcquire("user:123", 1)
	}

	// user:123 should be rejected
	result, _ := e.TryAcquire("user:123", 1)
	if result.Decision != model.DecisionReject {
		t.Error("user:123 should be rejected after exhausting tokens")
	}

	// user:456 should still be allowed (different limiter)
	result, _ = e.TryAcquire("user:456", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("user:456 should be allowed (independent limiter)")
	}

	// user:789 should also be allowed
	result, _ = e.TryAcquire("user:789", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("user:789 should be allowed (independent limiter)")
	}

	// Verify cache has 3 entries
	if e.Len() != 3 {
		t.Errorf("expected 3 cached limiters, got %d", e.Len())
	}
}

// TestEngine_LRUEviction verifies LRU eviction when maxKeys is exceeded.
func TestEngine_LRUEviction(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.DefaultTokenBucketConfig()

	// Small cache: only 3 keys
	e, err := engine.NewEngine(clk, config, 3)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Add 3 keys
	e.TryAcquire("key1", 1)
	e.TryAcquire("key2", 1)
	e.TryAcquire("key3", 1)

	if e.Len() != 3 {
		t.Errorf("expected 3 cached keys, got %d", e.Len())
	}

	// Add 4th key - should evict key1 (oldest)
	e.TryAcquire("key4", 1)

	if e.Len() != 3 {
		t.Errorf("expected 3 cached keys after eviction, got %d", e.Len())
	}

	// Access key2 (moves it to front)
	e.TryAcquire("key2", 1)

	// Add key5 - should evict key3 (now oldest, since key2 was accessed)
	e.TryAcquire("key5", 1)

	// key2, key4, key5 should be cached
	// key1 and key3 were evicted
	if e.Len() != 3 {
		t.Errorf("expected 3 cached keys, got %d", e.Len())
	}
}

// TestEngine_LRUEviction_StateReset verifies that evicted keys lose their state.
func TestEngine_LRUEviction_StateReset(t *testing.T) {
	clk := clock.NewManualClock(0)
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   5,
		RefillRate: 1.0,
	}

	// Cache only 2 keys
	e, err := engine.NewEngine(clk, config, 2)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Exhaust key1's tokens
	for i := 0; i < 5; i++ {
		e.TryAcquire("key1", 1)
	}

	// key1 should be rejected
	result, _ := e.TryAcquire("key1", 1)
	if result.Decision != model.DecisionReject {
		t.Error("key1 should be rejected after exhausting tokens")
	}

	// Add key2 and key3 (key1 will be evicted)
	e.TryAcquire("key2", 1)
	e.TryAcquire("key3", 1)

	// Access key1 again (creates fresh limiter)
	result, _ = e.TryAcquire("key1", 1)
	if result.Decision != model.DecisionAllow {
		t.Error("key1 should be allowed (fresh limiter after eviction)")
	}
}

// TestEngine_EmptyKey verifies that empty keys are rejected.
func TestEngine_EmptyKey(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()

	e, err := engine.NewEngine(clk, config, 100)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, err = e.TryAcquire("", 1)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

// TestEngine_AlgorithmTypes verifies all algorithm types work correctly.
func TestEngine_AlgorithmTypes(t *testing.T) {
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
			e, err := engine.NewEngine(clk, tt.config, 100)
			if err != nil {
				t.Fatalf("failed to create engine: %v", err)
			}

			// Should be able to acquire permits
			result, err := e.TryAcquire("user:123", 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Decision != model.DecisionAllow {
				t.Error("expected first request to be allowed")
			}
		})
	}
}

// TestEngine_Concurrent verifies thread-safety with concurrent access.
func TestEngine_Concurrent(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   1000,
		RefillRate: 100.0,
	}

	e, err := engine.NewEngine(clk, config, 1000)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	const numGoroutines = 20
	const opsPerGoroutine = 50

	var wg sync.WaitGroup

	// Concurrent requests to same key
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				e.TryAcquire("shared-key", 1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent same-key test completed")
}

// TestEngine_ConcurrentMultiKey verifies parallel execution for different keys.
func TestEngine_ConcurrentMultiKey(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   100,
		RefillRate: 10.0,
	}

	e, err := engine.NewEngine(clk, config, 1000)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	const numKeys = 50
	const opsPerKey = 20

	var wg sync.WaitGroup

	// Each goroutine accesses a different key
	wg.Add(numKeys)
	for i := 0; i < numKeys; i++ {
		go func(keyID int) {
			defer wg.Done()
			key := fmt.Sprintf("user:%d", keyID)
			for j := 0; j < opsPerKey; j++ {
				e.TryAcquire(key, 1)
			}
		}(i)
	}

	wg.Wait()

	// Should have created limiters for all keys
	if e.Len() != numKeys {
		t.Errorf("expected %d cached keys, got %d", numKeys, e.Len())
	}

	t.Logf("Concurrent multi-key test completed with %d keys", numKeys)
}

// TestEngine_ConcurrentGetOrCreate verifies atomic get-or-create behavior.
func TestEngine_ConcurrentGetOrCreate(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()

	e, err := engine.NewEngine(clk, config, 100)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	const numGoroutines = 50
	const sharedKey = "shared-key"

	var wg sync.WaitGroup

	// All goroutines try to access the same key simultaneously
	// Only one limiter should be created
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			e.TryAcquire(sharedKey, 1)
		}()
	}

	wg.Wait()

	// Should only have 1 entry in cache
	if e.Len() != 1 {
		t.Errorf("expected 1 cached key, got %d (race in get-or-create)", e.Len())
	}
}

// TestEngine_Len verifies cache size tracking.
func TestEngine_Len(t *testing.T) {
	clk := clock.NewSystemClock()
	config := engine.DefaultTokenBucketConfig()

	e, err := engine.NewEngine(clk, config, 100)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	if e.Len() != 0 {
		t.Errorf("expected empty cache, got size %d", e.Len())
	}

	e.TryAcquire("key1", 1)
	if e.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", e.Len())
	}

	e.TryAcquire("key2", 1)
	e.TryAcquire("key3", 1)
	if e.Len() != 3 {
		t.Errorf("expected 3 entries, got %d", e.Len())
	}

	// Accessing existing key doesn't change size
	e.TryAcquire("key1", 1)
	if e.Len() != 3 {
		t.Errorf("expected 3 entries after re-access, got %d", e.Len())
	}
}

// BenchmarkEngine_SingleKey measures single-key throughput.
func BenchmarkEngine_SingleKey(b *testing.B) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   1_000_000,
		RefillRate: 1_000_000.0,
	}

	e, _ := engine.NewEngine(clk, config, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.TryAcquire("user:123", 1)
	}
}

// BenchmarkEngine_MultiKey measures multi-key throughput.
func BenchmarkEngine_MultiKey(b *testing.B) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   1_000_000,
		RefillRate: 1_000_000.0,
	}

	e, _ := engine.NewEngine(clk, config, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("user:%d", i%1000)
		e.TryAcquire(key, 1)
	}
}

// BenchmarkEngine_Parallel measures parallel throughput.
func BenchmarkEngine_Parallel(b *testing.B) {
	clk := clock.NewSystemClock()
	config := engine.Config{
		Algorithm:  engine.AlgorithmTokenBucket,
		Capacity:   1_000_000,
		RefillRate: 1_000_000.0,
	}

	e, _ := engine.NewEngine(clk, config, 10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("user:%d", i%100)
			e.TryAcquire(key, 1)
			i++
		}
	})
}
