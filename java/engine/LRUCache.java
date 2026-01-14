package rl.java.engine;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.function.BiConsumer;

/**
 * Custom LRU (Least Recently Used) cache with eviction callback.
 *
 * This implementation provides:
 * - O(1) get/put/remove operations
 * - Access-order based eviction (least recently accessed entries evicted first)
 * - Thread-safe operations via synchronized methods
 * - Configurable max size
 * - Eviction callback for cleanup (e.g., releasing locks)
 *
 * Design:
 * - Uses LinkedHashMap with accessOrder=true for LRU ordering
 * - All operations are synchronized for thread-safety
 * - Eviction happens automatically when size exceeds maxSize
 *
 * @param <K> Key type
 * @param <V> Value type
 */
public final class LRUCache<K, V> {

    private final int maxSize;
    private final BiConsumer<K, V> evictionCallback;
    private final LinkedHashMap<K, V> map;

    /**
     * Creates an LRU cache with specified max size and eviction callback.
     *
     * @param maxSize Maximum number of entries (must be > 0)
     * @param evictionCallback Callback invoked when an entry is evicted (can be null)
     * @throws IllegalArgumentException if maxSize <= 0
     */
    public LRUCache(int maxSize, BiConsumer<K, V> evictionCallback) {
        if (maxSize <= 0) {
            throw new IllegalArgumentException("maxSize must be > 0");
        }

        this.maxSize = maxSize;
        this.evictionCallback = evictionCallback;

        // LinkedHashMap with accessOrder=true for LRU behavior
        // Initial capacity = maxSize, load factor = 0.75 (default)
        this.map = new LinkedHashMap<>(maxSize, 0.75f, true) {
            @Override
            protected boolean removeEldestEntry(Map.Entry<K, V> eldest) {
                boolean shouldRemove = size() > LRUCache.this.maxSize;
                if (shouldRemove && evictionCallback != null) {
                    // Invoke eviction callback before removal
                    evictionCallback.accept(eldest.getKey(), eldest.getValue());
                }
                return shouldRemove;
            }
        };
    }

    /**
     * Creates an LRU cache without eviction callback.
     *
     * @param maxSize Maximum number of entries
     */
    public LRUCache(int maxSize) {
        this(maxSize, null);
    }

    /**
     * Retrieves a value from the cache.
     * This marks the entry as recently used.
     *
     * @param key The key to look up
     * @return The value, or null if not present
     */
    public synchronized V get(K key) {
        return map.get(key);
    }

    /**
     * Inserts or updates a value in the cache.
     * This marks the entry as recently used.
     * May trigger eviction if size exceeds maxSize.
     *
     * @param key The key
     * @param value The value
     * @return The previous value, or null if none
     */
    public synchronized V put(K key, V value) {
        return map.put(key, value);
    }

    /**
     * Removes an entry from the cache.
     * Note: Does NOT invoke eviction callback (explicit removal, not eviction).
     *
     * @param key The key to remove
     * @return The removed value, or null if not present
     */
    public synchronized V remove(K key) {
        return map.remove(key);
    }

    /**
     * Checks if a key is present in the cache.
     * This does NOT mark the entry as recently used.
     *
     * @param key The key to check
     * @return true if present, false otherwise
     */
    public synchronized boolean containsKey(K key) {
        return map.containsKey(key);
    }

    /**
     * Returns the current size of the cache.
     *
     * @return Number of entries
     */
    public synchronized int size() {
        return map.size();
    }

    /**
     * Inserts a value only if the key is not already present.
     * This is an atomic operation that prevents race conditions.
     *
     * @param key The key
     * @param value The value
     * @return The existing value if present, or null if the new value was inserted
     */
    public synchronized V putIfAbsent(K key, V value) {
        V existing = map.get(key);
        if (existing != null) {
            return existing;  // Key already exists, return existing value
        }
        map.put(key, value);  // Key doesn't exist, insert new value
        return null;
    }

    /**
     * Clears all entries from the cache.
     * Note: Does NOT invoke eviction callbacks.
     */
    public synchronized void clear() {
        map.clear();
    }

    /**
     * Returns the maximum size of the cache.
     *
     * @return Maximum number of entries
     */
    public int maxSize() {
        return maxSize;
    }
}