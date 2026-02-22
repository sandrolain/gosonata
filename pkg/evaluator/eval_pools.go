package evaluator

import (
	"bytes"
	"regexp"
	"sync"
)

// regexCache is a process-wide cache of compiled *regexp.Regexp, keyed by pattern string.
// Patterns are compiled once per process and the compiled form is reused across all
// goroutines. sync.Map is ideal here: write-once (first compilation) / read-many.
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
