package evaluator

import (
	"bytes"
	"regexp"
	"sync"
)

// regexCache is a process-wide cache of compiled *regexp.Regexp, keyed by pattern string.
// Patterns are compiled once per process and the compiled form is reused across all
// goroutines. sync.Map is ideal here: write-once (first compilation) / read-many.
//
// THREAD-SAFETY AUDIT: safe.
//   - sync.Map handles concurrent reads and writes without external locking.
//   - *regexp.Regexp is immutable after compilation; concurrent use is safe per the
//     regexp package documentation.
//   - In the rare case where two goroutines compile the same pattern concurrently,
//     both Store the same deterministic value — the later write is harmless.
//   - No entry is ever deleted or mutated after insertion.
var regexCache sync.Map // map[string]*regexp.Regexp

// getOrCompileRegex retrieves or compiles a regex pattern.
// It caches the result in regexCache for subsequent calls.
// regexPattern must already be in Go regexp syntax (converted by the lexer).
func getOrCompileRegex(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	// Store even if another goroutine stored concurrently — both store the same value.
	regexCache.Store(pattern, re)
	return re, nil
}

// mustCompileRegex compiles a static pattern via the shared regex cache, panicking on error.
// Use this for package-level var declarations to pre-warm the cache for known-good patterns.
func mustCompileRegex(pattern string) *regexp.Regexp {
	re, err := getOrCompileRegex(pattern)
	if err != nil {
		panic("evaluator: failed to compile static regex: " + err.Error())
	}
	return re
}

// bufPool is a process-wide pool of *bytes.Buffer used in hot string-building
// paths (JSON marshaling, regex replacement, template expansion) to reduce GC
// pressure from short-lived buffer allocations.
//
// THREAD-SAFETY AUDIT: safe.
//   - sync.Pool is designed for concurrent use; Get/Put are internally locked.
//   - Each caller receives exclusive ownership of a buffer for the duration of its
//     use; the buffer is never shared between goroutines.
//   - Buffers are always Reset via acquireBuf() before use, so no residual state
//     from a previous owner is visible.
var bufPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// acquireBuf returns a reset buffer from the pool.
func acquireBuf() *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// releaseBuf returns a buffer to the pool. Only buffers whose internal backing
// array is reasonably sized are returned; very large ones are discarded to
// prevent unbounded memory retention.
func releaseBuf(b *bytes.Buffer) {
	if b.Cap() <= 64*1024 { // 64 KB ceiling
		bufPool.Put(b)
	}
}

// evalCtxPool is a process-wide pool of *EvalContext used in hot per-item
// evaluation loops (evalPath, evalObjects) to avoid a heap allocation for every
// element in an iterated array.
//
// THREAD-SAFETY AUDIT: safe.
//   - sync.Pool is designed for concurrent use.
//   - Each caller receives exclusive ownership via acquireEvalCtx; the context
//     is never shared between goroutines.
//   - Callers MUST NOT call releaseEvalCtx if the context was captured by a
//     lambda (c.escaped == true); markEscaped() ensures this.
var evalCtxPool = sync.Pool{
	New: func() interface{} { return new(EvalContext) },
}

// acquireEvalCtx returns a reset EvalContext from the pool configured as an
// array-item child of parent (or as a plain child when isArrayItem is false).
func acquireEvalCtx(data interface{}, parent *EvalContext, isArrayItem bool) *EvalContext {
	c := evalCtxPool.Get().(*EvalContext)
	c.data = data
	c.parent = parent
	if parent != nil {
		c.root = parent.root
		c.depth = parent.depth + 1
	} else {
		c.root = c
		c.depth = 0
	}
	c.isArrayItem = isArrayItem
	c.bindings = nil
	c.tcoTail = false
	c.escaped = false
	c.nowTime = nil
	return c
}

// releaseEvalCtx returns c to the pool. It is a no-op when c is nil or has
// been marked escaped (meaning a lambda closure holds a reference to it).
// All pointer fields are cleared before pooling to avoid retaining heap objects.
func releaseEvalCtx(c *EvalContext) {
	if c == nil || c.escaped {
		return
	}
	c.data = nil
	c.parent = nil
	c.root = nil
	c.bindings = nil
	c.depth = 0
	c.isArrayItem = false
	c.tcoTail = false
	c.nowTime = nil
	evalCtxPool.Put(c)
}
