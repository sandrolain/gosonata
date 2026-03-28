// Package cache provides a thread-safe expression cache for compiled JSONata expressions.
//
// # Example
//
//	c := cache.New(1024)
//	expr, err := c.GetOrCompile("$.items[price > 100]", compile)
package cache

import "github.com/sandrolain/gosonata/pkg/types"

// Cacher is the interface satisfied by all expression cache implementations.
// It is the type used by the evaluator to decouple from a specific cache strategy.
type Cacher interface {
	// Get retrieves a compiled expression. Returns (nil, false) on miss.
	Get(key string) (*types.Expression, bool)
	// Set inserts or replaces an expression.
	Set(key string, expr *types.Expression)
	// GetOrCompile returns the cached expression for key, or calls compile() once
	// and caches the result. Errors from compile() are not cached.
	GetOrCompile(key string, compile func() (*types.Expression, error)) (*types.Expression, error)
	// Len returns the number of entries currently in the cache.
	Len() int
	// Capacity returns the maximum number of entries the cache can hold.
	Capacity() int
	// Invalidate removes a single entry from the cache.
	Invalidate(key string)
	// Clear removes all entries from the cache.
	Clear()
}
