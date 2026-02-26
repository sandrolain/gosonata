# GoSonata Differences and Implementation Notes

**Version**: 0.1.0-dev
**Last Updated**: February 26, 2026
**Reference**: JSONata 2.1.0+ (JavaScript)

## Table of Contents

- [Overview](#overview)
- [Platform Differences](#platform-differences)
- [API Design Differences](#api-design-differences)
- [Implementation Differences](#implementation-differences)
- [Known Limitations](#known-limitations)
- [Extension Functions](#extension-functions)
- [Advantages over Other Implementations](#advantages-over-other-implementations)
- [Comparison with go-jsonata v206](#comparison-with-go-jsonata-v206)
- [Migration Notes](#migration-notes)

---

## Overview

This document describes differences between GoSonata and other JSONata implementations, particularly:

1. **JavaScript reference implementation** (official JSONata 2.1.0+)
2. **go-jsonata v206** (Blues Inc.'s Go port)
3. **Other Go implementations** (jsonata-go, etc.)

GoSonata aims for 100% semantic compatibility with JSONata 2.1.0+ while embracing Go idioms and improving upon existing Go implementations.

---

## Platform Differences

These differences arise from fundamental platform characteristics (JavaScript vs Go) and affect all Go implementations.

### 1. Type System

#### JavaScript

```javascript
// Single number type
let x = 42;          // Always Number (float64 internally)
let y = 3.14;        // Same type

// Null vs undefined
let a = null;        // Explicit null
let b = undefined;   // Undefined variable
let c;               // Also undefined
```

#### GoSonata

```go
// Multiple numeric types
var x int64 = 42
var y float64 = 3.14

// Null vs undefined
var a interface{} = types.NullValue  // Explicit null
var b interface{} = nil              // Undefined/missing
```

**Impact**:

- GoSonata uses `float64` as the primary numeric type (matching JavaScript)
- Integers are automatically converted to `float64` when needed
- **At the public API boundary**, both JSONata `null` and `undefined` are returned as Go `nil`
- `types.Null{}` is used **internally** during evaluation to distinguish `null` from `undefined`; it is converted to `nil` before returning from `Eval()` / `EvalWithBindings()`

**Practical implication**: data passed to `Eval()` must use `float64` for numeric fields
(not `int`). Use `json.Unmarshal` to deserialise data and all numbers will automatically
be `float64`, matching the behaviour of JSONata JS.

**Rationale**: This mapping preserves JSONata semantics while using Go's native types efficiently.

---

### 2. Map Iteration Order

#### JavaScript

```javascript
// Deterministic (ES2015+)
const obj = { b: 2, a: 1, c: 3 };
Object.keys(obj);  // Always ["b", "a", "c"] (insertion order)
```

#### Go

```go
// Randomized by design (security feature)
obj := map[string]interface{}{"b": 2, "a": 1, "c": 3}
for k, v := range obj {
    // Order is unpredictable
}
```

**Impact**:

- `$keys()` function returns keys in undefined order
- Object iteration order is non-deterministic
- Tests comparing key arrays must use unordered comparison

**GoSonata Solution**:

```go
// OrderedObject type preserves insertion order
type OrderedObject struct {
    Keys   []string
    Values map[string]interface{}
}
```

**Usage**:

- Object constructors (`{...}`) create `OrderedObject` when order matters
- `$keys()` returns keys in insertion order
- Test infrastructure supports `unordered: true` metadata for flexible comparison

**Comparison with go-jsonata**:

- **go-jsonata**: Hard-codes test pattern matching in test suite
- **GoSonata**: Generic metadata-driven approach (more maintainable)

---

### 3. Unicode and String Handling

#### JavaScript (UTF-16)

```javascript
// UTF-16 encoding
"𝄞".length;           // 2 (surrogate pair)
"𝄞".substring(0, 1);  // "�" (invalid)
```

#### GoSonata (UTF-8)

```go
// UTF-8 encoding
len("𝄞")              // 4 bytes
utf8.RuneCountInString("𝄞")  // 1 rune (correct)
```

**Impact**:

- String length differs for characters outside Basic Multilingual Plane (BMP)
- Substring operations behave differently with surrogate pairs
- Invalid UTF-8 vs invalid UTF-16 handling

**GoSonata Approach**:

- Use `utf8.RuneCountInString()` for character count
- Rune-aware substring operations
- Skip official tests involving invalid UTF-16 surrogates (not applicable to Go)

**Known Limitations**:

- Tests in `string-invalid-surrogates.json` are skipped (UTF-16 specific)
- Cannot perfectly replicate JavaScript's surrogate pair behavior

**Status**: All string functions are fully implemented and handle UTF-8 correctly; surrogate tests skipped as non-applicable.

---

### 4. Numeric Precision

#### JavaScript

```javascript
// IEEE 754 double precision
Number.MAX_SAFE_INTEGER;  // 2^53 - 1 = 9007199254740991
let big = 9007199254740992;  // Precision lost
```

#### GoSonata

```go
// IEEE 754 double precision (same)
float64(9007199254740992)  // Same precision limits

// But int64 can represent larger integers
var bigInt int64 = 9223372036854775807  // 2^63 - 1
```

**Impact**:

- Float arithmetic precision matches JavaScript
- Integer arithmetic can be more precise (when using `int64` internally)
- Large integer literals may lose precision when converted to `float64`

**GoSonata Approach**:

- Use `float64` for all JSONata numeric operations (spec compliance)
- Internal operations may use `int64` for integer math (optimization)
- Results converted back to `float64` before returning

**Note**: This ensures compatibility with JavaScript implementation.

---

### 5. Asynchronous Execution

#### JavaScript

```javascript
// Async evaluation with promises
const expr = jsonata('$.items');
expr.evaluate(data)
    .then(result => console.log(result))
    .catch(err => console.error(err));
```

#### GoSonata

```go
// Synchronous with context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := eval.Eval(ctx, expr, data)
if err != nil {
    log.Fatal(err)
}
```

**Differences**:

- JavaScript: Promise-based async evaluation
- GoSonata: Synchronous with `context.Context` for timeout/cancellation

**Advantages of Go Approach**:

- More explicit control flow
- Native goroutine support for concurrency
- No callback hell

---

## API Design Differences

GoSonata makes deliberate API design choices that differ from JavaScript implementation.

### 1. Compilation and Evaluation Separation

#### JavaScript (Official)

```javascript
const jsonata = require('jsonata');

// Combined: compile and create evaluable object
const expression = jsonata('$.name');
const result = expression.evaluate(data);
```

#### GoSonata

```go
import "github.com/sandrolain/gosonata"

// Explicit separation
expr, err := gosonata.Compile("$.name")  // Parse -> AST
eval := evaluator.New()                   // Create evaluator
result, err := eval.Eval(ctx, expr, data) // Evaluate
```

**Rationale**:

- **Clearer separation of concerns**: Parsing vs execution
- **Reusable evaluator**: Configure once, evaluate many expressions
- **Explicit error handling**: No magic, all errors explicit
- **Context support**: Native Go pattern for timeout/cancellation

**Convenience API** (for simple cases):

```go
// One-shot evaluation (similar to JavaScript style)
result, err := gosonata.Eval("$.name", data)
```

---

### 2. Option Configuration

#### JavaScript (Official)

```javascript
// Options passed to evaluate()
expression.evaluate(data, {
    maxDepth: 100,
    timeout: 5000
});
```

#### GoSonata

```go
// Functional options pattern (idiomatic Go)
eval := evaluator.New(
    evaluator.WithMaxDepth(100),
    evaluator.WithTimeout(5*time.Second),
    evaluator.WithDebug(true),
)
```

**Advantages**:

- Type-safe configuration
- Discoverable via IDE autocomplete
- Compile-time validation
- Extensible without breaking changes

---

### 3. Context Management

#### JavaScript

```javascript
// No explicit context handling
const result = expression.evaluate(data);

// Timeout via Promise.race()
Promise.race([
    expression.evaluate(data),
    new Promise((_, reject) =>
        setTimeout(() => reject(new Error('timeout')), 5000)
    )
]);
```

#### GoSonata

```go
// Native context.Context pattern
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := eval.Eval(ctx, expr, data)
if errors.Is(err, context.DeadlineExceeded) {
    fmt.Println("Query timed out")
}
```

**Advantages**:

- Standard Go pattern for cancellation
- Propagates across function calls
- Integrates with Go ecosystem (HTTP handlers, gRPC, etc.)
- Built-in goroutine coordination

---

### 4. Error Handling

#### JavaScript

```javascript
try {
    const result = expression.evaluate(data);
} catch (err) {
    console.error(err.code);     // Error code
    console.error(err.position); // Position in query
    console.error(err.token);    // Problematic token
}
```

#### GoSonata

```go
result, err := eval.Eval(ctx, expr, data)
if err != nil {
    if jsonataErr, ok := err.(*types.Error); ok {
        fmt.Printf("Error %s at position %d\n",
            jsonataErr.Code,
            jsonataErr.Position)
    }
}
```

**Differences**:

- **JavaScript**: Exception-based error handling
- **GoSonata**: Explicit error return values (idiomatic Go)

**Advantages**:

- No hidden exceptions
- Compiler enforces error handling
- Errors are values (can be inspected, wrapped, etc.)

---

### 5. Function Registration

#### JavaScript

```javascript
// Register custom function
expression.registerFunction('myFunc', function(arg1, arg2) {
    return arg1 + arg2;
}, '<nn:n>');  // Signature
```

#### GoSonata

```go
// Type-safe function signature via WithCustomFunction
result, err := gosonata.Eval(`$add(1, 2)`, nil,
    gosonata.WithCustomFunction("add", "<nn:n>",
        func(ctx context.Context, args ...interface{}) (interface{}, error) {
            return args[0].(float64) + args[1].(float64), nil
        }),
)
```

Multiple custom functions can be registered by chaining `WithCustomFunction` options.
Custom functions are looked up before built-ins, so they can override any built-in
with the same name.

See [API.md — WithCustomFunction](API.md#withcustomfunction) for full documentation.

---

## Implementation Differences

These are internal implementation choices that affect performance or behavior.

### 1. Parser Architecture

#### JavaScript Reference

- **Tokenizer**: Regex-based token matching
- **Parser**: Pratt parser (operator precedence)
- **AST**: Dynamic JavaScript objects

#### GoSonata

- **Lexer**: Hand-written state machine (Rob Pike's pattern)
- **Parser**: Recursive descent with Pratt operator precedence
- **AST**: Strongly-typed structs

**Trade-offs**:

| Aspect | JavaScript | GoSonata |
|--------|------------|----------|
| **Performance** | Fast (V8 JIT) | Faster (compiled) |
| **Memory** | GC overhead | Lower overhead |
| **Type Safety** | Runtime only | Compile-time |
| **Flexibility** | High | Medium |

**Advantages of GoSonata approach**:

- Zero-allocation string scanning where possible
- Pre-computed operator precedence tables
- No regex overhead for tokenization
- Better error messages with precise positions

---

### 2. Evaluation Strategy

#### JavaScript

```javascript
// Async evaluation with promises
async function evaluate(node, data) {
    switch (node.type) {
        case 'path':
            return await evaluatePath(node, data);
        // ...
    }
}
```

#### GoSonata

```go
// Synchronous with context checking
func (e *Evaluator) evalNode(ctx context.Context, node *ASTNode, evalCtx *EvalContext) (interface{}, error) {
    // Check cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Evaluate synchronously
    switch node.Type {
    case NodePath:
        return e.evalPath(ctx, node, evalCtx)
    // ...
    }
}
```

**Differences**:

- **JavaScript**: Promise-based async (required for JS event loop)
- **GoSonata**: Synchronous with context cancellation

**Performance implications**:

- GoSonata evaluation is typically faster (no promise overhead)
- Parallelism achieved via goroutines (not promises)

---

### 3. Memory Management

#### JavaScript

- **Garbage Collection**: Generational GC with incremental marking
- **Object pooling**: Not commonly used
- **Memory overhead**: Higher per-object overhead

#### GoSonata

- **Garbage Collection**: Concurrent mark-sweep with lower latency
- **Object pooling**: `sync.Pool` for frequently allocated objects
- **Pre-allocation**: Slices pre-allocated when size is known

**Example**:

```go
// Pre-allocate slice
results := make([]interface{}, 0, len(items))

// Use sync.Pool for buffers
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

buf := bufferPool.Get().(*bytes.Buffer)
defer bufferPool.Put(buf)
```

**Result**: Lower memory footprint and GC pressure.

---

### 4. Concurrency Model

#### JavaScript

- **Single-threaded**: Event loop with async/await
- **No true parallelism**: Only concurrent I/O
- **Worker threads**: Available but complex to use

#### GoSonata

- **Native concurrency**: Goroutines and channels
- **True parallelism**: Multi-core utilization
- **Simple model**: Straightforward concurrent evaluation

**Example**:

```go
// Evaluate multiple expressions in parallel
func (e *Evaluator) EvalMany(ctx context.Context, queries []string, data interface{}) ([]interface{}, error) {
    results := make([]interface{}, len(queries))
    var wg sync.WaitGroup

    for i, query := range queries {
        wg.Add(1)
        go func(idx int, q string) {
            defer wg.Done()
            expr, _ := gosonata.Compile(q)
            results[idx], _ = e.Eval(ctx, expr, data)
        }(i, query)
    }

    wg.Wait()
    return results, nil
}
```

**Performance**: Significant speedup for CPU-bound operations on multi-core systems.

---

## Known Limitations

### 1. UTF-16 Surrogate Pairs

**Status**: Not applicable to Go

**Details**:

- JavaScript uses UTF-16 encoding internally
- Surrogate pairs represent characters outside BMP
- Go uses UTF-8, which handles all Unicode natively

**Impact**:

- Tests in `string-invalid-surrogates.json` are skipped
- GoSonata behavior correct for UTF-8, incompatible with UTF-16 quirks

**Decision**: Skip surrogate-specific tests; documented as non-applicable.

---

### 2. Map Key Ordering

**Status**: Solved with `OrderedObject`

**Details**:

- Go maps have randomized iteration order (security feature)
- JavaScript objects maintain insertion order (ES2015+)

**Impact**:

- `$keys()` would return keys in random order with plain maps
- Object iteration would be non-deterministic

**Solution**:

```go
type OrderedObject struct {
    Keys   []string               // Preserves insertion order
    Values map[string]interface{}  // Fast value lookup
}
```

**Cost**: Slight memory overhead, negligible performance impact.

---

### 3. Floating-Point Arithmetic

**Status**: Inherent to IEEE 754

**Details**:

- Both JavaScript and Go use IEEE 754 double precision
- Floating-point arithmetic has rounding errors
- Some decimal numbers cannot be represented exactly

**Example**:

```go
// Both implementations
0.1 + 0.2 == 0.30000000000000004  // Not 0.3
```

**Impact**: Identical behavior to JavaScript (spec compliance).

---

### 4. Regular Expression Dialect

**Status**: Differences documented

**Details**:

- JavaScript: ECMA-262 regex flavor
- Go: RE2 regex flavor (subset of PCRE)

**Differences**:

| Feature | JavaScript | Go RE2 |
|---------|------------|--------|
| Lookahead | ✅ | ❌ |
| Lookbehind | ✅ | ❌ |
| Backreferences | ✅ | ❌ |
| Unicode | ✅ | ✅ |

**Impact**: Some regex patterns valid in JavaScript may not work in Go.

**Mitigation**: Document unsupported patterns; most common patterns work fine.

---

### 5. Function Signature Enforcement

**Status**: Partial — argument count validated; full type-pattern validation pending

**Details**:

- JavaScript implementation validates type signatures at runtime
- GoSonata validates argument count only; the signature string is stored but
  full type-pattern matching (e.g. `<a<n>:n>` = "array of numbers returns number")
  is not yet applied to arguments
- Custom functions accept the signature param but it is not enforced

**Roadmap**: Full signature validation planned for a future release.

---

## Extension Functions

GoSonata ships a `pkg/ext` library of optional functions beyond the JSONata 2.1.0+
specification. Because they are **not part of the standard**, they are off by
default and must be explicitly opted in, ensuring that expressions evaluated without
extension options remain portable to other compliant JSONata implementations.

See [API.md — Extension Functions](API.md#extension-functions-pkgext) for the
complete function reference and all usage patterns.

**Quick start**:

```go
import (
    "github.com/sandrolain/gosonata"
    "github.com/sandrolain/gosonata/pkg/ext"
)

// All extensions
result, err := gosonata.Eval(`$uuid()`, nil, ext.WithAll())

// Single category
result, err = gosonata.Eval(`$chunk(items, 3)`, data, ext.WithArray())
```

**Extension categories summary**:

| Package | # functions | Notable additions |
|---------|-------------|-------------------|
| `extstring` | 12 | `$camelCase`, `$template`, `$startsWith`, `$endsWith` |
| `extnumeric` | 16 | `$median`, `$stddev`, `$clamp`, trig functions |
| `extarray` | 14 + 6 HOF | `$chunk`, `$flatten`, set ops, `$groupBy`, `$accumulate` |
| `extobject` | 9 + 2 HOF | `$pick`, `$omit`, `$deepMerge`, `$mapValues` |
| `exttypes` | 11 | `$isString`, `$isEmpty`, `$default`, `$identity` |
| `extdatetime` | 5 | `$dateAdd`, `$dateDiff`, `$dateComponents` |
| `extcrypto` | 3 | `$uuid`, `$hash`, `$hmac` |
| `extformat` | 3 | `$csv`, `$toCSV`, Go template |
| `extfunc` | 2 HOF | `$pipe`, `$memoize` |

---

## Advantages over Other Implementations

### Over JavaScript Reference

#### Performance

- **Compiled language**: No JIT warm-up time
- **Lower memory**: More efficient memory layout
- **Native concurrency**: True parallelism on multi-core systems
- **Faster parsing**: Hand-written lexer without regex overhead

#### Deployment

- **Single binary**: No runtime dependencies
- **Low memory footprint**: Suitable for embedded systems
- **Native cross-compilation**: Easy to build for multiple platforms

#### Type Safety

- **Compile-time checks**: Catch more errors before runtime
- **Stronger guarantees**: Type system prevents certain bugs

---

### Over go-jsonata v206

#### Code Quality

| Aspect | go-jsonata v206 | GoSonata |
|--------|-----------------|----------|
| **Origin** | Transliterated from JS | Written from scratch |
| **Code Style** | JavaScript-like | Idiomatic Go |
| **Structure** | Monolithic files | Clean package separation |
| **Documentation** | Minimal | Comprehensive |

#### Architecture

**go-jsonata**:

- ~4700 lines in `jsonata.go`
- ~2000 lines in `parser.go`
- Direct translation preserves JS patterns

**GoSonata**:

- Modular package structure
- Separation of concerns (lexer, parser, evaluator)
- Testable components

#### Test Infrastructure

**go-jsonata**:

```go
// Hard-coded test pattern matching (anti-pattern)
if strings.Contains(expr, "$keys(") &&
   strings.Contains(expr, "library.loans") {
    equal = deepEqualWithUnorderedStringArrays(result, testcase.Result)
}
```

**GoSonata**:

```go
// Generic metadata-driven comparison
type TestCase struct {
    Expression string      `json:"expr"`
    Result     interface{} `json:"result"`
    Unordered  bool        `json:"unordered"`  // Generic flag
}

if testCase.Unordered {
    return compareResultsUnordered(result, expected)
}
```

**Advantages**:

- More maintainable
- Extensible without code changes
- Documents test intent in data, not code

#### Error Handling

**go-jsonata**: Custom error types, less structured

**GoSonata**: Structured errors with standard codes, positions, wrapping

#### API Design

**go-jsonata**:

```go
// Tightly coupled
e := jsonata.MustCompile(query)
result, _ := e.Eval(data)
```

**GoSonata**:

```go
// Flexible, composable
expr := gosonata.MustCompile(query)
eval := evaluator.New(
    evaluator.WithTimeout(5*time.Second),
    evaluator.WithDebug(true),
)
result, _ := eval.Eval(ctx, expr, data)
```

---

## Comparison with go-jsonata v206

### Similarities

Both implementations:

- Target JSONata 2.1.0+ spec
- Face same platform constraints (Go vs JavaScript)
- Pass majority of official test suite
- Use similar AST structure

### Key Differences

#### 1. Codebase Origin

- **go-jsonata**: Transliterated from JavaScript using AI (Claude Code)
- **GoSonata**: Written from scratch for Go

**Impact**: GoSonata uses idiomatic Go patterns; go-jsonata preserves JS style.

#### 2. Code Organization

**go-jsonata**:

```
jsonata.go       # 4700 lines (evaluator + functions)
parser.go        # 2000 lines (lexer + parser)
datetime.go      # Date/time utilities
test_suite_test.go
```

**GoSonata**:

```
pkg/
  parser/        # Lexer, parser (separate files)
  evaluator/     # Evaluator (separate concerns)
  functions/     # Function registry
  types/         # Shared types
```

**Advantages**: GoSonata is more modular and testable.

#### 3. Error Handling

**go-jsonata**:

```go
type JSONataError struct {
    Value string
    Token string
    // ... other fields
}
```

**GoSonata**:

```go
type Error struct {
    Code     ErrorCode  // S0xxx, T0xxx, D0xxx, U0xxx
    Message  string
    Position int
    Token    string
    Err      error      // Supports error wrapping
}
```

**Advantages**: GoSonata errors are more structured and support Go 1.13+ error wrapping.

#### 4. Testing Approach

**go-jsonata**: Skips many tests, hard-codes workarounds

**GoSonata**: Metadata-driven test infrastructure, explicit skips with rationale

#### 5. Performance Focus

**go-jsonata**: Correctness focus

**GoSonata**: Correctness + performance (hand-written lexer, optimizations, benchmarks)

#### 6. API Flexibility

**go-jsonata**: Expression-centric API

**GoSonata**: Separate compilation and evaluation with configuration options

---

## Migration Notes

### From JavaScript JSONata

#### Basic Evaluation

**Before (JavaScript)**:

```javascript
const jsonata = require('jsonata');
const expression = jsonata('$.name');
const result = expression.evaluate(data);
```

**After (GoSonata)**:

```go
import (
    "context"
    "github.com/sandrolain/gosonata"
)

expr, err := gosonata.Compile("$.name")
if err != nil {
    log.Fatal(err)
}
result, err := gosonata.EvalWithContext(context.Background(), "$.name", data)
```

#### With Timeout

**Before (JavaScript)**:

```javascript
const timeout = new Promise((_, reject) =>
    setTimeout(() => reject(new Error('timeout')), 5000)
);

Promise.race([expression.evaluate(data), timeout])
    .then(result => console.log(result))
    .catch(err => console.error(err));
```

**After (GoSonata)**:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := gosonata.EvalWithContext(ctx, "$.name", data)
if errors.Is(err, context.DeadlineExceeded) {
    fmt.Println("Timeout")
}
```

---

### From go-jsonata v206

#### Simple Cases

**Before**:

```go
import jsonata "github.com/blues/jsonata-go"

e := jsonata.MustCompile(query)
result, err := e.Eval(data)
```

**After**:

```go
import "github.com/sandrolain/gosonata"

expr := gosonata.MustCompile(query)
eval := evaluator.New()
result, err := eval.Eval(context.Background(), expr, data)
```

#### With Options

**Before** (go-jsonata has limited options):

```go
e := jsonata.MustCompile(query)
// No option configuration
result, err := e.Eval(data)
```

**After** (GoSonata):

```go
expr := gosonata.MustCompile(query)
eval := evaluator.New(
    evaluator.WithMaxDepth(200),
    evaluator.WithTimeout(10*time.Second),
    evaluator.WithDebug(true),
)
result, err := eval.Eval(ctx, expr, data)
```

---

## Future Enhancements

### Implemented (Phase 7 ✅)

1. **Streaming API** — `gosonata.EvalStream` / `evaluator.EvalStream` (NDJSON, context-aware)
2. **Custom Functions** — `gosonata.WithCustomFunction` / `evaluator.WithCustomFunction`
3. **Expression Caching** — LRU cache via `WithCaching` / `WithCacheSize`

### Planned (Phase 8+)

1. **Plugin System**: Loadable function libraries
2. **WASM Export**: Run in browser / edge environments

### Backward Compatibility

GoSonata maintains semantic compatibility with JSONata specification across versions.

API changes before v1.0.0 may occur but will be clearly documented.

---

## Summary

### Key Takeaways

1. **Platform Differences**: Go's type system, map ordering, and UTF-8 encoding require careful handling
2. **API Design**: GoSonata embraces Go idioms (contexts, options, explicit errors)
3. **Performance**: Hand-written lexer, concurrent evaluation, and memory optimizations
4. **Maintainability**: Clean architecture, comprehensive tests, extensive documentation
5. **Compatibility**: 100% semantic compatibility with JSONata spec (where platform permits)

### When to Use GoSonata

✅ **Good fit**:

- Go-based backend services
- High-performance data processing
- Embedded systems with resource constraints
- Microservices architecture
- CLI tools

❌ **Consider alternatives**:

- JavaScript/Node.js environments (use official jsonata package)
- Browser environments (use official jsonata package or WASM build when available)

---

## References

- [Architecture Documentation](ARCHITECTURE.md)
- [API Reference](API.md)
- [JSONata Official Documentation](https://docs.jsonata.org/)
- [go-jsonata v206 Repository](https://github.com/blues/jsonata-go)

---

**Document Maintenance**: This document should be updated when implementation differences are discovered or resolved. See [GitHub Copilot Instructions](../.github/copilot-instructions.md) for update procedures.
