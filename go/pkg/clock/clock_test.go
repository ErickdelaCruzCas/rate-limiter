package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/ratelimiter/go/pkg/clock"
)

// TestSystemClock_Monotonicity verifies that SystemClock always returns
// monotonically increasing values (time never goes backward).
func TestSystemClock_Monotonicity(t *testing.T) {
	clk := clock.NewSystemClock()

	prev := clk.NowNanos()
	for i := 0; i < 1000; i++ {
		now := clk.NowNanos()
		if now < prev {
			t.Errorf("time went backward: prev=%d, now=%d", prev, now)
		}
		prev = now
	}
}

// TestSystemClock_ProgressesWithTime verifies that SystemClock actually
// tracks real wall-clock time progression.
func TestSystemClock_ProgressesWithTime(t *testing.T) {
	clk := clock.NewSystemClock()

	start := clk.NowNanos()
	time.Sleep(10 * time.Millisecond)
	end := clk.NowNanos()

	elapsed := end - start
	expected := int64(10 * time.Millisecond)

	// Allow for timing variance (5ms to 50ms is acceptable)
	if elapsed < expected/2 || elapsed > expected*5 {
		t.Errorf("unexpected elapsed time: got %d ns, expected ~%d ns", elapsed, expected)
	}
}

// TestManualClock_InitialValue verifies that ManualClock starts at the specified value.
func TestManualClock_InitialValue(t *testing.T) {
	tests := []struct {
		name      string
		startTime int64
	}{
		{"zero", 0},
		{"positive", 1_000_000_000},
		{"large", 9_999_999_999_999},
		{"negative", -1_000_000_000}, // Negative values are allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk := clock.NewManualClock(tt.startTime)
			if got := clk.NowNanos(); got != tt.startTime {
				t.Errorf("initial time mismatch: got %d, want %d", got, tt.startTime)
			}
		})
	}
}

// TestManualClock_AdvanceNanos verifies that AdvanceNanos correctly increments time.
func TestManualClock_AdvanceNanos(t *testing.T) {
	clk := clock.NewManualClock(0)

	// Advance by various durations
	advances := []int64{
		1_000_000_000,     // 1 second
		500_000_000,       // 0.5 seconds
		2_500_000_000,     // 2.5 seconds
		100,               // 100 nanoseconds
		0,                 // 0 (should be no-op)
	}

	expected := int64(0)
	for _, delta := range advances {
		err := clk.AdvanceNanos(delta)
		if err != nil {
			t.Fatalf("AdvanceNanos(%d) returned error: %v", delta, err)
		}
		expected += delta
		if got := clk.NowNanos(); got != expected {
			t.Errorf("after advancing %d: got %d, want %d", delta, got, expected)
		}
	}

	// Verify total advancement
	totalExpected := int64(4_000_000_100)
	if got := clk.NowNanos(); got != totalExpected {
		t.Errorf("total time mismatch: got %d, want %d", got, totalExpected)
	}
}

// TestManualClock_AdvanceNanos_NegativeDelta verifies that negative deltas are rejected.
func TestManualClock_AdvanceNanos_NegativeDelta(t *testing.T) {
	clk := clock.NewManualClock(1_000_000_000)

	err := clk.AdvanceNanos(-500_000_000)
	if err == nil {
		t.Error("expected error for negative delta, got nil")
	}

	// Time should not have changed
	if got := clk.NowNanos(); got != 1_000_000_000 {
		t.Errorf("time changed after failed advance: got %d, want %d", got, 1_000_000_000)
	}
}

// TestManualClock_SetNanos verifies that SetNanos can jump to arbitrary times.
func TestManualClock_SetNanos(t *testing.T) {
	clk := clock.NewManualClock(0)

	// Jump to various times (including backward)
	times := []int64{
		1_000_000_000,
		5_000_000_000,
		2_000_000_000, // Backward jump (non-monotonic but allowed)
		0,             // Back to zero
		-1_000_000_000, // Negative time
	}

	for _, targetTime := range times {
		clk.SetNanos(targetTime)
		if got := clk.NowNanos(); got != targetTime {
			t.Errorf("SetNanos(%d): got %d, want %d", targetTime, got, targetTime)
		}
	}
}

