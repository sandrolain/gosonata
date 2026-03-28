package unit_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/sandrolain/gosonata/pkg/cache"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// ---- basic functionality ---------------------------------------------------

func TestExprCache_New(t *testing.T) {
	c := cache.New(10)
	if got := c.Len(); got != 0 {
		t.Fatalf("expected empty cache, got %d", got)
	}
	if got := c.Capacity(); got != 10 {
		t.Fatalf("expected capacity 10, got %d", got)
	}
}

func TestExprCache_DefaultCapacity(t *testing.T) {
	c := cache.New(0)
	if got := c.Capacity(); got != 256 {
		t.Fatalf("expected default capacity 256, got %d", got)
	}
}

func TestExprCache_SetGet(t *testing.T) {
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

func TestExprCache_Miss(t *testing.T) {
	c := cache.New(4)
	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected cache miss")
	}
}

func TestExprCache_SetUpdate(t *testing.T) {
	c := cache.New(4)
	expr1, _ := parser.Compile("$.a")
	expr2, _ := parser.Compile("$.b")
	c.Set("k", expr1)
	c.Set("k", expr2)
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

// ---- FIFO eviction ---------------------------------------------------------

func TestExprCache_FIFOEviction(t *testing.T) {
	c := cache.New(3)
	for _, k := range []string{"a", "b", "c", "d"} {
		expr, _ := parser.Compile("$.x")
		c.Set(k, expr)
	}
	if got := c.Len(); got != 3 {
		t.Fatalf("expected 3 entries after FIFO eviction, got %d", got)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal(`expected "a" to be evicted (FIFO oldest)`)
	}
	if _, ok := c.Get("d"); !ok {
		t.Fatal(`expected "d" to survive`)
	}
}

func TestExprCache_FIFOEviction_Stats(t *testing.T) {
	c := cache.New(2)
	expr, _ := parser.Compile("$.x")
	c.Set("a", expr)
	c.Set("b", expr)
	c.Set("c", expr)
	stats := c.Stats()
	if stats.Evictions != 1 {
		t.Fatalf("expected 1 eviction, got %d", stats.Evictions)
	}
}

func TestExprCache_UpdateDoesNotEvict(t *testing.T) {
	c := cache.New(2)
	expr, _ := parser.Compile("$.x")
	expr2, _ := parser.Compile("$.y")
	c.Set("a", expr)
	c.Set("b", expr)
	c.Set("a", expr2)
	if _, ok := c.Get("b"); !ok {
		t.Fatal(`"b" should still be present after updating "a"`)
	}
	if e, ok := c.Get("a"); !ok || e != expr2 {
		t.Fatal(`"a" should contain updated expression`)
	}
	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}
}

// ---- Invalidate / Clear ----------------------------------------------------

func TestExprCache_Invalidate(t *testing.T) {
	c := cache.New(4)
	expr, _ := parser.Compile("$.x")
	c.Set("k", expr)
	c.Invalidate("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after Invalidate")
	}
	if c.Len() != 0 {
		t.Fatalf("expected empty after Invalidate, got %d", c.Len())
	}
}

func TestExprCache_InvalidateMissing(t *testing.T) {
	c := cache.New(4)
	c.Invalidate("not-there") // must not panic
}

func TestExprCache_Clear(t *testing.T) {
	c := cache.New(4)
	expr, _ := parser.Compile("$.x")
	for _, k := range []string{"a", "b", "c"} {
		c.Set(k, expr)
	}
	c.Clear()
	if got := c.Len(); got != 0 {
		t.Fatalf("expected 0 after Clear, got %d", got)
	}
}

// ---- GetOrCompile ----------------------------------------------------------

func TestExprCache_GetOrCompile(t *testing.T) {
	c := cache.New(4)
	calls := 0
	compileFn := func() (*types.Expression, error) {
		calls++
		return parser.Compile("$.age")
	}

	expr1, err := c.GetOrCompile("$.age", compileFn)
	if err != nil || expr1 == nil {
		t.Fatalf("first GetOrCompile: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 compile call, got %d", calls)
	}

	expr2, err := c.GetOrCompile("$.age", compileFn)
	if err != nil || expr2 == nil {
		t.Fatalf("second GetOrCompile: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected still 1 call (cached), got %d", calls)
	}
	if expr1 != expr2 {
		t.Fatal("expected same pointer from cache")
	}
}

// ---- Cacher interface compliance -------------------------------------------

func TestExprCache_ImplementsCacher(t *testing.T) {
	var _ cache.Cacher = cache.New(1)
}

// ---- Stats -----------------------------------------------------------------

func TestExprCache_Stats(t *testing.T) {
	c := cache.New(4)
	expr, _ := parser.Compile("$.x")
	c.Set("a", expr)

	c.Get("a") // hit
	c.Get("a") // hit
	c.Get("z") // miss

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Len != 1 {
		t.Fatalf("expected Len=1, got %d", stats.Len)
	}
	if stats.Capacity != 4 {
		t.Fatalf("expected Capacity=4, got %d", stats.Capacity)
	}
}

// ---- Concurrency -----------------------------------------------------------

func TestExprCache_Concurrent(t *testing.T) {
	const goroutines = 100
	const ops = 200
	c := cache.New(50)
	expr, _ := parser.Compile("$.x")

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("k%d", (g*ops+i)%80)
				c.Set(key, expr)
				c.Get(key)
				if i%10 == 0 {
					c.Invalidate(fmt.Sprintf("k%d", i%20))
				}
			}
		}()
	}
	wg.Wait()

	if c.Len() > c.Capacity() {
		t.Fatalf("cache len %d exceeds capacity %d after concurrent ops", c.Len(), c.Capacity())
	}
}

// ---- Benchmarks ------------------------------------------------------------

func BenchmarkExprCache_Get(b *testing.B) {
	c := cache.New(256)
	expr, _ := parser.Compile("$.name")
	c.Set("$.name", expr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("$.name")
	}
}

func BenchmarkExprCache_Get_Parallel(b *testing.B) {
	c := cache.New(256)
	expr, _ := parser.Compile("$.name")
	c.Set("$.name", expr)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get("$.name")
		}
	})
}

func BenchmarkExprCache_Set(b *testing.B) {
	c := cache.New(256)
	expr, _ := parser.Compile("$.name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("$.name", expr)
	}
}
