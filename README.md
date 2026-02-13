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

> **Status**: 🚧 Under active development - Phase 0 (Setup) complete

## Features

- ✅ **High Performance**: Hand-written recursive descent parser optimized for speed
- ✅ **Concurrency**: Native goroutine support for parallel evaluation (enabled by default)
- ✅ **Streaming**: Efficient handling of large JSON documents
- ✅ **Spec Compliant**: Target 100% compatibility with JSONata 2.1.0+ specification
- ✅ **Type Safe**: Strongly typed with comprehensive error handling
- ✅ **Well Tested**: 90%+ code coverage target, full conformance test suite
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

// Evaluate against different data
result1, _ := expr.Eval(ctx, data1)
result2, _ := expr.Eval(ctx, data2)
result3, _ := expr.Eval(ctx, data3)
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

# Run conformance tests (JSONata test suite)
task test:conformance
```

## Performance

GoSonata is designed for high performance:

```bash
# Run benchmarks
task bench

# Parser benchmarks
task bench:parser

# Evaluator benchmarks
task bench:evaluator
```

**Target Performance** (vs JavaScript reference):

- Parsing: 5x faster
- Evaluation: 10x faster
- Memory: 50% less allocation

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
