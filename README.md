# GoSonata

[![Go Version](https://img.shields.io/github/go-mod/go-version/sandrolain/gosonata)](https://go.dev/doc/go1.26)
[![GoDoc](https://godoc.org/github.com/sandrolain/gosonata?status.svg)](https://godoc.org/github.com/sandrolain/gosonata)
<!--[![Test](https://github.com/sandrolain/gosonata/workflows/Test/badge.svg)](https://github.com/sandrolain/gosonata/actions?query=workflow%3ATest)
[![Lint](https://github.com/sandrolain/gosonata/workflows/Lint/badge.svg)](https://github.com/sandrolain/gosonata/actions?query=workflow%3ALint)
[![Security](https://github.com/sandrolain/gosonata/workflows/Security/badge.svg)](https://github.com/sandrolain/gosonata/actions?query=workflow%3ASecurity)
[![codecov](https://codecov.io/gh/sandrolain/gosonata/branch/main/graph/badge.svg)](https://codecov.io/gh/sandrolain/gosonata)-->
[![Go Report Card](https://goreportcard.com/badge/github.com/sandrolain/gosonata)](https://goreportcard.com/report/github.com/sandrolain/gosonata)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance Go implementation of [JSONata](https://jsonata.org/) 2.1.0+, designed for intensive data streaming scenarios.

> **Status**: ✅ Conformance complete — 1273/1273 official JSONata test suite cases + 249 imported conformance tests passing (100%)

## Features

- ✅ **High Performance**: Hand-written recursive descent parser optimized for speed
- ✅ **Concurrency**: Native goroutine support for parallel evaluation (enabled by default)
- ✅ **Streaming**: Efficient handling of large JSON documents
- ✅ **Spec Compliant**: Target 100% compatibility with JSONata 2.1.0+ specification
- ✅ **Type Safe**: Strongly typed with comprehensive error handling
- ✅ **Well Tested**: 1273/1273 official JSONata test suite cases passing (102 groups, 100%), plus 249 additional imported conformance tests
- ✅ **Production Ready**: DoS protection, resource limits, structured logging

## What is JSONata?

JSONata is a lightweight query and transformation language for JSON data. It allows you to:

- Extract data from complex JSON structures
- Transform data with powerful expressions
- Combine, filter, sort and aggregate data
- Perform calculations and string manipulations

## Installation

```bash
go get github.com/sandrolain/gosonata
```

**Requirements**: Go 1.26 or later

## Quick Start

### Simple Evaluation

```go
package main

import (
    "fmt"
    "log"

    "github.com/sandrolain/gosonata"
)

func main() {
    data := map[string]interface{}{
        "name": "John",
        "age":  30,
    }

    result, err := gosonata.Eval("$.name", data)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result) // Output: John
}
```

### Compile Once, Evaluate Many Times

For better performance when evaluating the same expression multiple times:

```go
// Compile the expression once
expr, err := gosonata.Compile("$.items[price > 100]")
if err != nil {
    log.Fatal(err)
}

ev := evaluator.New()
ctx := context.Background()

// Evaluate against different data
result1, _ := ev.Eval(ctx, expr, data1)
result2, _ := ev.Eval(ctx, expr, data2)
result3, _ := ev.Eval(ctx, expr, data3)
```

### With Options

```go
result, err := gosonata.Eval("$.items", data,
    gosonata.WithCaching(true),
    gosonata.WithConcurrency(false),
    gosonata.WithTimeout(5*time.Second),
    gosonata.WithDebug(true),
)
```

## Examples

### Extract Data

```go
// Get all product names
result, _ := gosonata.Eval("$.products.name", data)

// Get names of products over $100
result, _ := gosonata.Eval("$.products[price > 100].name", data)
```

### Transform Data

```go
// Create new structure
query := `{
    "total": $sum(items.price),
    "count": $count(items),
    "names": items.name
}`
result, _ := gosonata.Eval(query, data)
```

### Aggregate Data

```go
// Calculate total and average
query := `{
    "total": $sum(sales.amount),
    "average": $average(sales.amount),
    "max": $max(sales.amount)
}`
result, _ := gosonata.Eval(query, data)
```

For more examples, see the [examples/](examples/) directory.

## Documentation

- [API Documentation](https://godoc.org/github.com/sandrolain/gosonata) - Complete GoDoc reference
- [JSONata Documentation](https://jsonata.org/) - Official JSONata language documentation

## Testing

Run the tests:

```bash
# All tests
task test

# Unit tests only
task test:unit

# With coverage
task coverage

# Run conformance tests (official JSONata test suite — 1273 cases in 102 groups)
task test:conformance

# Run GoSonata vs JSONata JS comparison report
task bench:comparison:report
```

## Performance

GoSonata is designed for high performance. All benchmarks run on Apple M2 (arm64), Go 1.26.

```bash
# Run all benchmarks
task bench

# Parser benchmarks
task bench:parse

# Evaluator benchmarks
task bench:eval

# GoSonata vs JSONata JS comparison report
task bench:comparison:report
```

### Parser benchmarks

| Expression | ns/op | B/op | allocs/op |
|---|---|---|---|
| Simple path (`$.name`) | 388 | 880 | 8 |
| Complex path (`$.users[0].address.city`) | 2,462 | 4,400 | 37 |
| With functions (`$sum($.items.price)`) | 2,129 | 3,952 | 33 |
| Nested lambda | 2,808 | 4,840 | 41 |
| Object transformation | 3,333 | 5,512 | 47 |

### Evaluator benchmarks — pre-compiled expression

| Scenario | ns/op | B/op | allocs/op |
|---|---|---|---|
| Simple path (1 user) | 699 | 664 | 11 |
| Filter (10 users) | 1,111 | 872 | 15 |
| Filter (100 users) | 1,234 | 968 | 17 |
| Filter (1000 users) | 1,241 | 968 | 17 |
| Aggregation (100 users) | 1,250 | 936 | 17 |
| Transform (100 users) | 2,132 | 2,240 | 38 |
| Sort (100 users) | 1,045 | 952 | 21 |
| Arithmetic expression | 1,168 | 832 | 19 |
| Concurrent eval (100 users) | 431 | 872 | 15 |

> Filter, aggregation and sort evaluation cost stays nearly constant from 10 to 1000 items thanks to lazy path resolution.

### GoSonata vs JSONata JS (reference implementation)

Eval-only comparison (expression pre-compiled on both sides, data in native format).
Each scenario is verified to produce identical results in both engines (`TestResultCorrectness`).

| Scenario | GoSonata ns/op | JSONata JS ns/op | Speedup |
|---|---|---|---|
| SimplePath / 1 user | ~730 | ~1,600 | **~2×** |
| Filter / 10 users | ~3,600 | ~12,500 | **~3.4×** |
| Filter / 100 users | ~29,000 | ~120,000 | **~4.2×** |
| Filter / 1000 users | ~317,000 | ~1,180,000 | **~3.7×** |
| Aggregation / 10 users | ~2,000 | ~4,500 | **~2.3×** |
| Aggregation / 100 users | ~9,900 | ~23,400 | **~2.4×** |
| Transform / 10 users | ~4,600 | ~12,300 | **~2.7×** |
| Transform / 100 users | ~22,700 | ~48,600 | **~2.1×** |
| Sort / 10 users | ~7,700 | ~50,300 | **~6.6×** |
| Sort / 100 users | ~107,000 | ~985,000 | **~9.2×** |
| Arithmetic | ~1,300 | ~3,200 | **~2.5×** |

> JSONata JS timings measured within a single persistent Node.js process (no startup cost).
> JS `evaluate()` is inherently async (Promise); Go is synchronous — the async overhead
> is included since it is unavoidable in real JS usage.

## Security

Security is a priority. GoSonata includes:

- DoS protection (depth limits, timeouts, range limits)
- Input validation and sanitization
- Resource limits and monitoring
- Regular security scans (gosec, trivy)

To run security scans:

```bash
task security
```

## License

[MIT License](LICENSE) - Copyright (c) 2026 Sandro Lain

## Acknowledgments

- [JSONata](https://jsonata.org/) - Original specification and reference implementation
- [jsonata-js](https://github.com/jsonata-js/jsonata) - JavaScript reference implementation
- [go-jsonata](https://github.com/jsonata-go/jsonata) - Go transliteration inspiration

## Links

- [JSONata Official Site](https://jsonata.org/)
- [JSONata Try](https://try.jsonata.org/) - Interactive playground
- [JSONata Exerciser](https://jsonata.org/exerciser) - Test your expressions
- [GitHub Repository](https://github.com/sandrolain/gosonata)
- [Issue Tracker](https://github.com/sandrolain/gosonata/issues)

## Support

- 📖 [Documentation](https://godoc.org/github.com/sandrolain/gosonata)
- 🐛 [Issue Tracker](https://github.com/sandrolain/gosonata/issues)

---

Made with ❤️ for the Go community
