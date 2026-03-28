package cache

import (
	"sync"
	"sync/atomic"

	"github.com/sandrolain/gosonata/pkg/types"
)

// cacheSnapshot is an immutable snapshot of the bounded cache state.
// It is replaced atomically on every write. Readers hold a pointer to a
// snapshot without locking; they never modify the slice or map.
type cacheSnapshot struct {
	keys  []string                     // FIFO insertion order, oldest first
	index map[string]*types.Expression // key → compiled expression
}

// ExprCacheStats holds runtime statistics for an ExprCache.
type ExprCacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Len       int
	Capacity  int
}

// ExprCache is a thread-safe, FIFO-evicting expression cache with lock-free reads.
//
// Reads are fully lock-free: they atomically load the current snapshot pointer
// and look up the key in the immutable index map. No locks are acquired on the
// read path, making it ideal for high-concurrency workloads.
//
// Writes are serialized via a sync.Mutex so that at most one goroutine rebuilds
// the snapshot at a time. The new snapshot is then stored atomically so that
// concurrent readers immediately see the updated state without acquiring any lock.
//
// Eviction policy is FIFO: when the cache is full, the oldest inserted entry is
// removed to make room for the new one.
//
// ExprCache implements the Cacher interface.
type ExprCache struct {
	mu        sync.Mutex
	snap      atomic.Pointer[cacheSnapshot]
	capacity  int
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
}

// New creates a new ExprCache with the given capacity.
// If capacity <= 0, a default of 256 is used.
func New(capacity int) *ExprCache {
	if capacity <= 0 {
		capacity = 256
	}
	b := &ExprCache{capacity: capacity}
	b.snap.Store(&cacheSnapshot{
		keys:  make([]string, 0, capacity),
		index: make(map[string]*types.Expression, capacity),
	})
	return b
}

// Get retrieves a compiled expression from the cache.
// The read path is fully lock-free: it atomically loads the current snapshot
// and performs a map lookup without acquiring any mutex.
// Returns (expr, true) on hit and (nil, false) on miss.
func (b *ExprCache) Get(key string) (*types.Expression, bool) {
	expr, ok := b.snap.Load().index[key]
	if ok {
		b.hits.Add(1)
	} else {
		b.misses.Add(1)
	}
	return expr, ok
}

// Set inserts or replaces an expression in the cache.
// If the key is new and the cache is at capacity, the oldest entry is evicted (FIFO).
// Write is serialized; the new snapshot is published atomically.
func (b *ExprCache) Set(key string, expr *types.Expression) {
	b.mu.Lock()
	defer b.mu.Unlock()

	old := b.snap.Load()

	if _, exists := old.index[key]; exists {
		// Key already present: update value, keep FIFO order unchanged.
		newIndex := make(map[string]*types.Expression, len(old.index))
		for k, v := range old.index {
			newIndex[k] = v
		}
		newIndex[key] = expr

		newKeys := make([]string, len(old.keys))
		copy(newKeys, old.keys)

		b.snap.Store(&cacheSnapshot{keys: newKeys, index: newIndex})
		return
	}

	// New key: append at end, evict at front if over capacity.
	newKeys := make([]string, len(old.keys), len(old.keys)+1)
	copy(newKeys, old.keys)
	newKeys = append(newKeys, key)

	newIndex := make(map[string]*types.Expression, len(old.index)+1)
	for k, v := range old.index {
		newIndex[k] = v
	}
	newIndex[key] = expr

	for len(newKeys) > b.capacity {
		evictKey := newKeys[0]
		newKeys = newKeys[1:]
		delete(newIndex, evictKey)
		b.evictions.Add(1)
	}

	b.snap.Store(&cacheSnapshot{keys: newKeys, index: newIndex})
}

// GetOrCompile retrieves the expression for key from cache, or calls compile()
// to create it, caches the result, and returns it.
// compile is called at most once per key (no negative caching of errors).
func (b *ExprCache) GetOrCompile(key string, compile func() (*types.Expression, error)) (*types.Expression, error) {
	if expr, ok := b.Get(key); ok {
		return expr, nil
	}
	expr, err := compile()
	if err != nil {
		return nil, err
	}
	b.Set(key, expr)
	return expr, nil
}

// Len returns the number of entries currently in the cache.
// Lock-free.
func (b *ExprCache) Len() int {
	return len(b.snap.Load().index)
}

// Capacity returns the maximum number of entries the cache can hold.
func (b *ExprCache) Capacity() int {
	return b.capacity
}

// Invalidate removes a single entry from the cache.
// If the key is not present, Invalidate is a no-op.
func (b *ExprCache) Invalidate(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	old := b.snap.Load()
	if _, exists := old.index[key]; !exists {
		return
	}

	newIndex := make(map[string]*types.Expression, len(old.index)-1)
	for k, v := range old.index {
		if k != key {
			newIndex[k] = v
		}
	}

	newKeys := make([]string, 0, len(old.keys)-1)
	for _, k := range old.keys {
		if k != key {
			newKeys = append(newKeys, k)
		}
	}

	b.snap.Store(&cacheSnapshot{keys: newKeys, index: newIndex})
}

// Clear removes all entries from the cache and resets statistics.
func (b *ExprCache) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snap.Store(&cacheSnapshot{
		keys:  make([]string, 0, b.capacity),
		index: make(map[string]*types.Expression, b.capacity),
	})
}

// Stats returns a snapshot of cache runtime statistics.
// Lock-free.
func (b *ExprCache) Stats() ExprCacheStats {
	return ExprCacheStats{
		Hits:      b.hits.Load(),
		Misses:    b.misses.Load(),
		Evictions: b.evictions.Load(),
		Len:       b.Len(),
		Capacity:  b.capacity,
	}
}

// compile-time interface check
var _ Cacher = (*ExprCache)(nil)
