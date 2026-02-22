package unit_test

import (
	"testing"

	"github.com/sandrolain/gosonata/pkg/cache"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

func TestCacheNew(t *testing.T) {
	c := cache.New(10)
	if got := c.Len(); got != 0 {
		t.Fatalf("expected empty cache, got %d", got)
	}
	if got := c.Capacity(); got != 10 {
		t.Fatalf("expected capacity 10, got %d", got)
	}
}

func TestCacheDefaultCapacity(t *testing.T) {
	c := cache.New(0)
	if got := c.Capacity(); got != 256 {
		t.Fatalf("expected default capacity 256, got %d", got)
	}
}

func TestCacheSetGet(t *testing.T) {
	c := cache.New(4)
	expr, err := parser.Compile("$.name")
	if err != nil {
		t.Fatal(err)
	}
	c.Set("$.name", expr)
	if got := c.Len(); got != 1 {
		t.Fatalf("expected 1 entry, got %d", got)
	}
	got, ok := c.Get("$.name")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != expr {
		t.Fatal("expected same expression pointer")
	}
}

func TestCacheMiss(t *testing.T) {
	c := cache.New(4)
	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected cache miss")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := cache.New(3)
	for _, k := range []string{"a", "b", "c", "d"} {
		expr, _ := parser.Compile("$.x")
		c.Set(k, expr)
	}
	if got := c.Len(); got != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", got)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal(`expected "a" to be evicted (LRU)`)
	}
	if _, ok := c.Get("d"); !ok {
		t.Fatal(`expected most-recently-inserted "d" to survive`)
	}
}

func TestCacheInvalidate(t *testing.T) {
	c := cache.New(4)
	expr, _ := parser.Compile("$.x")
	c.Set("k", expr)
	c.Invalidate("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after Invalidate")
	}
}

func TestCacheClear(t *testing.T) {
	c := cache.New(4)
	for _, k := range []string{"a", "b", "c"} {
		expr, _ := parser.Compile("$.x")
		c.Set(k, expr)
	}
	c.Clear()
	if got := c.Len(); got != 0 {
		t.Fatalf("expected 0 after Clear, got %d", got)
	}
}

func TestCacheGetOrCompile(t *testing.T) {
	c := cache.New(4)
	callCount := 0
	compileFn := func() (*types.Expression, error) {
		callCount++
		return parser.Compile("$.age")
	}

	expr1, err := c.GetOrCompile("$.age", compileFn)
	if err != nil || expr1 == nil {
		t.Fatalf("first GetOrCompile: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 compile call, got %d", callCount)
	}

	expr2, err := c.GetOrCompile("$.age", compileFn)
	if err != nil || expr2 == nil {
		t.Fatalf("second GetOrCompile: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected still 1 call (cached), got %d", callCount)
	}
	if expr1 != expr2 {
		t.Fatal("expected same pointer from cache")
	}
}

func TestCacheSetUpdate(t *testing.T) {
	c := cache.New(4)
	expr1, _ := parser.Compile("$.a")
	expr2, _ := parser.Compile("$.b")
	c.Set("k", expr1)
	c.Set("k", expr2) // overwrite
	got, ok := c.Get("k")
	if !ok {
		t.Fatal("expected hit after overwrite")
	}
	if got != expr2 {
		t.Fatal("expected updated expression pointer")
	}
	if c.Len() != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d", c.Len())
	}
}
