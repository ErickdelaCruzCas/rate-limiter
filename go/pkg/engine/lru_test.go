package engine_test

import (
	"sync"
	"testing"

	"github.com/ratelimiter/go/pkg/engine"
)

// TestLRU_PutAndGet verifies basic put/get operations.
func TestLRU_PutAndGet(t *testing.T) {
	cache := engine.NewLRUCache(10)

	// Put value
	cache.Put("key1", "value1")

	// Get value
	if val, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist")
	} else if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	// Get non-existent key
	if _, ok := cache.Get("key2"); ok {
		t.Error("expected key2 to not exist")
	}
}

// TestLRU_Update verifies that Put updates existing keys.
func TestLRU_Update(t *testing.T) {
	cache := engine.NewLRUCache(10)

	cache.Put("key1", "value1")
	cache.Put("key1", "value2") // Update

	if val, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist")
	} else if val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}
}

// TestLRU_Eviction verifies LRU eviction when capacity is exceeded.
func TestLRU_Eviction(t *testing.T) {
	cache := engine.NewLRUCache(3) // Max 3 entries

	// Fill cache
	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	cache.Put("key3", "value3")

	// Add 4th entry - should evict key1 (oldest)
	cache.Put("key4", "value4")

	// key1 should be evicted
	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be evicted")
	}

	// Others should exist
	if _, ok := cache.Get("key2"); !ok {
		t.Error("expected key2 to exist")
	}
	if _, ok := cache.Get("key3"); !ok {
		t.Error("expected key3 to exist")
	}
	if _, ok := cache.Get("key4"); !ok {
		t.Error("expected key4 to exist")
	}
}

// TestLRU_EvictionWithAccess verifies LRU behavior with access patterns.
func TestLRU_EvictionWithAccess(t *testing.T) {
	cache := engine.NewLRUCache(3)

	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	cache.Put("key3", "value3")

	// Access key1 (moves it to front)
	cache.Get("key1")

	// Add key4 - should evict key2 (now oldest after key1 was accessed)
	cache.Put("key4", "value4")

	// key2 should be evicted
	if _, ok := cache.Get("key2"); ok {
		t.Error("expected key2 to be evicted")
	}

	// key1 should still exist (was accessed recently)
	if _, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist (recently accessed)")
	}
}

// TestLRU_PutIfAbsent_NewKey verifies PutIfAbsent for new keys.
func TestLRU_PutIfAbsent_NewKey(t *testing.T) {
	cache := engine.NewLRUCache(10)

	// Insert new key
	existing := cache.PutIfAbsent("key1", "value1")
	if existing != nil {
		t.Errorf("expected nil for new key, got %v", existing)
	}

	// Verify it was inserted
	if val, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist")
	} else if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

// TestLRU_PutIfAbsent_ExistingKey verifies PutIfAbsent for existing keys.
func TestLRU_PutIfAbsent_ExistingKey(t *testing.T) {
	cache := engine.NewLRUCache(10)

	// Insert initial value
	cache.Put("key1", "value1")

	// Try to insert different value
	existing := cache.PutIfAbsent("key1", "value2")
	if existing == nil {
		t.Error("expected existing value, got nil")
	} else if existing != "value1" {
		t.Errorf("expected value1, got %v", existing)
	}

	// Verify original value unchanged
	if val, ok := cache.Get("key1"); !ok {
		t.Error("expected key1 to exist")
	} else if val != "value1" {
		t.Errorf("expected value1 (unchanged), got %v", val)
	}
}

// TestLRU_Len verifies length tracking.
func TestLRU_Len(t *testing.T) {
	cache := engine.NewLRUCache(10)

	if cache.Len() != 0 {
		t.Errorf("expected len=0, got %d", cache.Len())
	}

	cache.Put("key1", "value1")
	if cache.Len() != 1 {
		t.Errorf("expected len=1, got %d", cache.Len())
	}

	cache.Put("key2", "value2")
	if cache.Len() != 2 {
		t.Errorf("expected len=2, got %d", cache.Len())
	}

	// Update doesn't change length
	cache.Put("key1", "newvalue")
	if cache.Len() != 2 {
		t.Errorf("expected len=2 after update, got %d", cache.Len())
	}
}

// TestLRU_Concurrent verifies thread-safety under concurrent access.
func TestLRU_Concurrent(t *testing.T) {
	cache := engine.NewLRUCache(1000)

	const numGoroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup

	// Concurrent puts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := string(rune('a' + (id*opsPerGoroutine+j)%26))
				cache.Put(key, id*opsPerGoroutine+j)
			}
		}(i)
	}

	// Concurrent gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := string(rune('a' + (id*opsPerGoroutine+j)%26))
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Cache size after concurrent ops: %d", cache.Len())
}

// TestLRU_PutIfAbsent_Concurrent verifies PutIfAbsent is atomic.
func TestLRU_PutIfAbsent_Concurrent(t *testing.T) {
	cache := engine.NewLRUCache(100)

	const numGoroutines = 20
	const key = "shared-key"

	var wg sync.WaitGroup
	insertCount := 0
	var mu sync.Mutex

	// All goroutines try to insert the same key
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			existing := cache.PutIfAbsent(key, id)
			if existing == nil {
				// We won the race
				mu.Lock()
				insertCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Exactly one goroutine should have won
	if insertCount != 1 {
		t.Errorf("expected exactly 1 insert, got %d", insertCount)
	}

	// Key should exist
	if _, ok := cache.Get(key); !ok {
		t.Error("expected key to exist")
	}
}

// BenchmarkLRU_Put measures Put performance.
func BenchmarkLRU_Put(b *testing.B) {
	cache := engine.NewLRUCache(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(string(rune('a'+i%26)), i)
	}
}

// BenchmarkLRU_Get measures Get performance.
func BenchmarkLRU_Get(b *testing.B) {
	cache := engine.NewLRUCache(10000)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Put(string(rune('a'+i%26)), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(string(rune('a' + i%26)))
	}
}

// BenchmarkLRU_PutIfAbsent measures PutIfAbsent performance.
func BenchmarkLRU_PutIfAbsent(b *testing.B) {
	cache := engine.NewLRUCache(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.PutIfAbsent(string(rune('a'+i%26)), i)
	}
}
