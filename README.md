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

<img src="gopher.png" width="320" />

## Features

- ✅ **High Performance**: Hand-written recursive descent parser optimized for speed
- ✅ **Concurrency**: Native goroutine support for parallel evaluation (enabled by default)
- ✅ **Streaming**: Efficient handling of large JSON documents
- ✅ **Spec Compliant**: Target 100% compatibility with JSONata 2.1.0+ specification
- ✅ **Type Safe**: Strongly typed with comprehensive error handling
- ✅ **Well Tested**: 1273/1273 official JSONata test suite cases passing (102 groups, 100%), plus 249 additional imported conformance tests
- ✅ **Production Ready**: DoS protection, resource limits, structured logging
- ✅ **WebAssembly**: Browser, Node.js (js/wasm) and WASI runtime support

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

### Project documentation (in `docs/`)

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - Package structure, core components, data flow, performance and concurrency model
- [docs/API.md](docs/API.md) - Public API reference, options, usage patterns and examples
- [docs/DIFFERENCES.md](docs/DIFFERENCES.md) - Implementation differences vs JS reference, known limitations and workarounds
- [docs/WASM.md](docs/WASM.md) - WebAssembly integration: browser, Node.js, WASI

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

All operations measured with a fresh compile (no cache). The `NodeArena` allocator pre-allocates
a 16 KB chunk per expression, which increases reported `B/op` but dramatically reduces `allocs/op`.

| Expression | ns/op | B/op | allocs/op | Δ allocs vs baseline |
|---|---|---|---|---|
| Simple path (`$.name`) | 2,254 | 16,728 | 8 | — |
| Complex path (`$.users[0].address.city`) | 3,020 | 16,920 | 21 | **−43%** |
| With functions (`$sum($.items.price)`) | 2,817 | 16,888 | 19 | **−42%** |
| Nested lambda | 3,210 | 16,944 | 23 | **−44%** |
| Object transformation | 3,548 | 16,992 | 26 | **−45%** |

> `B/op` is dominated by the NodeArena 16 KB pre-allocation (64-node bump-pointer chunk).
> Total live memory per expression is similar; the arena just batches allocations into one.

### Evaluator benchmarks — pre-compiled expression

| Scenario | ns/op | B/op | allocs/op | Δ ns/op vs baseline |
|---|---|---|---|---|
| Simple path (1 user) | 533 | 568 | 9 | **−9%** |
| Filter (10 users) | 793 | 632 | 10 | **−10%** |
| Filter (100 users) | 847 | 632 | 10 | **−15%** |
| Filter (1000 users) | 849 | 632 | 10 | **−17%** |
| Aggregation (100 users) | 826 | 648 | 11 | **−18%** |
| Transform (100 users) | 1,530 | 1,856 | 30 | **−11%** |
| Sort (100 users) | 767 | 808 | 18 | **−11%** |
| Arithmetic expression | 793 | 544 | 13 | **−18%** |
| Concurrent eval (100 users) | 329 | 632 | 10 | **−15%** |

> Filter, aggregation and sort evaluation cost stays nearly constant from 10 to 1000 items thanks to lazy path resolution.
> Baseline: initial implementation before the OPT-01…OPT-14 optimization sprint (2026-02-26).

### GoSonata vs JSONata JS (reference implementation)

Eval-only comparison (expression pre-compiled on both sides, data in native format).
Each scenario is verified to produce identical results in both engines (`TestResultCorrectness`).
GoSonata numbers updated after the OPT-01…OPT-14 sprint (2026-02-26); JS numbers unchanged.

| Scenario | GoSonata ns/op | JSONata JS ns/op | Speedup |
|---|---|---|---|
| SimplePath / 1 user | ~770 | ~1,420 | **~1.8×** |
| Filter / 10 users | ~2,900 | ~10,940 | **~3.8×** |
| Filter / 100 users | ~15,700 | ~104,400 | **~6.6×** |
| Filter / 1000 users | ~143,000 | ~960,100 | **~6.7×** |
| Aggregation / 10 users | ~1,300 | ~3,480 | **~2.7×** |
| Aggregation / 100 users | ~4,100 | ~19,280 | **~4.7×** |
| Transform / 10 users | ~2,400 | ~10,070 | **~4.2×** |
| Transform / 100 users | ~13,300 | ~45,760 | **~3.4×** |
| Sort / 10 users | ~5,900 | ~44,300 | **~7.5×** |
| Sort / 100 users | ~63,000 | ~823,100 | **~13.1×** |
| Arithmetic | ~840 | ~2,610 | **~3.1×** |

> JSONata JS timings measured within a single persistent Node.js process (no startup cost).
> JS `evaluate()` is inherently async (Promise); Go is synchronous — the async overhead
> is included since it is unavoidable in real JS usage.
> GoSonata estimates scaled from pre-sprint baseline using the per-scenario improvement
> ratios measured in `tests/benchmark`. Run `task bench:comparison:report` to regenerate exact values.

## WebAssembly

GoSonata can be compiled to WebAssembly for use in browsers, Node.js, and WASI runtimes.

### Build

```bash
# Build browser / Node.js target (GOOS=js GOARCH=wasm)
task wasm:build:js

# Build WASI runtime target (GOOS=wasip1 GOARCH=wasm)
task wasm:build:wasi

# Copy wasm_exec.js support file
task wasm:copy-support:js
```

### Use in Node.js

```javascript
require('./wasm_exec.js');
const fs = require('fs');
const go = new Go();
const { instance } = await WebAssembly.instantiate(
  fs.readFileSync('./gosonata.wasm'), go.importObject
);
go.run(instance);
const result = JSON.parse(globalThis.gosonata.eval(
  '$.users[age > 25].name',
  JSON.stringify({ users: [...] })
));
```

### Use in Browser

```html
<script src="wasm_exec.js"></script>
<script>
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch('gosonata.wasm'), go.importObject)
    .then(({ instance }) => {
      go.run(instance);
      const result = JSON.parse(
        globalThis.gosonata.eval('$.name', JSON.stringify({ name: 'Alice' }))
      );
    });
</script>
```

### Use via WASI (stdin/stdout JSON protocol)

```bash
echo '{"query":"$.name","data":{"name":"Alice"}}' | wasmtime cmd/wasm/wasi/gosonata.wasm
# → {"result":"Alice"}
```

See [docs/WASM.md](docs/WASM.md) and [examples/wasm/](examples/wasm/) for full documentation.

### WASM performance

GoSonata WASM runs via the Go runtime embedded in the binary. Measured on Apple M2, Go 1.26, Node.js v24:

| Scenario | Go ns/op | JS ns/op | WASM ns/op |
|--|--:|--:|--:|
| SimplePath | 736 | 945 | 19,695 |
| Filter / 10 users | 2,599 | 10,861 | 72,124 |
| Filter / 100 users | 18,836 | 121,162 | 531,210 |
| Aggregation / 100 users | 5,226 | 19,861 | 445,592 |
| Sort / 10 users | 6,974 | 44,368 | 147,293 |

Native Go is ~18–85× faster than WASM. Use WASM for browser/non-Go environments; prefer native Go for backend services.

---

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
