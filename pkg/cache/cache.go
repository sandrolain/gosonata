// Package cache provides a thread-safe LRU cache for compiled JSONata expressions.
//
// The cache is used by the GoSonata evaluator when the WithCaching option is enabled.
// It avoids re-parsing and re-compiling the same expression string on every call,
// which is especially valuable when the same query is applied to many different documents.
//
// # Example
//
//	c := cache.New(1024)
//	expr, err := c.GetOrCompile("$.items[price > 100]", compile)
package cache

import (
	"container/list"
	"sync"

	"github.com/sandrolain/gosonata/pkg/types"
)

// entry is a cache entry stored in the doubly-linked list.
type entry struct {
	key  string
	expr *types.Expression
}

// Cache is a thread-safe LRU (Least Recently Used) cache for compiled expressions.
// Once the capacity is reached, the least recently accessed entry is evicted.
//
// Safe for concurrent use by multiple goroutines.
type Cache struct {
	mu       sync.RWMutex
	capacity int
	ll       *list.List
	items    map[string]*list.Element
}

// New creates a new LRU cache with the given capacity.
// capacity must be > 0; if <= 0, a default of 256 is used.
func New(capacity int) *Cache {
	if capacity <= 0 {
		capacity = 256
	}
	return &Cache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

// Get retrieves a compiled expression from the cache.
// Returns (expr, true) if found and moves the entry to front (MRU).
// Returns (nil, false) if not present.
func (c *Cache) Get(key string) (*types.Expression, bool) {
	c.mu.RLock()
	el, ok := c.items[key]
	// OPT-08: if the element is already at the front, skip the write lock entirely.
	alreadyFront := ok && c.ll.Front() == el
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}

	if !alreadyFront {
		// Promote to front under write lock; re-check in case of concurrent eviction.
		c.mu.Lock()
		el, ok = c.items[key]
		if ok {
			c.ll.MoveToFront(el)
		}
		c.mu.Unlock()

		if !ok {
			return nil, false
		}
	}
	return el.Value.(*entry).expr, true
}

// Set inserts or replaces an expression in the cache.
// If at capacity, the least recently used entry is evicted first.
func (c *Cache) Set(key string, expr *types.Expression) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		el.Value.(*entry).expr = expr
		c.ll.MoveToFront(el)
		return
	}

	if c.ll.Len() >= c.capacity {
		c.evictLocked()
	}

	el := c.ll.PushFront(&entry{key: key, expr: expr})
	c.items[key] = el
}

// GetOrCompile retrieves the expression for key from cache, or calls compile()
// to create it, caches the result, and returns it.
// compile is called at most once per key (no negative caching of errors).
func (c *Cache) GetOrCompile(key string, compile func() (*types.Expression, error)) (*types.Expression, error) {
	if expr, ok := c.Get(key); ok {
		return expr, nil
	}
	expr, err := compile()
	if err != nil {
		return nil, err
	}
	c.Set(key, expr)
	return expr, nil
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Capacity returns the maximum number of entries the cache can hold.
func (c *Cache) Capacity() int {
	return c.capacity
}

// Invalidate removes a single entry from the cache.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.Remove(el)
		delete(c.items, key)
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	c.items = make(map[string]*list.Element, c.capacity)
}

// evictLocked removes the least recently used entry.
// Must be called with c.mu held for writing.
func (c *Cache) evictLocked() {
	el := c.ll.Back()
	if el == nil {
		return
	}
	c.ll.Remove(el)
	delete(c.items, el.Value.(*entry).key)
}
