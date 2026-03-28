# GoSonata API Reference

**Version**: 0.1.0-dev
**Last Updated**: March 4, 2026
**Go Version**: 1.26.0+

## Table of Contents

- [Quick Start](#quick-start)
- [Top-Level Functions](#top-level-functions)
  - [Compile](#compile)
  - [MustCompile](#mustcompile)
  - [Eval](#eval)
  - [EvalWithContext](#evalwithcontext)
  - [EvalStream (top-level)](#evalstream-top-level)
  - [StreamResult (top-level)](#streamresult-top-level)
  - [CustomFunc](#customfunc)
  - [Version](#version)
- [Parser Package](#parser-package)
- [Evaluator Package](#evaluator-package)
  - [EvalOption: WithCaching / WithCacheSize](#withcaching)
  - [EvalOption: WithCustomFunction](#withcustomfunction)
  - [EvalStream (Evaluator)](#evalstream-evaluator)
  - [StreamResult](#streamresult)
- [Types Package](#types-package)
- [Functions Package](#functions-package)
- [Extension Functions (pkg/ext)](#extension-functions-pkgext)
- [Stream Package (pkg/stream)](#stream-package-pkgstream)
  - [MultiEval](#multieval)
  - [EvalObserver](#evalobserver)
  - [MultiEvalStats](#multievalstats)
- [Error Handling](#error-handling)
- [Advanced Usage](#advanced-usage)
- [Examples](#examples)

---

## Quick Start

### Installation

```bash
go get github.com/sandrolain/gosonata
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sandrolain/gosonata"
)

func main() {
    // One-shot evaluation
    data := map[string]interface{}{
        "name": "John Doe",
        "age": 30,
    }

    result, err := gosonata.Eval("$.name", data)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result) // Output: John Doe
}
```

### Compile and Reuse

```go
import (
    "context"
    "github.com/sandrolain/gosonata"
    "github.com/sandrolain/gosonata/pkg/evaluator"
)

// Compile once, evaluate many times
expr, err := gosonata.Compile("$.items[price > 100]")
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
eval := evaluator.New()

// Evaluate against multiple datasets
result1, _ := eval.Eval(ctx, expr, data1)
result2, _ := eval.Eval(ctx, expr, data2)
```

---

## Top-Level Functions

The `gosonata` package provides convenience functions for common use cases.

### Compile

```go
func Compile(query string, opts ...parser.CompileOption) (*types.Expression, error)
```

Compiles a JSONata expression for repeated evaluation.

**Parameters**:

- `query`: JSONata expression string
- `opts`: Optional compilation options

**Returns**:

- `*types.Expression`: Compiled expression ready for evaluation
- `error`: Compilation error with position information

**Example**:

```go
expr, err := gosonata.Compile("$.items[price > 100]")
if err != nil {
    log.Fatal(err)
}
```

### MustCompile

```go
func MustCompile(query string) *types.Expression
```

Like `Compile` but panics if compilation fails. Useful for static expressions.

**Parameters**:

- `query`: JSONata expression string

**Returns**:

- `*types.Expression`: Compiled expression

**Panics**: If compilation fails

**Example**:

```go
var itemsQuery = gosonata.MustCompile("$.items[price > 100]")

func handler() {
    result, err := eval.Eval(ctx, itemsQuery, data)
    // ...
}
```

### Eval

```go
func Eval(query string, data interface{}, opts ...evaluator.EvalOption) (interface{}, error)
```

One-shot evaluation: compiles and evaluates an expression in a single call.

**Parameters**:

- `query`: JSONata expression string
- `data`: Input data (typically `map[string]interface{}` or `[]interface{}`)
- `opts`: Optional evaluator options

**Returns**:

- `interface{}`: Result value (use type assertion)
- `error`: Compilation or evaluation error

**Note**: Creates a new evaluator and context with 30-second timeout. For better performance with repeated queries, use `Compile` + evaluator.

**Example**:

```go
result, err := gosonata.Eval("$.name", data)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.(string))
```

### EvalWithContext

```go
func EvalWithContext(ctx context.Context, query string, data interface{}, opts ...evaluator.EvalOption) (interface{}, error)
```

Like `Eval` but with custom context for timeout and cancellation.

**Parameters**:

- `ctx`: Context for timeout and cancellation
- `query`: JSONata expression string
- `data`: Input data
- `opts`: Optional evaluator options

**Returns**:

- `interface{}`: Result value
- `error`: Compilation or evaluation error

**Example**:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := gosonata.EvalWithContext(ctx, "$.items", data)
```

### Version

```go
func Version() string
```

Returns the current version of GoSonata.

**Returns**:

- `string`: Version string (e.g., "v0.1.0-dev")

**Example**:

```go
fmt.Println("GoSonata version:", gosonata.Version())
```

### CustomFunc

```go
type CustomFunc = functions.CustomFunc
// i.e. func(ctx context.Context, args ...interface{}) (interface{}, error)
```

Type alias for user-defined functions. Re-exports `functions.CustomFunc`
so callers need not import `pkg/functions` directly.

### AdvancedCustomFunc

```go
type AdvancedCustomFunc = functions.AdvancedCustomFunc
// i.e. func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error)
```

Type alias for higher-order user-defined functions that need to call back into the
evaluator (e.g. to invoke a lambda argument). Re-exported from `pkg/functions`
so callers only need to import the top-level `gosonata` package.

### AdvancedCustomFunctionDef

```go
type AdvancedCustomFunctionDef = functions.AdvancedCustomFunctionDef
```

Type alias for the struct that bundles name, signature, and an `AdvancedCustomFunc`
implementation. Re-exported from `pkg/functions`.

### FunctionEntry

```go
type FunctionEntry = functions.FunctionEntry
```

Marker interface implemented by both `CustomFunctionDef` and
`AdvancedCustomFunctionDef`. Allows mixing both kinds in a single call to
[`WithFunctions`](#withfunctions).

### EvalStream (top-level)

```go
func EvalStream(ctx context.Context, query string, r io.Reader, opts ...EvalOption) (<-chan StreamResult, error)
```

Convenience wrapper: compiles `query` and calls `Evaluator.EvalStream`.
See [EvalStream (Evaluator)](#evalstream-evaluator) for full semantics.

**Example**:

```go
ch, err := gosonata.EvalStream(ctx, "$.name", os.Stdin)
for res := range ch {
    fmt.Println(res.Value)
}
```

### EvalBytes

```go
func EvalBytes(query string, raw json.RawMessage, opts ...evaluator.EvalOption) (interface{}, error)
```

Zero-copy evaluation directly against raw JSON bytes. Compiles `query` and delegates to
[`Evaluator.EvalBytes`](#evalbytes-evaluator).

For expressions that qualify for the **fast path** (pure paths, equality comparisons,
and 23 supported stdlib functions), no `json.Unmarshal` call is made on the input document.
Complex expressions fall back to full-AST evaluation automatically.

**Parameters**:

- `query`: JSONata expression string
- `raw`: Raw JSON input (`json.RawMessage` or `[]byte`)
- `opts`: Optional evaluator options

**Returns**:

- `interface{}`: Result value
- `error`: Compilation or evaluation error

**Example**:

```go
raw := json.RawMessage(`{"user":{"email":"alice@example.com","active":true}}`)

email, err := gosonata.EvalBytes("user.email", raw)
// → "alice@example.com"

active, err := gosonata.EvalBytes("$exists(user.email)", raw)
// → true
```

### EvalBytesWithContext

```go
func EvalBytesWithContext(ctx context.Context, query string, raw json.RawMessage, opts ...evaluator.EvalOption) (interface{}, error)
```

Like `EvalBytes` but with a custom context for timeout and cancellation.

**Example**:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
result, err := gosonata.EvalBytesWithContext(ctx, "user.role", raw)
```

### StreamResult (top-level)

```go
type StreamResult = evaluator.StreamResult
```

Type alias re-exported for callers that only import the `gosonata` package.

---

## Parser Package

The `parser` package handles lexical analysis and parsing of JSONata expressions.

### Parse

```go
func Parse(query string) (*types.Expression, error)
```

Parses a JSONata expression and returns the compiled Expression.

**Parameters**:

- `query`: JSONata expression string

**Returns**:

- `*types.Expression`: Compiled expression with AST
- `error`: Parsing error with position information

**Example**:

```go
import "github.com/sandrolain/gosonata/pkg/parser"

expr, err := parser.Parse("$.items[0].name")
if err != nil {
    fmt.Printf("Parse error: %v\n", err)
    return
}
```

### Compile (Parser)

```go
func Compile(query string, opts ...CompileOption) (*types.Expression, error)
```

Alias for `Parse` with optional compile-time configuration.

**Parameters**:

- `query`: JSONata expression string
- `opts`: Compilation options

**Returns**:

- `*types.Expression`: Compiled expression
- `error`: Compilation error

**Example**:

```go
expr, err := parser.Compile(query,
    parser.WithRecovery(true),
    parser.WithMaxDepth(200),
)
```

### CompileOption

#### WithRecovery

```go
func WithRecovery(enable bool) CompileOption
```

Enables error recovery mode for parsing invalid syntax.

**Parameters**:

- `enable`: Whether to enable recovery mode

**Default**: `false`

**Example**:

```go
expr, err := parser.Compile(query, parser.WithRecovery(true))
// Partial AST may be available even with errors
```

#### WithMaxDepth

```go
func WithMaxDepth(depth int) CompileOption
```

Sets the maximum parsing depth to prevent stack overflow.

**Parameters**:

- `depth`: Maximum recursion depth

**Default**: `100`

**Example**:

```go
expr, err := parser.Compile(query, parser.WithMaxDepth(200))
```

### Lexer

```go
type Lexer struct {
    // ... internal fields
}

func NewLexer(input string) *Lexer
func (l *Lexer) Next(allowRegex bool) Token
func (l *Lexer) Error() error
```

Low-level tokenizer. Typically not used directly; use `Parse` instead.

**Example**:

```go
lexer := parser.NewLexer("$.name")
for {
    token := lexer.Next(false)
    if token.Type == parser.TokenEOF {
        break
    }
    fmt.Printf("%v: %v\n", token.Type, token.Value)
}
```

---

## Evaluator Package

The `evaluator` package evaluates compiled expressions against data.

### Evaluator

```go
type Evaluator struct {
    // ... internal fields
}

func New(opts ...EvalOption) *Evaluator
func (e *Evaluator) Eval(ctx context.Context, expr *types.Expression, data interface{}) (interface{}, error)
```

Main evaluation engine.

**Example**:

```go
import "github.com/sandrolain/gosonata/pkg/evaluator"

eval := evaluator.New(
    evaluator.WithConcurrency(true),
    evaluator.WithMaxDepth(100),
)

result, err := eval.Eval(ctx, expr, data)
```

### New

```go
func New(opts ...EvalOption) *Evaluator
```

Creates a new evaluator with optional configuration.

**Parameters**:

- `opts`: Evaluator options

**Returns**:

- `*Evaluator`: Configured evaluator instance

**Default Configuration**:

- Caching: disabled
- Concurrency: enabled
- MaxDepth: 10000
- Timeout: 30 seconds

**Example**:

```go
eval := evaluator.New(
    evaluator.WithCaching(true),
    evaluator.WithTimeout(5*time.Second),
    evaluator.WithDebug(true),
)
```

### Eval (Evaluator)

```go
func (e *Evaluator) Eval(ctx context.Context, expr *types.Expression, data interface{}) (interface{}, error)
```

Evaluates a compiled expression against data.

**Parameters**:

- `ctx`: Context for timeout and cancellation
- `expr`: Compiled expression
- `data`: Input data

**Returns**:

- `interface{}`: Result value
- `error`: Evaluation error

**Example**:

```go
ctx := context.Background()
result, err := eval.Eval(ctx, expr, data)
if err != nil {
    log.Fatal(err)
}
```

### EvalOption

#### WithCaching

```go
func WithCaching(enabled bool) EvalOption
```

Enables result caching for repeated queries.

**Parameters**:

- `enabled`: Whether to enable caching

**Default**: `false`

**Example**:

```go
eval := evaluator.New(evaluator.WithCaching(true))
```

**Note**: When enabled, compiled expressions are cached in a lock-free FIFO cache (default 256
entries). Cache size can be tuned with `WithCacheSize`. The top-level `gosonata.Eval`
also benefits when `WithCaching(true)` is passed.

#### WithCacheSize

```go
func WithCacheSize(size int) EvalOption
```

Sets the maximum number of cached compiled expressions. Only meaningful when
`WithCaching(true)` is also set (or caching is otherwise enabled).

**Parameters**:

- `size`: Maximum number of entries in the cache

**Default**: `256`

**Example**:

```go
eval := evaluator.New(
    evaluator.WithCaching(true),
    evaluator.WithCacheSize(1024),
)
```

#### WithCache

```go
func WithCache(c cache.Cacher) EvalOption
```

Attaches an external cache implementing the `cache.Cacher` interface. The evaluator uses it regardless of
the `WithCaching` flag, enabling shared caches across multiple `Evaluator` instances.

**Parameters**:

- `c`: A `*cache.ExprCache` (or any `cache.Cacher` implementation) — see `pkg/cache`

**Example**:

```go
import "github.com/sandrolain/gosonata/pkg/cache"

sharedCache := cache.New(512)

eval1 := evaluator.New(evaluator.WithCache(sharedCache))
eval2 := evaluator.New(evaluator.WithCache(sharedCache))
```

#### WithConcurrency

```go
func WithConcurrency(enabled bool) EvalOption
```

Enables concurrent evaluation of independent expressions.

**Parameters**:

- `enabled`: Whether to enable concurrency

**Default**: `true`

**Example**:

```go
eval := evaluator.New(evaluator.WithConcurrency(false))
```

#### WithMaxDepth

```go
func WithMaxDepth(depth int) EvalOption
```

Sets the maximum evaluation depth to prevent stack overflow.

**Parameters**:

- `depth`: Maximum recursion depth

**Default**: `10000`

**Example**:

```go
eval := evaluator.New(evaluator.WithMaxDepth(200))
```

#### WithTimeout

```go
func WithTimeout(timeout time.Duration) EvalOption
```

Sets the evaluation timeout duration.

**Parameters**:

- `timeout`: Maximum evaluation time

**Default**: `30 * time.Second`

**Example**:

```go
eval := evaluator.New(evaluator.WithTimeout(5*time.Second))
```

#### WithDebug

```go
func WithDebug(enabled bool) EvalOption
```

Enables debug logging for evaluation steps.

**Parameters**:

- `enabled`: Whether to enable debug mode

**Default**: `false`

**Example**:

```go
eval := evaluator.New(evaluator.WithDebug(true))
```

#### WithLogger

```go
func WithLogger(logger *slog.Logger) EvalOption
```

Sets a custom structured logger.

**Parameters**:

- `logger`: Custom `log/slog` logger

**Default**: `slog.Default()`

**Example**:

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

eval := evaluator.New(evaluator.WithLogger(logger))
```

#### WithCustomFunction

```go
func WithCustomFunction(name, signature string, fn CustomFunc) EvalOption
```

Registers a user-defined function that can be called from JSONata expressions as
`$name(...)`. Custom functions are looked up **before** built-ins, so they can
override any built-in with the same name.

**Parameters**:

- `name`: Function name **without** the leading `$` (e.g. `"greet"` is called as `$greet(...)`)
- `signature`: Optional JSONata type-signature string (e.g. `"<s:s>"`, `""` to skip validation)
- `fn`: `func(ctx context.Context, args ...interface{}) (interface{}, error)`

**Default**: none

**Example**:

```go
result, err := gosonata.Eval(`$greet("World")`, nil,
    gosonata.WithCustomFunction("greet", "<s:s>",
        func(ctx context.Context, args ...interface{}) (interface{}, error) {
            return "Hello, " + args[0].(string) + "!", nil
        }),
)
// result == "Hello, World!"
```

Multiple functions can be registered by chaining the option:

```go
eval := evaluator.New(
    evaluator.WithCustomFunction("add", "<nn:n>", fnAdd),
    evaluator.WithCustomFunction("mul", "<nn:n>", fnMul),
)
```

### EvalWithBindings

```go
func (e *Evaluator) EvalWithBindings(ctx context.Context, expr *types.Expression, data interface{}, bindings map[string]interface{}) (interface{}, error)
```

Evaluates a compiled expression with per-call variable bindings. Useful when the same
expression is evaluated repeatedly against different data **and** needs different
`$variable` values each time.

**Parameters**:

- `ctx`: Context for timeout and cancellation
- `expr`: Compiled expression
- `data`: Input data (`$` root)
- `bindings`: Map of variable names (without `$`) to values

**Returns**:

- `interface{}`: Result value
- `error`: Evaluation error

**Example**:

```go
expr := gosonata.MustCompile("$.items[price > $threshold]")
ev := evaluator.New()
ctx := context.Background()

result1, _ := ev.EvalWithBindings(ctx, expr, data, map[string]interface{}{"threshold": 100.0})
result2, _ := ev.EvalWithBindings(ctx, expr, data, map[string]interface{}{"threshold": 200.0})
```

---

### EvalStream (Evaluator)

```go
func (e *Evaluator) EvalStream(ctx context.Context, expr *types.Expression, r io.Reader) (<-chan StreamResult, error)
```

Reads a sequence of JSON values from `r` (NDJSON / JSON-seq) and evaluates `expr`
against each document, sending results on the returned channel.

The channel is closed when all input is consumed or the context is cancelled.
A fatal I/O or JSON-decode error is sent as a `StreamResult` with `Err != nil`
and the channel is then closed. Per-document evaluation errors are sent
individually and the stream continues to the next document.

**Returns**: `(<-chan StreamResult, error)` — error is non-nil only if `expr` is nil.

**Example**:

```go
ch, err := eval.EvalStream(ctx, expr, os.Stdin)
for res := range ch {
    if res.Err != nil {
        log.Printf("error: %v", res.Err)
        continue
    }
    fmt.Println(res.Value)
}
```

### EvalBytes (Evaluator)

```go
func (e *Evaluator) EvalBytes(ctx context.Context, expr *types.Expression, raw json.RawMessage) (interface{}, error)
```

Evaluates a compiled expression directly against raw JSON bytes. For fast-path eligible
expressions the evaluation is zero-copy (no `json.Unmarshal` on the full document).
All other expressions fall back to full-AST evaluation automatically.

**Fast-path eligibility** is determined at compile time by `parser.Compile` via static AST
analysis. Call `expr.IsFastPath()` to check eligibility after compilation.

**Eligible patterns**:

| Pattern | Example |
|---|---|
| Pure dotted-path | `user.address.city` |
| Equality / inequality vs literal | `status = "active"`, `code != 0` |
| Supported stdlib function on path | `$exists(a.b)`, `$sum(totals)` |

**Supported fast-path functions**: `$exists`, `$not`, `$boolean`, `$string`, `$number`,
`$lowercase`, `$uppercase`, `$trim`, `$length`, `$type`, `$abs`, `$floor`, `$ceil`,
`$sqrt`, `$count`, `$sum`, `$max`, `$min`, `$average`, `$reverse`, `$keys`,
`$distinct`, `$contains`.

**Parameters**:

- `ctx`: Context for timeout and cancellation
- `expr`: Compiled expression (from `parser.Compile`)
- `raw`: Raw JSON input

**Returns**:

- `interface{}`: Result value
- `error`: Evaluation error

**Example**:

```go
expr, _ := parser.Compile("user.email")

ev := evaluator.New()
raw := json.RawMessage(`{"user":{"email":"bob@example.com"}}`)

result, err := ev.EvalBytes(context.Background(), expr, raw)
// → "bob@example.com"
```

### StreamResult

```go
type StreamResult struct {
    Value interface{} // result for one document, nil when Err is set
    Err   error       // non-nil on per-document or fatal I/O error
}
```

Holds the output of one NDJSON streaming evaluation step. Re-exported at the
top level as `gosonata.StreamResult`.

### EvalContext

```go
type EvalContext struct {
    // ... internal fields (all unexported)
}

func NewContext(data interface{}) *EvalContext
func (c *EvalContext) NewChildContext(data interface{}) *EvalContext
func (c *EvalContext) Data() interface{}
func (c *EvalContext) SetBinding(name string, value interface{})
func (c *EvalContext) GetBinding(name string) (interface{}, bool)
```

Evaluation context manages variable bindings and data scope. `bindings` is
initialised lazily (nil until the first `SetBinding` call), which eliminates
one map allocation per context in simple path-step evaluation.

**Example**:

```go
ctx := evaluator.NewContext(data)
ctx.SetBinding("myVar", 42)

value, ok := ctx.GetBinding("myVar")
if ok {
    fmt.Println(value) // 42
}
```

---

## Types Package

The `types` package defines core data structures.

### Expression

```go
type Expression struct {
    // ... internal fields
}

func (e *Expression) AST() *ASTNode
func (e *Expression) Source() string
func (e *Expression) Errors() []error
```

Represents a compiled JSONata expression.

> **Note**: `Expression` does not provide a self-contained `Eval()` method due to
> import-cycle constraints. Always use `evaluator.New().Eval(ctx, expr, data)`.

**Methods**:

#### AST

```go
func (e *Expression) AST() *ASTNode
```

Returns the Abstract Syntax Tree of the expression.

**Returns**:

- `*ASTNode`: Root node of the AST

**Example**:

```go
expr, _ := gosonata.Compile("$.name")
ast := expr.AST()
fmt.Println(ast.Type) // "path"
```

#### Source

```go
func (e *Expression) Source() string
```

Returns the original source string of the expression.

### ASTNode

```go
type ASTNode struct {
    Type        NodeType
    Value       interface{}
    Position    int
    LHS         *ASTNode
    RHS         *ASTNode
    Steps       []*ASTNode
    Arguments   []*ASTNode
    Expressions []*ASTNode
    KeepArray   bool
    ConsArray   bool
    Stage       string
    Index       int
    IsGrouping  bool
    Errors      []error
}

func NewASTNode(nodeType NodeType, position int) *ASTNode
func (n *ASTNode) String() string
```

Represents a node in the Abstract Syntax Tree.

**Fields**:

- `Type`: Node type identifier (e.g., `NodePath`, `NodeBinary`)
- `Value`: Literal value (for literals)
- `Position`: Character position in source
- `LHS`/`RHS`: Left/right operands (for binary operators)
- `Steps`: Path steps (for path navigation)
- `Arguments`: Function arguments
- `Expressions`: Block expressions
- `KeepArray`: Preserve array structure
- `ConsArray`: Force array construction
- `Stage`: Pipeline stage identifier
- `IsGrouping`: Object constructor semantics

**Example**:

```go
expr, _ := parser.Parse("$.items[0]")
root := expr.AST()

fmt.Println(root.Type)        // "path"
fmt.Println(len(root.Steps))  // 2
fmt.Println(root.Steps[0].Type) // "name" (for "items")
```

### NodeType

```go
type NodeType string

const (
    // Literals
    NodeString  NodeType = "string"
    NodeNumber  NodeType = "number"
    NodeBoolean NodeType = "value"
    NodeNull    NodeType = "value"

    // Navigation
    NodePath       NodeType = "path"
    NodeName       NodeType = "name"
    NodeWildcard   NodeType = "wildcard"
    NodeDescendant NodeType = "descendant"
    NodeParent     NodeType = "parent"

    // Operators
    NodeBinary NodeType = "binary"
    NodeUnary  NodeType = "unary"

    // Functions
    NodeFunction NodeType = "function"
    NodeLambda   NodeType = "lambda"
    NodePartial  NodeType = "partial"

    // ... and more
)
```

Node type identifiers.

### Error

```go
type Error struct {
    Code     ErrorCode
    Message  string
    Position int
    Token    string
    Err      error
}

func NewError(code ErrorCode, message string, position int) *Error
func (e *Error) Error() string
func (e *Error) Unwrap() error
func (e *Error) WithToken(token string) *Error
func (e *Error) WithCause(err error) *Error
```

Structured JSONata error with code and position.

**Example**:

```go
_, err := gosonata.Compile("$.name[")
if err != nil {
    if jsonataErr, ok := err.(*types.Error); ok {
        fmt.Printf("Error %s at position %d: %s\n",
            jsonataErr.Code,
            jsonataErr.Position,
            jsonataErr.Message)
    }
}
```

### ErrorCode

```go
type ErrorCode string

const (
    // S0xxx: Parser/Syntax errors
    ErrStringNotClosed   ErrorCode = "S0101"
    ErrSyntaxError       ErrorCode = "S0201"

    // T0xxx: Type errors
    ErrArgumentCountMismatch ErrorCode = "T0410"

    // D0xxx: Evaluation errors
    ErrInvokeNonFunction ErrorCode = "D1002"

    // U0xxx: Runtime errors
    ErrUndefinedVariable ErrorCode = "U1001"
    ErrUndefinedFunction ErrorCode = "U1002"
)
```

Standard JSONata error codes.

### Null

```go
type Null struct{}

var NullValue = Null{}

func (Null) MarshalJSON() ([]byte, error)
```

Represents JSONata `null` (distinct from `undefined`/`nil`).

> **Note**: `Eval()` converts `types.Null` to `nil` before returning, so both
> JSON `null` and JSONata `undefined` are returned as Go `nil` at the API boundary.
> `types.Null` is used only internally during evaluation.

---

## Functions Package

The `functions` package is the public extension point for user-defined (custom)
functions. All built-in JSONata functions are implemented directly inside
`pkg/evaluator`.

### CustomFunc

```go
type CustomFunc func(ctx context.Context, args ...interface{}) (interface{}, error)
```

Signature for user-defined functions.

### CustomFunctionDef

```go
type CustomFunctionDef struct {
    Name      string
    Signature string  // JSONata type-signature, e.g. "<s:s>" (empty = no validation)
    Fn        CustomFunc
}
```

Holds the definition of a single custom function. Passed to the evaluator via
`evaluator.WithCustomFunction(name, signature, fn)` (or the corresponding
`gosonata.WithCustomFunction` alias).

**Example**:

```go
import "github.com/sandrolain/gosonata/pkg/functions"

def := functions.CustomFunctionDef{
    Name:      "double",
    Signature: "<n:n>",
    Fn: func(ctx context.Context, args ...interface{}) (interface{}, error) {
        return args[0].(float64) * 2, nil
    },
}

---

## Extension Functions (`pkg/ext`)

GoSonata ships an optional library of extension functions that go beyond the official
JSONata 2.1.0+ specification. They are **off by default** and must be explicitly
registered via `WithFunctions` (or the `ext.*` category helpers).

### Import paths

```go
import "github.com/sandrolain/gosonata/pkg/ext"          // top-level helpers
import "github.com/sandrolain/gosonata/pkg/ext/extstring" // individual category
// …etc.
```

### `WithFunctions`

```go
func WithFunctions(defs ...functions.FunctionEntry) EvalOption
```

Registers any mix of `CustomFunctionDef` and `AdvancedCustomFunctionDef` in a single
variadic call. Both types implement `functions.FunctionEntry`, so you can spread
a typed slice directly.

Re-exported at the top level as `gosonata.WithFunctions`.

**Example — register all extensions at once**:

```go
import (
    "github.com/sandrolain/gosonata"
    "github.com/sandrolain/gosonata/pkg/ext"
)

result, err := gosonata.Eval(`$startsWith($.name, "Go")`, data,
    ext.WithAll(),
)
```

**Example — register by category**:

```go
result, err := gosonata.Eval(`$chunk(items, 3)`, data,
    ext.WithArray(),
    ext.WithString(),
)
```

**Example — spread a specific sub-package**:

```go
import extstring "github.com/sandrolain/gosonata/pkg/ext/extstring"

result, err := gosonata.Eval(`$camelCase("hello world")`, data,
    gosonata.WithFunctions(extstring.AllEntries()...),
)
```

### Category Helpers

| Helper | Sub-package | Description |
|--------|-------------|-------------|
| `ext.WithAll()` | `pkg/ext` | All extensions (simple + advanced HOF) |
| `ext.WithString()` | `extstring` | Extended string functions |
| `ext.WithNumeric()` | `extnumeric` | Extended numeric/statistical functions |
| `ext.WithArray()` | `extarray` | Extended array functions (incl. HOF) |
| `ext.WithObject()` | `extobject` | Extended object functions (incl. HOF) |
| `ext.WithTypes()` | `exttypes` | Type-predicate functions |
| `ext.WithDateTime()` | `extdatetime` | Extended date/time functions |
| `ext.WithCrypto()` | `extcrypto` | Cryptographic functions |
| `ext.WithFormat()` | `extformat` | Data-format functions (CSV, template) |
| `ext.WithFunctional()` | `extfunc` | Functional utilities (pipe, memoize) |

### Function Reference

#### `extstring` — Extended String Functions

| JSONata name | Signature | Description |
|---|---|---|
| `$startsWith(str, prefix)` | `<s-s:b>` | `true` if `str` begins with `prefix` |
| `$endsWith(str, suffix)` | `<s-s:b>` | `true` if `str` ends with `suffix` |
| `$indexOf(str, search [, start])` | `<s-s-n?:n>` | First index of `search`, or `-1` |
| `$lastIndexOf(str, search)` | `<s-s:n>` | Last index of `search`, or `-1` |
| `$capitalize(str)` | `<s:s>` | Capitalizes first character |
| `$titleCase(str)` | `<s:s>` | Title-cases every word |
| `$camelCase(str)` | `<s:s>` | Converts to camelCase |
| `$snakeCase(str)` | `<s:s>` | Converts to snake_case |
| `$kebabCase(str)` | `<s:s>` | Converts to kebab-case |
| `$repeat(str, n)` | `<s-n:s>` | Repeats `str` `n` times |
| `$words(str)` | `<s:a<s>>` | Splits string into array of words |
| `$template(tmpl, obj)` | `<s-o:s>` | Substitutes `{{key}}` placeholders from `obj` |

#### `extnumeric` — Extended Numeric & Statistical Functions

| JSONata name | Signature | Description |
|---|---|---|
| `$log(x [, base])` | `<n-n?:n>` | Logarithm; default base `e` |
| `$sign(x)` | `<n:n>` | Returns `-1`, `0`, or `1` |
| `$trunc(x)` | `<n:n>` | Truncates toward zero |
| `$clamp(x, min, max)` | `<n-n-n:n>` | Clamps `x` to `[min, max]` |
| `$sin(x)` | `<n:n>` | Sine (radians) |
| `$cos(x)` | `<n:n>` | Cosine (radians) |
| `$tan(x)` | `<n:n>` | Tangent (radians) |
| `$asin(x)` | `<n:n>` | Arc-sine |
| `$acos(x)` | `<n:n>` | Arc-cosine |
| `$atan(x)` | `<n:n>` | Arc-tangent |
| `$pi()` | `<:n>` | π constant |
| `$median(array)` | `<a<n>:n>` | Median value |
| `$variance(array)` | `<a<n>:n>` | Population variance |
| `$stddev(array)` | `<a<n>:n>` | Population standard deviation |
| `$percentile(array, p)` | `<a<n>-n:n>` | p-th percentile (0–100) |
| `$mode(array)` | `<a<n>:n>` | Most frequent value |

#### `extarray` — Extended Array Functions

Simple functions:

| JSONata name | Signature | Description |
|---|---|---|
| `$first(array)` | `<a:x>` | First element |
| `$last(array)` | `<a:x>` | Last element |
| `$take(array, n)` | `<a-n:a>` | First `n` elements |
| `$skip(array, n)` | `<a-n:a>` | All elements after the first `n` |
| `$slice(array, start [, end])` | `<a-n-n?:a>` | Sub-array (negative indices supported) |
| `$flatten(array [, depth])` | `<a-n?:a>` | Flattens nested arrays to `depth` (default: full) |
| `$chunk(array, size)` | `<a-n:a<a>>` | Splits into sub-arrays of `size` |
| `$union(arr1, arr2)` | `<a-a:a>` | Set union (deduped) |
| `$intersection(arr1, arr2)` | `<a-a:a>` | Set intersection |
| `$difference(arr1, arr2)` | `<a-a:a>` | Elements in `arr1` not in `arr2` |
| `$symmetricDifference(arr1, arr2)` | `<a-a:a>` | Elements in exactly one array |
| `$range(start, end [, step])` | `<n-n-n?:a<n>>` | Numeric range array |
| `$zipLongest(arr1, arr2, …)` | variadic | Zip, padding shorter arrays with `null` |
| `$window(array, size [, step])` | `<a-n-n?:a<a>>` | Sliding-window sub-arrays |

Advanced HOF (receive a key/comparator lambda):

| JSONata name | Description |
|---|---|
| `$groupBy(array, fn)` | Groups elements by key returned by `fn(value)` |
| `$countBy(array, fn)` | Counts elements per group key |
| `$sumBy(array, fn)` | Sums numeric `fn(value)` per group key |
| `$minBy(array, fn)` | Minimum `fn(value)` per group key |
| `$maxBy(array, fn)` | Maximum `fn(value)` per group key |
| `$accumulate(array, fn [, init])` | Running accumulation (returns all intermediate values) |

#### `extobject` — Extended Object Functions

Simple functions:

| JSONata name | Signature | Description |
|---|---|---|
| `$values(obj)` | `<o:a>` | Array of object values |
| `$pairs(obj)` | `<o:a<a>>` | Array of `[key, value]` pairs |
| `$fromPairs(pairs)` | `<a<a>:o>` | Builds object from `[[k,v],…]` |
| `$pick(obj, keys)` | `<o-a<s>:o>` | Keeps only the listed keys |
| `$omit(obj, keys)` | `<o-a<s>:o>` | Drops the listed keys |
| `$deepMerge(obj1, obj2)` | `<o-o:o>` | Recursive deep merge (right wins on conflict) |
| `$invert(obj)` | `<o:o>` | Swaps keys and values |
| `$size(obj)` | `<o:n>` | Number of own keys |
| `$rename(obj, from, to)` | `<o-s-s:o>` | Renames a key |

Advanced HOF:

| JSONata name | Description |
|---|---|
| `$mapValues(obj, fn)` | Transforms each value with `fn(value, key)` |
| `$mapKeys(obj, fn)` | Transforms each key with `fn(key, value)` |

#### `exttypes` — Type Predicates

| JSONata name | Signature | Description |
|---|---|---|
| `$isString(v)` | `<x:b>` | `true` if `v` is a string |
| `$isNumber(v)` | `<x:b>` | `true` if `v` is a number |
| `$isBoolean(v)` | `<x:b>` | `true` if `v` is a boolean |
| `$isArray(v)` | `<x:b>` | `true` if `v` is an array |
| `$isObject(v)` | `<x:b>` | `true` if `v` is an object |
| `$isNull(v)` | `<x:b>` | `true` if `v` is JSON `null` |
| `$isFunction(v)` | `<x:b>` | `true` if `v` is a function |
| `$isUndefined(v)` | `<x:b>` | `true` if `v` is `undefined` |
| `$isEmpty(v)` | `<x:b>` | `true` for `undefined`, `null`, `""`, `[]`, `{}` |
| `$default(v, fallback)` | `<x-x:x>` | Returns `v` if non-empty, otherwise `fallback` |
| `$identity(v)` | `<x:x>` | Returns `v` unchanged |

#### `extdatetime` — Extended Date/Time Functions

| JSONata name | Signature | Description |
|---|---|---|
| `$dateAdd(ts, amount, unit)` | `<s-n-s:s>` | Adds `amount` `unit` (year/month/day/hour/minute/second) to ISO timestamp |
| `$dateDiff(ts1, ts2, unit)` | `<s-s-s:n>` | Difference between two ISO timestamps in `unit` |
| `$dateComponents(ts)` | `<s:o>` | Decomposes ISO timestamp into `{year, month, day, hour, minute, second, ms, weekday, tz}` |
| `$dateStartOf(ts, unit)` | `<s-s:s>` | Start of the `unit` period containing `ts` |
| `$dateEndOf(ts, unit)` | `<s-s:s>` | End of the `unit` period containing `ts` |

#### `extcrypto` — Cryptographic Functions

| JSONata name | Signature | Description |
|---|---|---|
| `$uuid()` | `<:s>` | Generates a random UUID v4 |
| `$hash(data, algo)` | `<s-s:s>` | Hashes `data` with `algo` (md5/sha1/sha256/sha512); hex output |
| `$hmac(data, key, algo)` | `<s-s-s:s>` | HMAC of `data` with `key` and `algo`; hex output |

#### `extformat` — Data Format Functions

| JSONata name | Signature | Description |
|---|---|---|
| `$csv(text [, delimiter])` | `<s-s?:a<a>>` | Parses CSV text into array-of-arrays |
| `$toCSV(array [, delimiter])` | `<a-s?:s>` | Serialises array-of-arrays to CSV text |
| `$template(tmpl, obj)` | `<s-o:s>` | Go `text/template`-based substitution |

#### `extfunc` — Functional Utilities

| JSONata name | Description |
|---|---|
| `$pipe(value, fn1, fn2, …)` | Threads `value` through a sequence of functions |
| `$memoize(fn)` | Returns a memoized version of `fn` (caches results by args) |

### `AdvancedCustomFunctionDef`

For extension functions that need to call back into the evaluator (e.g. to invoke a
lambda argument passed by the caller), use `AdvancedCustomFunctionDef` instead of
`CustomFunctionDef`:

```go
import "github.com/sandrolain/gosonata/pkg/functions"

def := functions.AdvancedCustomFunctionDef{
    Name:      "myHOF",
    Signature: "<af:a>",
    Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
        arr := args[0].([]interface{})
        fn  := args[1] // *Lambda or *FunctionDef
        result := make([]interface{}, 0, len(arr))
        for _, item := range arr {
            v, err := caller.Call(ctx, fn, item)
            if err != nil {
                return nil, err
            }
            result = append(result, v)
        }
        return result, nil
    },
}
```

Both `CustomFunctionDef` and `AdvancedCustomFunctionDef` implement `functions.FunctionEntry`
and can be mixed in a single `gosonata.WithFunctions` call:

```go
gosonata.WithFunctions(
    append(extstring.AllEntries(), extfunc.AllEntries()...)...,
)
```

---

## Stream Package (`pkg/stream`)

The `pkg/stream` package provides `MultiEval`, a high-performance engine for evaluating
**N independent JSONata expressions against a continuous stream of JSON documents**.

It is designed for intensive message-processing pipelines (e.g. Kafka, NATS, file ingestion)
where the same set of expressions must fire on every incoming document as fast as possible.

### MultiEval

```go
type MultiEval struct { /* unexported */ }

func NewMultiEval(initial []*types.Expression, opts ...MultiEvalOption) *MultiEval
func NewMultiEvalWithObserver(initial []*types.Expression, hook EvalObserver, opts ...MultiEvalOption) *MultiEval
```

Holds a mutable, indexed list of compiled expressions and a built-in `EvalPlan` cache.
On each document, all requested expressions are resolved through a single set of fast-path
(gjson) extractions, minimising redundant JSON parsing.

#### Constructors

| Constructor | Usage |
|---|---|
| `NewMultiEval(initial, opts...)` | No metrics hook; uses `NoopObserver` |
| `NewMultiEvalWithObserver(initial, hook, opts...)` | Attach a custom `EvalObserver` |

#### MultiEvalOption

```go
func WithPlanCacheSize(n int) MultiEvalOption  // default: 256
func WithEvaluatorOptions(opts ...evaluator.EvalOption) MultiEvalOption
```

`WithPlanCacheSize` caps the in-memory `EvalPlan` cache size (FIFO eviction).
`WithEvaluatorOptions` passes evaluator options such as `WithCustomFunction`, `WithMaxDepth`
to the internal `*evaluator.Evaluator`.

#### Expression Management

```go
func (se *MultiEval) Compile(query string) (int, error)
func (se *MultiEval) Add(expr *types.Expression) int
func (se *MultiEval) Replace(idx int, query string) error
func (se *MultiEval) Remove(idx int) error
func (se *MultiEval) Reset()
func (se *MultiEval) Len() int
```

- **`Compile`**: Compiles `query` and appends it; returns the slot index.
- **`Add`**: Appends a pre-compiled expression; returns the slot index.
- **`Replace`**: Recompiles the expression at `idx` and invalidates the EvalPlan cache.
- **`Remove`**: Nils out the slot at `idx` (slot stays valid, future evals return `nil`).
- **`Reset`**: Clears all slots and the EvalPlan cache.

#### Evaluation

```go
func (se *MultiEval) Dispatch(
    ctx context.Context,
    rawJSON json.RawMessage,
    topicKey string,
    indices []int,
) ([]interface{}, error)

func (se *MultiEval) DispatchOne(
    ctx context.Context,
    rawJSON json.RawMessage,
    topicKey string,
    exprIndex int,
) (interface{}, error)
```

**`Dispatch`** evaluates the expressions at `indices` against `rawJSON` in one pass:

1. Looks up (or builds) a `EvalPlan` keyed by `(topicKey, indices fingerprint)`.
2. For fast-path slots: extracts the gjson result from a single `gjson.GetManyBytes` call.
3. For full-path slots: delegates to `evaluator.EvalBytes`.
4. Returns a `[]interface{}` aligned with `indices` (nil entry for removed slots).

`topicKey` is a caller-provided string that differentiates `EvalPlan` entries where
the same indices apply to documents with the same field structure. Use the same key for
all documents of the same schema/topic.

**`DispatchOne`** is a convenience wrapper: calls `Dispatch` with `[]int{exprIndex}` and
returns the single result.

**Example**:

```go
import (
    "context"
    "encoding/json"

    "github.com/sandrolain/gosonata/pkg/stream"
    gosonata "github.com/sandrolain/gosonata"
)

se := stream.NewMultiEval(nil)

// Register expressions once at start-up.
idxName, _ := se.Compile("Account.Name")
idxTotal, _ := se.Compile("order.total")
idxCount, _ := se.Compile("$count(items)")
idxFilter, _ := se.Compile("items[price > 20].name")

indices := []int{idxName, idxTotal, idxCount, idxFilter}
ctx := context.Background()

// Evaluate every incoming document.
for raw := range incomingDocuments {
    results, err := se.Dispatch(ctx, raw, "orders:v1", indices)
    if err != nil {
        log.Printf("eval error: %v", err)
        continue
    }
    fmt.Println(results[0], results[1], results[2], results[3])
}
```

#### Stats

```go
func (se *MultiEval) Stats() MultiEvalStats
```

### MultiEvalStats

```go
type MultiEvalStats struct {
    EvalCount     int64     // total Dispatch / DispatchOne calls
    FastPathCount int64     // evaluations served by fast path
    ErrorCount    int64     // evaluations that returned an error
    Plans         PlanStats // EvalPlan cache statistics
}

type PlanStats struct {
    Size      int
    Hits      int64
    Misses    int64
    Evictions int64
}
```

### EvalObserver

```go
type EvalObserver interface {
    OnEval(exprIndex int, fastPath bool, duration time.Duration, err error)
    OnPlanHit(planKey string)
    OnPlanMiss(planKey string)
    OnEviction()
}

type NoopObserver struct{}
```

`NoopObserver` implements `EvalObserver` with zero-overhead no-op methods; it is used
when `NewMultiEval` is called without a hook.

Implement `EvalObserver` to pipe telemetry into Prometheus, OpenTelemetry, or any other
monitoring system.

**Example**:

```go
type promHook struct {
    evalTotal    prometheus.Counter
    fastTotal    prometheus.Counter
    cacheHits    prometheus.Counter
    cacheMisses  prometheus.Counter
}

func (h *promHook) OnEval(idx int, fast bool, d time.Duration, err error) {
    h.evalTotal.Inc()
    if fast { h.fastTotal.Inc() }
}
func (h *promHook) OnPlanHit(key string)  { h.cacheHits.Inc() }
func (h *promHook) OnPlanMiss(key string) { h.cacheMisses.Inc() }
func (h *promHook) OnEviction()            { /* optional */ }

se := stream.NewMultiEvalWithObserver(nil, &promHook{...})
```

---

## Error Handling

### Error Types

GoSonata uses structured errors with codes and positions.

### Checking Error Types

```go
result, err := gosonata.Eval(query, data)
if err != nil {
    switch e := err.(type) {
    case *types.Error:
        // JSONata error with code and position
        fmt.Printf("Error %s at position %d: %s\n",
            e.Code, e.Position, e.Message)
    default:
        // Other error
        fmt.Printf("Error: %v\n", err)
    }
}
```

### Error Categories

| Category | Code Pattern | Description |
|----------|--------------|-------------|
| **Syntax** | `S0xxx` | Parsing and syntax errors |
| **Type** | `T0xxx` | Type conversion and validation errors |
| **Evaluation** | `D0xxx` | Runtime evaluation errors |
| **Runtime** | `U0xxx` | Undefined variables/functions |

### Common Error Codes

| Code | Description |
|------|-------------|
| `S0101` | String literal not closed |
| `S0102` | Number out of range |
| `S0201` | Syntax error |
| `S0202` | Expected token not found |
| `T0410` | Function argument count mismatch |
| `T1003` | Invalid type for operation |
| `D1002` | Attempted to invoke non-function |
| `U1001` | Undefined variable |
| `U1002` | Undefined function |

### Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := gosonata.EvalWithContext(ctx, query, data)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("Query timed out")
    } else if errors.Is(err, context.Canceled) {
        fmt.Println("Query was canceled")
    }
}
```

---

## Advanced Usage

### Custom Evaluator Configuration

```go
import (
    "log/slog"
    "os"
    "time"
)

// Create custom logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

// Configure evaluator
eval := evaluator.New(
    evaluator.WithLogger(logger),
    evaluator.WithDebug(true),
    evaluator.WithMaxDepth(200),
    evaluator.WithTimeout(10*time.Second),
    evaluator.WithConcurrency(true),
)

// Use evaluator
result, err := eval.Eval(ctx, expr, data)
```

### Working with Contexts

```go
// Create base context
baseCtx := context.Background()

// Add timeout
ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
defer cancel()

// Add custom values
ctx = context.WithValue(ctx, "requestID", "12345")

// Evaluate
result, err := eval.Eval(ctx, expr, data)
```

### Type Assertions

```go
result, err := gosonata.Eval(query, data)
if err != nil {
    log.Fatal(err)
}

switch v := result.(type) {
case string:
    fmt.Printf("String: %s\n", v)
case float64:
    fmt.Printf("Number: %f\n", v)
case bool:
    fmt.Printf("Boolean: %t\n", v)
case []interface{}:
    fmt.Printf("Array with %d items\n", len(v))
case map[string]interface{}:
    fmt.Printf("Object with %d keys\n", len(v))
case nil:
    fmt.Println("Undefined")
case types.Null:
    fmt.Println("Null")
default:
    fmt.Printf("Unknown type: %T\n", v)
}
```

### Handling Arrays and Objects

```go
// Array result
result, _ := gosonata.Eval("$.items", data)
if arr, ok := result.([]interface{}); ok {
    for i, item := range arr {
        fmt.Printf("Item %d: %v\n", i, item)
    }
}

// Object result
result, _ := gosonata.Eval("$.user", data)
if obj, ok := result.(map[string]interface{}); ok {
    for key, value := range obj {
        fmt.Printf("%s: %v\n", key, value)
    }
}
```

### Error Recovery

```go
// Parse with error recovery enabled
expr, err := parser.Compile(query, parser.WithRecovery(true))
if err != nil {
    // Check for partial AST
    if expr != nil && expr.AST() != nil {
        fmt.Println("Partial AST available despite errors")
        // Can inspect or process partial AST
    }
}
```

---

## Examples

### Example 1: Simple Path Navigation

```go
package main

import (
    "fmt"
    "log"

    "github.com/sandrolain/gosonata"
)

func main() {
    data := map[string]interface{}{
        "user": map[string]interface{}{
            "name": "Alice",
            "age":  28,
        },
    }

    result, err := gosonata.Eval("$.user.name", data)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result) // Output: Alice
}
```

### Example 2: Array Filtering

```go
data := map[string]interface{}{
    "items": []interface{}{
        map[string]interface{}{"name": "Item 1", "price": 50},
        map[string]interface{}{"name": "Item 2", "price": 150},
        map[string]interface{}{"name": "Item 3", "price": 200},
    },
}

query := "$.items[price > 100]"
result, err := gosonata.Eval(query, data)
if err != nil {
    log.Fatal(err)
}

items := result.([]interface{})
fmt.Printf("Found %d items over $100\n", len(items))
// Output: Found 2 items over $100
```

### Example 3: Aggregation

```go
data := map[string]interface{}{
    "numbers": []interface{}{1, 2, 3, 4, 5},
}

// Sum array
result, _ := gosonata.Eval("$sum($.numbers)", data)
fmt.Println(result) // Output: 15

// Count items
result, _ = gosonata.Eval("$count($.numbers)", data)
fmt.Println(result) // Output: 5

// Average
result, _ = gosonata.Eval("$average($.numbers)", data)
fmt.Println(result) // Output: 3
```

### Example 4: Reusable Expression with Variable Bindings

```go
// Compile once
expr := gosonata.MustCompile("$.items[price > $threshold]")
ev := evaluator.New()
ctx := context.Background()

// Use EvalWithBindings to pass per-call variable bindings
result1, _ := ev.EvalWithBindings(ctx, expr, data1, map[string]interface{}{"threshold": 100.0})
result2, _ := ev.EvalWithBindings(ctx, expr, data2, map[string]interface{}{"threshold": 200.0})
result3, _ := ev.EvalWithBindings(ctx, expr, data3, map[string]interface{}{"threshold": 50.0})
```

### Example 5: Timeout Handling

```go
import (
    "context"
    "time"
)

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

result, err := gosonata.EvalWithContext(ctx, complexQuery, largeData)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("Query timed out after 2 seconds")
        return
    }
    log.Fatal(err)
}
```

### Example 6: Debug Mode

```go
import "log/slog"

logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

eval := evaluator.New(
    evaluator.WithLogger(logger),
    evaluator.WithDebug(true),
)

// Evaluation will log detailed steps
result, err := eval.Eval(ctx, expr, data)
```

### Example 7: Object Construction

```go
data := map[string]interface{}{
    "firstName": "John",
    "lastName":  "Doe",
    "age":       30,
}

query := `{
    "fullName": $.firstName & " " & $.lastName,
    "adult": $.age >= 18
}`

result, err := gosonata.Eval(query, data)
// Output: {"fullName": "John Doe", "adult": true}
```

### Example 8: Higher-Order Functions

```go
data := map[string]interface{}{
    "numbers": []interface{}{1, 2, 3, 4, 5},
}

// Map
result, _ := gosonata.Eval("$map($.numbers, function($v) { $v * 2 })", data)
// Output: [2, 4, 6, 8, 10]

// Filter
result, _ = gosonata.Eval("$filter($.numbers, function($v) { $v > 2 })", data)
// Output: [3, 4, 5]

// Reduce
result, _ = gosonata.Eval("$reduce($.numbers, function($acc, $v) { $acc + $v }, 0)", data)
// Output: 15
```

---

## Performance Tips

### 1. Compile Once, Evaluate Many

```go
// ❌ Bad: Compile every time
for _, data := range datasets {
    result, _ := gosonata.Eval(query, data)
}

// ✅ Good: Compile once
expr := gosonata.MustCompile(query)
eval := evaluator.New()
for _, data := range datasets {
    result, _ := eval.Eval(ctx, expr, data)
}
```

### 2. Reuse Evaluator

```go
// ✅ Good: Reuse evaluator instance
eval := evaluator.New()
for _, query := range queries {
    expr, _ := gosonata.Compile(query)
    result, _ := eval.Eval(ctx, expr, data)
}
```

### 3. Use Context Timeout

```go
// Set reasonable timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := eval.Eval(ctx, expr, data)
```

### 4. Enable Concurrency

```go
// Default is enabled; only disable if needed
eval := evaluator.New(evaluator.WithConcurrency(true))
```

---

## Migration Guide

### From JavaScript JSONata

```javascript
// JavaScript
const jsonata = require('jsonata');
const expression = jsonata('$.name');
const result = expression.evaluate(data);
```

```go
// Go
import (
    "context"
    "github.com/sandrolain/gosonata"
    "github.com/sandrolain/gosonata/pkg/evaluator"
)

expr, _ := gosonata.Compile("$.name")
result, _ := evaluator.New().Eval(context.Background(), expr, data)
```

### From go-jsonata v206

```go
// Old (go-jsonata)
import "github.com/blues/jsonata-go"

e := jsonata.MustCompile(query)
result, _ := e.Eval(data)
```

```go
// New (GoSonata)
import "github.com/sandrolain/gosonata"

expr := gosonata.MustCompile(query)
eval := evaluator.New()
result, _ := eval.Eval(context.Background(), expr, data)
```

---

## API Stability

### Current Status (v0.1.0-dev)

⚠️ **Beta API**: Subject to change before v1.0.0

### Stable APIs

- Top-level functions (`Compile`, `Eval`, `EvalWithContext`, `MustCompile`, `EvalStream`)
- Top-level types: `CustomFunc`, `AdvancedCustomFunc`, `CustomFunctionDef`, `AdvancedCustomFunctionDef`, `FunctionEntry`, `EvalOption`, `StreamResult`
- Parser API (`Parse`, `Compile`)
- Evaluator: `New`, `Eval`, `EvalWithBindings`, `EvalStream`, `EvalBytes`
- Stream package: `NewMultiEval`, `NewMultiEvalWithObserver`, `Dispatch`, `DispatchOne`, `MultiEvalStats`, `EvalObserver`
- Core types (`Expression`, `ASTNode`, `Error`)
- Options: `WithCaching`, `WithCacheSize`, `WithCache`, `WithTimeout`, `WithConcurrency`, `WithDebug`, `WithLogger`, `WithMaxDepth`, `WithCustomFunction`, `WithFunctions`

### Planned Changes (Phase 8+)

- Plugin system
- OpenTelemetry integration

> **Note**: WebAssembly (js/wasm and WASI) is already implemented. See [docs/WASM.md](WASM.md).

---

## References

- [Architecture Documentation](ARCHITECTURE.md)
- [Differences from Other Implementations](DIFFERENCES.md)
- [JSONata Official Documentation](https://docs.jsonata.org/)
- [GitHub Repository](https://github.com/sandrolain/gosonata)

---

**Document Maintenance**: This document should be updated with each public API change. See [GitHub Copilot Instructions](../.github/copilot-instructions.md) for update procedures.
