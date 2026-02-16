# GoSonata Architecture

**Version**: 0.1.0-dev
**Last Updated**: February 16, 2026
**Target**: JSONata 2.1.0+

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Package Structure](#package-structure)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [Type System](#type-system)
- [Error Handling](#error-handling)
- [Performance Optimizations](#performance-optimizations)
- [Concurrency Model](#concurrency-model)
- [Future Architecture](#future-architecture)

---

## Overview

GoSonata is a high-performance Go implementation of JSONata 2.1.0+, designed for intensive data streaming scenarios. The architecture follows clean separation of concerns with three primary stages:

1. **Lexical Analysis** → Tokenization
2. **Parsing** → Abstract Syntax Tree (AST) construction
3. **Evaluation** → Expression execution against data

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Query     │───▶│   Lexer     │───▶│   Parser    │───▶│  Evaluator  │
│  (string)   │    │  (tokens)   │    │    (AST)    │    │  (result)   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                                              │
                                              ▼
                                       ┌─────────────┐
                                       │  Functions  │
                                       │  Registry   │
                                       └─────────────┘
```

### Key Characteristics

- **Hand-written recursive descent parser** for maximum performance
- **Zero external dependencies** for core functionality
- **Context-aware evaluation** with timeout and cancellation support
- **Concurrent evaluation** enabled by default
- **Optional caching** for compiled expressions
- **Streaming support** for large JSON documents

---

## Design Principles

### 1. Performance First

- Minimize heap allocations in hot paths
- Use `sync.Pool` for frequently allocated objects
- Pre-allocate slices when size is known
- Avoid reflection in critical code paths
- Profile-guided optimizations

### 2. Idiomatic Go

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use standard library exclusively for core features
- Interfaces for extensibility
- Explicit error handling (no panics in public API)

### 3. Safety and Security

- Input validation at all boundaries
- Depth limits to prevent stack overflow
- Timeout mechanisms via `context.Context`
- Resource limits (memory, time, recursion)
- Safe defaults (opt-in for risky features)

### 4. Testability

- Unit tests for all packages
- 90%+ code coverage target
- Race condition detection (`-race` flag)
- Conformance tests against official JSONata suite
- Benchmark tests for all critical paths

### 5. Maintainability

- Clear separation of concerns
- Well-documented public APIs
- Architecture Decision Records (ADRs)
- Consistent code style via `gofmt` and linters

---

## Package Structure

```
gosonata/
├── gosonata.go              # Public API (top-level convenience)
│
├── pkg/
│   ├── parser/              # Lexical analysis & parsing
│   │   ├── parser.go        # Parser API & orchestration
│   │   ├── parser_impl.go   # Recursive descent implementation
│   │   ├── lexer.go         # Tokenization
│   │   └── tokens.go        # Token definitions
│   │
│   ├── evaluator/           # Expression evaluation
│   │   ├── evaluator.go     # Evaluator API
│   │   ├── eval_impl.go     # Evaluation implementation
│   │   ├── context.go       # Evaluation context & bindings
│   │   └── functions.go     # Built-in function implementations
│   │
│   ├── functions/           # Function registry
│   │   └── registry.go      # Function registration & lookup
│   │
│   ├── types/               # Core type system
│   │   ├── ast.go           # AST node definitions
│   │   ├── expression.go    # Compiled expression type
│   │   └── errors.go        # Error types & codes
│   │
│   ├── runtime/             # Runtime utilities (future)
│   └── cache/               # Caching system (future)
│
└── internal/                # Private implementation details
    ├── ast/                 # AST manipulation utilities
    └── runtime/             # Runtime helpers
```

### Package Responsibilities

#### `gosonata` (top-level)

Public API for common use cases:

```go
Compile(query string) (*Expression, error)
Eval(query, data) (result, error)
EvalWithContext(ctx, query, data) (result, error)
MustCompile(query string) *Expression
```

#### `pkg/parser`

Transforms JSONata expressions into Abstract Syntax Trees:

- **Lexer**: Tokenizes input strings
- **Parser**: Builds AST via recursive descent
- **Error Recovery**: Optional mode for malformed input

**Key Types**:

- `Token`: Lexical token with type, value, position
- `Parser`: Stateful parser with token stream
- `CompileOptions`: Parser configuration

#### `pkg/evaluator`

Evaluates AST nodes against JSON data:

- **Evaluator**: Stateful evaluation engine
- **EvalContext**: Variable bindings and data context
- **Built-in Functions**: Core function implementations

**Key Types**:

- `Evaluator`: Main evaluation engine
- `EvalContext`: Execution context with bindings
- `EvalOptions`: Evaluator configuration

#### `pkg/types`

Shared type definitions:

- **AST Nodes**: 23+ node types (literals, operators, functions, etc.)
- **Expression**: Compiled expression container
- **Errors**: Structured error types with codes

**Key Types**:

- `ASTNode`: AST node with type, value, relations
- `Expression`: Compiled expression with AST
- `Error`: JSONata error with code and position

#### `pkg/functions`

Function registry and management:

- **Registry**: Function registration and lookup
- **Signatures**: Function signature definitions

**Key Types**:

- `FunctionRegistry`: Function storage and retrieval
- `BuiltinFunc`: Function signature type

---

## Core Components

### 1. Lexer

**Purpose**: Convert input string into token stream

**Implementation**: Hand-written lexer based on Rob Pike's "Lexical Scanning in Go" pattern

**Key Features**:

- Stateful scanning with position tracking
- Context-sensitive tokenization (regex vs division)
- Unicode support (UTF-8)
- Detailed error reporting

**Token Types** (partial list):

- Literals: `String`, `Number`, `Boolean`, `Null`
- Operators: `+`, `-`, `*`, `/`, `=`, `!=`, `<`, `>`, `<=`, `>=`
- Special: `$` (root), `@` (context), `#` (index), `%` (parent)
- Structural: `{`, `}`, `[`, `]`, `(`, `)`, `,`, `;`

**Performance Considerations**:

- Single-pass tokenization
- No backtracking
- Minimal allocations
- O(n) time complexity

### 2. Parser

**Purpose**: Build Abstract Syntax Tree from token stream

**Implementation**: Hand-written recursive descent parser

**Algorithm**:

```go
func (p *Parser) parseExpression(rbp int) (*ASTNode, error) {
    // Pratt parser with operator precedence
    left, err := p.parsePrefix()
    if err != nil {
        return nil, err
    }

    for p.current().BindingPower() > rbp {
        left, err = p.parseInfix(left)
        if err != nil {
            return nil, err
        }
    }

    return left, nil
}
```

**Key Features**:

- **Operator Precedence**: Pratt parsing with binding powers
- **Error Recovery**: Optional mode for partial parsing
- **Depth Limiting**: Prevents stack overflow
- **Position Tracking**: Detailed error locations

**Parsing Stages**:

1. **Prefix**: Literals, identifiers, unary operators
2. **Infix**: Binary operators, function calls, paths
3. **Postfix**: Array/object access, filters

**Performance Considerations**:

- Pre-computed operator precedence tables
- Minimal AST node allocations
- O(n) parsing complexity for well-formed input

### 3. AST (Abstract Syntax Tree)

**Node Structure**:

```go
type ASTNode struct {
    Type     NodeType           // Node type identifier
    Value    interface{}        // Literal value (if any)
    Position int                // Source position

    // Relations
    LHS         *ASTNode        // Left-hand side
    RHS         *ASTNode        // Right-hand side
    Steps       []*ASTNode      // Path steps
    Arguments   []*ASTNode      // Function arguments
    Expressions []*ASTNode      // Block expressions

    // Attributes
    KeepArray bool              // Preserve array structure
    ConsArray bool              // Force array construction
    Stage     string            // Pipeline stage
}
```

**Node Types** (23 total):

| Category | Types |
|----------|-------|
| **Literals** | `string`, `number`, `boolean`, `null` |
| **Navigation** | `path`, `name`, `wildcard`, `descendant`, `parent` |
| **Operators** | `binary`, `unary` |
| **Functions** | `function`, `lambda`, `partial` |
| **Control Flow** | `condition`, `block`, `bind` |
| **Constructors** | `array`, `object` |
| **Special** | `regex`, `variable`, `transform`, `sort`, `filter`, `context`, `index`, `range`, `apply` |

**Design Decisions**:

- Flat structure (no visitor pattern) for performance
- Direct evaluation via type switching
- Attributes for optimization hints

### 4. Evaluator

**Purpose**: Execute AST against JSON data

**Algorithm**:

```go
func (e *Evaluator) evalNode(ctx context.Context, node *ASTNode, evalCtx *EvalContext) (interface{}, error) {
    // Context cancellation check
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Type-based dispatch
    switch node.Type {
    case NodeLiteral:
        return node.Value, nil
    case NodePath:
        return e.evalPath(ctx, node, evalCtx)
    case NodeBinary:
        return e.evalBinary(ctx, node, evalCtx)
    // ... 20+ more cases
    }
}
```

**Key Features**:

- **Context Management**: Variable bindings and data scope
- **Depth Tracking**: Prevents infinite recursion
- **Timeout Support**: Via `context.Context`
- **Function Dispatch**: Built-in and lambda functions

**Evaluation Context**:

```go
type EvalContext struct {
    data     interface{}              // Current data ($)
    parent   *EvalContext             // Parent context ($$)
    bindings map[string]interface{}   // Variable bindings
    depth    int                      // Recursion depth
}
```

**Performance Optimizations**:

- Short-circuit evaluation for boolean operators
- Lazy evaluation of conditional branches
- Sequence flattening optimizations
- Type assertion caching

### 5. Function Registry

**Purpose**: Manage built-in function implementations

**Structure**:

```go
type FunctionRegistry struct {
    functions  map[string]BuiltinFunc
    signatures map[string]string
}

type BuiltinFunc func(ctx context.Context, args ...interface{}) (interface{}, error)
```

**Registration**:

```go
registry.Register("$sum", fnSum, "<a<n>:n>")
registry.Register("$uppercase", fnUppercase, "<s:s>")
```

**Lookup**:

```go
fn, ok := registry.Lookup("$sum")
if !ok {
    return ErrUndefinedFunction
}
result, err := fn(ctx, args...)
```

**Built-in Functions** (66+ planned):

- String: 13 functions (`substring`, `uppercase`, `lowercase`, etc.)
- Numeric: 12 functions (`sum`, `count`, `max`, `min`, etc.)
- Array: 10 functions (`append`, `reverse`, `sort`, etc.)
- Aggregate: 6 functions (`sum`, `average`, `min`, `max`, etc.)
- Higher-order: 5 functions (`map`, `filter`, `reduce`, etc.)
- Boolean: 3 functions (`boolean`, `not`, `exists`)
- Date/Time: 7 functions (`now`, `fromMillis`, etc.)
- Encoding: 4 functions (`encodeUrl`, `decodeUrl`, etc.)
- Special: 4 functions (`type`, `eval`, `assert`, `error`)

---

## Data Flow

### Compilation Flow

```
┌──────────────┐
│ Input Query  │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│    Lexer     │  Tokenization
│  (tokenize)  │  - Scan input string
└──────┬───────┘  - Identify tokens
       │          - Track positions
       ▼
┌──────────────┐
│    Parser    │  AST Construction
│   (parse)    │  - Build tree structure
└──────┬───────┘  - Validate syntax
       │          - Report errors
       ▼
┌──────────────┐
│  Expression  │  Compiled Result
│   (ready)    │  - Contains AST
└──────────────┘  - Reusable
```

### Evaluation Flow

```
┌──────────────┐  ┌──────────────┐
│  Expression  │  │  Input Data  │
└──────┬───────┘  └──────┬───────┘
       │                 │
       └────────┬────────┘
                ▼
       ┌──────────────┐
       │  Evaluator   │  Initialization
       │    (new)     │  - Create context
       └──────┬───────┘  - Set options
              │
              ▼
       ┌──────────────┐
       │  eval(AST)   │  Recursive Evaluation
       │              │  - Navigate tree
       │  ┌─────────┐ │  - Apply operators
       │  │ Context │ │  - Call functions
       │  └─────────┘ │  - Manage bindings
       └──────┬───────┘
              │
              ▼
       ┌──────────────┐
       │    Result    │  Final Value
       └──────────────┘
```

### Function Call Flow

```
┌──────────────┐
│ Function AST │
│   Node       │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Evaluate     │  Argument Evaluation
│  Arguments   │  - Eval each arg
└──────┬───────┘  - Type conversion
       │
       ▼
┌──────────────┐
│   Lookup     │  Function Resolution
│   Function   │  - Check registry
└──────┬───────┘  - Get signature
       │
       ▼
┌──────────────┐
│   Validate   │  Signature Matching
│  Signature   │  - Check arg count
└──────┬───────┘  - Check arg types
       │
       ▼
┌──────────────┐
│   Execute    │  Function Call
│   Function   │  - Native Go code
└──────┬───────┘  - Return result
       │
       ▼
┌──────────────┐
│    Result    │
└──────────────┘
```

---

## Type System

### Go Type Mapping

JSONata types map to Go types as follows:

| JSONata | Go | Notes |
|---------|-----|-------|
| `undefined` | `nil` | Go's zero value for interface{} |
| `null` | `types.Null{}` | Custom type to distinguish from undefined |
| `boolean` | `bool` | Native Go boolean |
| `number` | `float64` | Default numeric type |
| `string` | `string` | UTF-8 encoded |
| `array` | `[]interface{}` | Dynamic array |
| `object` | `map[string]interface{}` or `*OrderedObject` | Ordered when key order matters |
| `function` | `*Lambda` | Lambda function type |

### Special Types

#### Null Type

```go
type Null struct{}

var NullValue = Null{}

func (Null) MarshalJSON() ([]byte, error) {
    return []byte("null"), nil
}
```

**Design Decision**: Distinguish JSONata `null` (explicit) from `undefined` (Go `nil`).

#### OrderedObject

```go
type OrderedObject struct {
    Keys   []string
    Values map[string]interface{}
}
```

**Purpose**: Preserve object key order for deterministic output.

**Use Cases**:

- `$keys()` function
- Object construction with `{...}`
- Output serialization

#### Lambda Function

```go
type Lambda struct {
    Arguments  []string        // Parameter names
    Body       *ASTNode        // Function body
    Context    *EvalContext    // Closure bindings
    Signature  string          // Type signature
}
```

**Purpose**: Represent user-defined lambda functions.

**Example**:

```jsonata
$sum := function($x, $y) { $x + $y };
```

### Type Conversions

#### String Coercion

```go
func toString(v interface{}) string {
    switch val := v.(type) {
    case string:
        return val
    case float64:
        return formatNumber(val)
    case bool:
        return strconv.FormatBool(val)
    case nil:
        return ""
    default:
        // JSON serialization
    }
}
```

#### Number Coercion

```go
func toNumber(v interface{}) (float64, error) {
    switch val := v.(type) {
    case float64:
        return val, nil
    case int:
        return float64(val), nil
    case string:
        return strconv.ParseFloat(val, 64)
    default:
        return 0, ErrCannotConvertNumber
    }
}
```

#### Boolean Coercion

```go
func toBoolean(v interface{}) bool {
    switch val := v.(type) {
    case bool:
        return val
    case nil:
        return false
    case string:
        return val != ""
    case float64:
        return val != 0
    case []interface{}:
        return len(val) > 0
    default:
        return true
    }
}
```

---

## Error Handling

### Error Types

#### Structured Errors

```go
type Error struct {
    Code     ErrorCode  // S0xxx, T0xxx, D0xxx, U0xxx
    Message  string     // Human-readable message
    Position int        // Character position in input
    Token    string     // Problematic token
    Err      error      // Wrapped error
}
```

#### Error Categories

| Code Prefix | Category | Examples |
|-------------|----------|----------|
| **S0xxx** | Syntax/Parser | `S0101`: String not closed |
| **T0xxx** | Type errors | `T0410`: Argument count mismatch |
| **D0xxx** | Evaluation | `D1002`: Invoke non-function |
| **U0xxx** | Runtime | `U1001`: Undefined variable |

### Error Handling Strategy

#### 1. Early Validation

```go
func (p *Parser) Parse() (*Expression, error) {
    if p.input == "" {
        return nil, NewError(ErrSyntaxError, "empty expression", 0)
    }
    // ...
}
```

#### 2. Error Wrapping

```go
if err != nil {
    return nil, fmt.Errorf("failed to evaluate path at position %d: %w", pos, err)
}
```

#### 3. Panic Recovery

```go
func (e *Evaluator) safeEval(ctx context.Context, node *ASTNode) (result interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("evaluation panic: %v", r)
        }
    }()
    return e.eval(ctx, node)
}
```

#### 4. Context Cancellation

```go
func (e *Evaluator) evalNode(ctx context.Context, node *ASTNode, evalCtx *EvalContext) (interface{}, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        // Proceed
    }
}
```

---

## Performance Optimizations

### 1. Memory Management

#### Pre-allocation

```go
// Allocate slice with known capacity
results := make([]interface{}, 0, len(items))
```

#### sync.Pool

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

buf := bufferPool.Get().(*bytes.Buffer)
defer bufferPool.Put(buf)
```

### 2. Parsing Optimizations

#### Operator Precedence Table

```go
var bindingPowers = map[TokenType]int{
    TokenOr:       10,
    TokenAnd:      20,
    TokenEqual:    30,
    TokenNotEqual: 30,
    // ... pre-computed
}
```

#### Token Lookahead

```go
func (p *Parser) peek() Token {
    if p.pos+1 < len(p.tokens) {
        return p.tokens[p.pos+1]
    }
    return TokenEOF
}
```

### 3. Evaluation Optimizations

#### Short-circuit Evaluation

```go
func (e *Evaluator) evalAnd(ctx context.Context, lhs, rhs *ASTNode) (bool, error) {
    left, err := e.eval(ctx, lhs)
    if err != nil || !toBoolean(left) {
        return false, err  // Short-circuit
    }
    right, err := e.eval(ctx, rhs)
    return toBoolean(right), err
}
```

#### Sequence Flattening

```go
func (e *Evaluator) flatten(arr []interface{}) []interface{} {
    result := make([]interface{}, 0, len(arr))
    for _, item := range arr {
        if subArr, ok := item.([]interface{}); ok {
            result = append(result, e.flatten(subArr)...)
        } else {
            result = append(result, item)
        }
    }
    return result
}
```

#### Type Assertion Caching

```go
// Cache type assertion results in hot paths
if cached, ok := e.typeCache[node]; ok {
    return cached
}
```

### 4. Benchmarking

```go
func BenchmarkEvaluation(b *testing.B) {
    expr := MustCompile("$.items[price > 100]")
    data := loadTestData()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = expr.Eval(context.Background(), data)
    }
}
```

---

## Concurrency Model

### Design Principles

1. **Safe by Default**: All public APIs are goroutine-safe
2. **Concurrent Evaluation**: Optional parallel evaluation of independent expressions
3. **No Shared Mutable State**: Each evaluation has isolated context

### Concurrent Evaluation

```go
func (e *Evaluator) EvalMany(ctx context.Context, queries []string, data interface{}) ([]interface{}, error) {
    results := make([]interface{}, len(queries))
    errs := make([]error, len(queries))

    var wg sync.WaitGroup
    for i, query := range queries {
        wg.Add(1)
        go func(idx int, q string) {
            defer wg.Done()
            result, err := e.Eval(ctx, q, data)
            results[idx] = result
            errs[idx] = err
        }(i, query)
    }

    wg.Wait()
    return results, errors.Join(errs...)
}
```

### Thread-Safe Components

#### Function Registry

```go
type FunctionRegistry struct {
    mu         sync.RWMutex
    functions  map[string]BuiltinFunc
}

func (r *FunctionRegistry) Lookup(name string) (BuiltinFunc, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    fn, ok := r.functions[name]
    return fn, ok
}
```

#### Expression Cache (Future)

```go
type Cache struct {
    mu      sync.RWMutex
    items   map[string]*Expression
    maxSize int
}

func (c *Cache) Get(key string) (*Expression, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    expr, ok := c.items[key]
    return expr, ok
}
```

### Race Condition Prevention

- All tests run with `-race` flag
- No global mutable state
- Isolated evaluation contexts
- Synchronized access to shared registries

---

## Future Architecture

### Planned Features

#### 1. Expression Caching

```go
type CachePolicy interface {
    ShouldCache(query string) bool
    EvictionPolicy() EvictionPolicy
}

func WithCache(cache *Cache, policy CachePolicy) EvalOption
```

#### 2. Custom Functions

```go
type CustomFunc func(ctx context.Context, args ...interface{}) (interface{}, error)

func RegisterFunction(name string, fn CustomFunc, signature string) error
```

#### 3. Streaming Support

```go
func EvalStream(ctx context.Context, query string, r io.Reader) (interface{}, error)
```

#### 4. Plugin System

```go
type Plugin interface {
    Name() string
    Init(*Evaluator) error
    Functions() map[string]BuiltinFunc
}

func LoadPlugin(path string) (Plugin, error)
```

#### 5. OpenTelemetry Integration

```go
import "go.opentelemetry.io/otel"

func WithTracer(tracer trace.Tracer) EvalOption
func WithMeter(meter metric.Meter) EvalOption
```

### Architecture Evolution

#### Phase 1-4 (Current)

- Core parser and evaluator
- Basic built-in functions
- Test suite integration

#### Phase 5-6 (In Progress)

- Advanced functions (higher-order, date/time)
- Performance optimizations
- Streaming support

#### Phase 7 (Future)

- Custom function registration
- Expression caching
- API stabilization

#### Phase 8+ (Roadmap)

- Plugin system
- WASM export
- OpenTelemetry integration
- Advanced streaming features

---

## References

- [JSONata Specification](https://docs.jsonata.org/)
- [API Documentation](API.md)
- [Differences from Other Implementations](DIFFERENCES.md)

---

**Document Maintenance**: This document should be updated whenever architectural changes are made. See [GitHub Copilot Instructions](../.github/copilot-instructions.md) for update procedures.