// TestManualClock_Concurrent verifies that ManualClock is thread-safe.
//
// This test spawns multiple goroutines that concurrently read and advance time,
// ensuring that no race conditions occur and all operations complete successfully.
func TestManualClock_Concurrent(t *testing.T) {
	clk := clock.NewManualClock(0)

	const numGoroutines = 10
	const advancesPerGoroutine = 100
	const advanceAmount = 1_000_000 // 1 millisecond

	var wg sync.WaitGroup

	// Spawn goroutines that advance time
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < advancesPerGoroutine; j++ {
				err := clk.AdvanceNanos(advanceAmount)
				if err != nil {
					t.Errorf("AdvanceNanos failed: %v", err)
				}
			}
		}()
	}

	// Spawn goroutines that read time
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < advancesPerGoroutine; j++ {
				_ = clk.NowNanos() // Just read, no validation needed
			}
		}()
	}

	wg.Wait()

	// Verify final time (all advances should have been applied)
	expected := int64(numGoroutines * advancesPerGoroutine * advanceAmount)
	if got := clk.NowNanos(); got != expected {
		t.Errorf("final time mismatch: got %d, want %d", got, expected)
	}
}

// TestManualClock_SetNanos_Concurrent verifies SetNanos thread-safety.
func TestManualClock_SetNanos_Concurrent(t *testing.T) {
	clk := clock.NewManualClock(0)

	const numGoroutines = 20
	const setsPerGoroutine = 50

	var wg sync.WaitGroup

	// Spawn goroutines that set and read time concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < setsPerGoroutine; j++ {
				// Set to a unique value based on goroutine ID and iteration
				targetTime := int64(id*setsPerGoroutine + j)
				clk.SetNanos(targetTime)

				// Read immediately after (may not match due to concurrent sets)
				_ = clk.NowNanos()
			}
		}(i)
	}

	wg.Wait()

	// Just verify that we didn't crash (final value is non-deterministic due to concurrency)
	finalTime := clk.NowNanos()
	t.Logf("Final time after concurrent SetNanos: %d", finalTime)
}

// TestManualClock_MixedOperations tests a realistic scenario with
// interleaved advances, sets, and reads.
func TestManualClock_MixedOperations(t *testing.T) {
	clk := clock.NewManualClock(0)

	// Start at 0
	if got := clk.NowNanos(); got != 0 {
		t.Errorf("initial: got %d, want 0", got)
	}

	// Advance 1 second
	clk.AdvanceNanos(1_000_000_000)
	if got := clk.NowNanos(); got != 1_000_000_000 {
		t.Errorf("after advance: got %d, want 1000000000", got)
	}

	// Jump to 5 seconds
	clk.SetNanos(5_000_000_000)
	if got := clk.NowNanos(); got != 5_000_000_000 {
		t.Errorf("after set: got %d, want 5000000000", got)
	}

	// Advance another 2 seconds
	clk.AdvanceNanos(2_000_000_000)
	if got := clk.NowNanos(); got != 7_000_000_000 {
		t.Errorf("after second advance: got %d, want 7000000000", got)
	}

	// Jump backward to 3 seconds
	clk.SetNanos(3_000_000_000)
	if got := clk.NowNanos(); got != 3_000_000_000 {
		t.Errorf("after backward set: got %d, want 3000000000", got)
	}
}

// BenchmarkSystemClock measures the performance of SystemClock.NowNanos().
func BenchmarkSystemClock_NowNanos(b *testing.B) {
	clk := clock.NewSystemClock()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clk.NowNanos()
	}
}

// BenchmarkManualClock_NowNanos measures the performance of ManualClock.NowNanos().
func BenchmarkManualClock_NowNanos(b *testing.B) {
	clk := clock.NewManualClock(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clk.NowNanos()
	}
}

// BenchmarkManualClock_AdvanceNanos measures the performance of ManualClock.AdvanceNanos().
func BenchmarkManualClock_AdvanceNanos(b *testing.B) {
	clk := clock.NewManualClock(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clk.AdvanceNanos(1_000_000)
	}
}

// BenchmarkManualClock_SetNanos measures the performance of ManualClock.SetNanos().
func BenchmarkManualClock_SetNanos(b *testing.B) {
	clk := clock.NewManualClock(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clk.SetNanos(int64(i))
	}
}

// BenchmarkManualClock_Parallel measures concurrent performance of ManualClock operations.
func BenchmarkManualClock_Parallel(b *testing.B) {
	clk := clock.NewManualClock(0)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			// Mix of reads and advances
			if i%2 == 0 {
				_ = clk.NowNanos()
			} else {
				_ = clk.AdvanceNanos(1_000)
			}
			i++
		}
	})
}
