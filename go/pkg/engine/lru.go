package engine

import (
	"container/list"
	"sync"
)

// LRUCache is a thread-safe Least Recently Used (LRU) cache implementation.
//
// The cache maintains a fixed maximum size and evicts the least recently used
// entries when the capacity is exceeded. This is essential for the engine layer
// to prevent unbounded memory growth when rate limiting many different keys.
//
// Implementation details:
// - Uses a doubly-linked list (container/list) for LRU ordering
// - Uses a map for O(1) key lookups
// - All operations are protected by a single mutex
//
// Thread-safe: All methods are safe for concurrent use by multiple goroutines.
//
// Time complexity:
// - Get: O(1)
// - Put/PutIfAbsent: O(1)
// - Eviction: O(1) amortized
//
// Example usage:
//
//	cache := NewLRUCache(1000) // Max 1000 entries
//
//	// Put value
//	cache.Put("user:123", rateLimiterInstance)
//
//	// Get value (moves to front, marking as recently used)
//	if val, ok := cache.Get("user:123"); ok {
//	    limiter := val.(*RateLimiter)
//	    // Use limiter...
//	}
//
//	// Atomic get-or-create pattern
//	existing := cache.PutIfAbsent("user:456", newLimiterInstance)
//	if existing != nil {
//	    // Key already existed, use existing value
//	    limiter := existing.(*RateLimiter)
//	} else {
//	    // Our value was inserted
//	}
type LRUCache struct {
	mu        sync.Mutex
	maxSize   int
	items     map[string]*list.Element
	evictList *list.List
}

// entry is the value stored in the linked list.
// It contains both the key (for eviction) and the cached value.
type entry struct {
	key   string
	value interface{}
}

// NewLRUCache creates a new LRU cache with the specified maximum size.
//
// Parameters:
//   - maxSize: Maximum number of entries to cache (must be > 0)
//
// Returns:
//   - *LRUCache: Initialized empty cache
//
// The cache will automatically evict the least recently used entry when
// maxSize is exceeded. "Recently used" is determined by Get() and Put() calls.
//
// Example:
//
//	cache := NewLRUCache(10000) // Cache up to 10,000 rate limiters
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize:   maxSize,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache and marks it as recently used.
//
// If the key exists, it is moved to the front of the LRU list (most recently used).
// If the key does not exist, returns (nil, false).
//
// Parameters:
//   - key: The cache key to look up
//
// Returns:
//   - value: The cached value (nil if not found)
//   - ok: true if key exists, false otherwise
//
// Thread-safe: Can be called concurrently from multiple goroutines.
//
// Example:
//
//	if limiter, ok := cache.Get("user:123"); ok {
//	    // Key found, limiter is marked as recently used
//	    result := limiter.(*RateLimiter).TryAcquire(...)
//	} else {
//	    // Key not found, create new limiter
//	}
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Move to front (most recently used)
	c.evictList.MoveToFront(elem)

	return elem.Value.(*entry).value, true
}

// Put adds or updates a value in the cache and marks it as recently used.
//
// If the key already exists, its value is updated and it's moved to the front.
// If the key is new and the cache is at capacity, the least recently used
// entry is evicted before inserting the new entry.
//
// Parameters:
//   - key: The cache key
//   - value: The value to cache (typically a rate limiter instance)
//
// Thread-safe: Can be called concurrently from multiple goroutines.
//
// Example:
//
//	limiter := tokenbucket.New(clock, 100, 10.0)
//	cache.Put("user:123", limiter)
func (c *LRUCache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing entry and move to front
		c.evictList.MoveToFront(elem)
		elem.Value.(*entry).value = value
		return
	}

	// Add new entry
	ent := &entry{key: key, value: value}
	elem := c.evictList.PushFront(ent)
	c.items[key] = elem

	// Evict oldest if over capacity
	if c.evictList.Len() > c.maxSize {
		c.removeOldest()
	}
}

// PutIfAbsent adds a value to the cache only if the key doesn't already exist.
//
// This is an atomic "check and insert" operation useful for implementing
// get-or-create patterns without race conditions.
//
// If the key already exists:
//   - The existing value is returned
//   - The provided value is NOT inserted
//   - The existing entry is moved to the front (marked as recently used)
//
// If the key does not exist:
//   - The provided value is inserted
//   - nil is returned
//   - If at capacity, the least recently used entry is evicted
//
// Parameters:
//   - key: The cache key
//   - value: The value to cache if key doesn't exist
//
// Returns:
//   - interface{}: The existing value if key existed, nil if value was inserted
//
// Thread-safe: Can be called concurrently from multiple goroutines.
//
// Example (get-or-create pattern):
//
//	newLimiter := tokenbucket.New(clock, 100, 10.0)
//	existing := cache.PutIfAbsent("user:123", newLimiter)
//	if existing != nil {
//	    // Key already existed, use the existing limiter
//	    limiter := existing.(*RateLimiter)
//	} else {
//	    // Our value was inserted, use newLimiter
//	    limiter := newLimiter
//	}
func (c *LRUCache) PutIfAbsent(key string, value interface{}) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Key exists: move to front and return existing value
		c.evictList.MoveToFront(elem)
		return elem.Value.(*entry).value
	}

	// Key doesn't exist: insert new entry
	ent := &entry{key: key, value: value}
	elem := c.evictList.PushFront(ent)
	c.items[key] = elem

	// Evict oldest if over capacity
	if c.evictList.Len() > c.maxSize {
		c.removeOldest()
	}

	return nil
}

// Len returns the current number of entries in the cache.
//
// Thread-safe: Can be called concurrently from multiple goroutines.
//
// Example:
//
//	size := cache.Len()
//	fmt.Printf("Cache contains %d entries\n", size)
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictList.Len()
}

// removeOldest removes the least recently used entry from the cache.
//
// This method must be called with c.mu held (enforced by caller).
//
// The LRU entry is at the back of the evictList. This method:
// 1. Removes the element from the back of the list
// 2. Deletes the corresponding entry from the map
func (c *LRUCache) removeOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.evictList.Remove(elem)
		kv := elem.Value.(*entry)
		delete(c.items, kv.key)
	}
}
