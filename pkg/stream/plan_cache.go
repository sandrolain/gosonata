package stream

import (
	"sync"
	"sync/atomic"
)

// evalPlanSnap is an immutable snapshot of the plan cache.
// Readers hold a pointer to a snapshot without locking.
type evalPlanSnap struct {
	keys  []string
	index map[string]*EvalPlan
}

// evalPlanCache is a FIFO-evicting cache for [EvalPlan] values.
//
// Reads are fully lock-free: a single atomic.Pointer load is followed by a
// map lookup on the immutable snapshot. Writes are serialised by a mutex and
// publish a new snapshot atomically.
type evalPlanCache struct {
	mu        sync.Mutex
	snap      atomic.Pointer[evalPlanSnap]
	capacity  int
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
}

// newEvalPlanCache creates a plan cache with the given capacity (default 256).
func newEvalPlanCache(capacity int) *evalPlanCache {
	if capacity <= 0 {
		capacity = 256
	}
	c := &evalPlanCache{capacity: capacity}
	c.snap.Store(&evalPlanSnap{
		keys:  make([]string, 0, capacity),
		index: make(map[string]*EvalPlan, capacity),
	})
	return c
}

// get retrieves an EvalPlan by key. Lock-free.
func (c *evalPlanCache) get(key string) (*EvalPlan, bool) {
	plan, ok := c.snap.Load().index[key]
	if ok {
		c.hits.Add(1)
	} else {
		c.misses.Add(1)
	}
	return plan, ok
}

// set stores an EvalPlan. If the cache is full, the oldest entry is evicted.
func (c *evalPlanCache) set(key string, plan *EvalPlan) (evicted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	old := c.snap.Load()

	if _, exists := old.index[key]; exists {
		// Update in-place, keep FIFO order.
		newIndex := make(map[string]*EvalPlan, len(old.index))
		for k, v := range old.index {
			newIndex[k] = v
		}
		newIndex[key] = plan
		newKeys := make([]string, len(old.keys))
		copy(newKeys, old.keys)
		c.snap.Store(&evalPlanSnap{keys: newKeys, index: newIndex})
		return false
	}

	newKeys := make([]string, len(old.keys), len(old.keys)+1)
	copy(newKeys, old.keys)
	newKeys = append(newKeys, key)

	newIndex := make(map[string]*EvalPlan, len(old.index)+1)
	for k, v := range old.index {
		newIndex[k] = v
	}
	newIndex[key] = plan

	didEvict := false
	for len(newKeys) > c.capacity {
		evictKey := newKeys[0]
		newKeys = newKeys[1:]
		delete(newIndex, evictKey)
		c.evictions.Add(1)
		didEvict = true
	}

	c.snap.Store(&evalPlanSnap{keys: newKeys, index: newIndex})
	return didEvict
}

// invalidate removes a single entry. No-op if not present.
func (c *evalPlanCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	old := c.snap.Load()
	if _, exists := old.index[key]; !exists {
		return
	}

	newIndex := make(map[string]*EvalPlan, len(old.index)-1)
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
	c.snap.Store(&evalPlanSnap{keys: newKeys, index: newIndex})
}

// clear removes all entries.
func (c *evalPlanCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.Store(&evalPlanSnap{
		keys:  make([]string, 0, c.capacity),
		index: make(map[string]*EvalPlan, c.capacity),
	})
}

// len returns the current number of entries. Lock-free.
func (c *evalPlanCache) len() int { return len(c.snap.Load().index) }

// stats returns a snapshot of cache statistics.
func (c *evalPlanCache) stats() PlanStats {
	return PlanStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Len:       c.len(),
		Capacity:  c.capacity,
	}
}

// PlanStats holds runtime statistics for the evaluation plan cache.
type PlanStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Len       int
	Capacity  int
}
