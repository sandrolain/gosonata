# GoSonata — WebAssembly Integration

**Last Updated**: 2026-02-24

GoSonata can be compiled to [WebAssembly](https://webassembly.org/) for use in browsers, Node.js and other WASI runtimes. Two distinct build targets are provided:

| Target | `GOOS` | `GOARCH` | Best for |
|--|--|--|--|
| **js/wasm** | `js` | `wasm` | Browser, Node.js, Deno |
| **wasip1** | `wasip1` | `wasm` | wasmtime, WasmEdge, Wasmer, other WASI-compliant runtimes |

---

## Table of Contents

- [GoSonata — WebAssembly Integration](#gosonata--webassembly-integration)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
  - [Build](#build)
    - [Raw build commands](#raw-build-commands)
  - [js/wasm — Browser \& Node.js](#jswasm--browser--nodejs)
    - [JavaScript API](#javascript-api)
    - [Browser integration](#browser-integration)
      - [Minimal example (vanilla JS)](#minimal-example-vanilla-js)
      - [Using the gosonata\_wasm.js loader](#using-the-gosonata_wasmjs-loader)
    - [Node.js integration](#nodejs-integration)
  - [wasip1 — WASI runtimes](#wasip1--wasi-runtimes)
    - [Protocol](#protocol)
    - [wasmtime](#wasmtime)
    - [From Go or any language](#from-go-or-any-language)
  - [Performance notes](#performance-notes)
    - [Measured three-way comparison (Apple M2, Go 1.26, Node.js v24)](#measured-three-way-comparison-apple-m2-go-126-nodejs-v24)
    - [Reproduce the benchmark](#reproduce-the-benchmark)
  - [Comparison tests](#comparison-tests)
  - [Troubleshooting](#troubleshooting)
    - [`wasm_exec.js not found`](#wasm_execjs-not-found)
    - [`gosonata.wasm not found`](#gosonatawasm-not-found)
    - [`globalThis.gosonata is undefined` in Node.js](#globalthisgosonata-is-undefined-in-nodejs)
    - [WASM binary is very large (~5.4 MB)](#wasm-binary-is-very-large-54-mb)
    - [Node.js version](#nodejs-version)

---

## Prerequisites

- **Go 1.26+** (same as the main project)
- **Node.js ≥ v18** for js/wasm examples and comparison tests
- **wasmtime** for WASI testing (`brew install wasmtime` on macOS)

---

## Build

Use the provided Taskfile tasks (or the raw `go build` commands below):

```bash
# Build js/wasm target → cmd/wasm/js/gosonata.wasm
task wasm:build:js

# Build WASI target → cmd/wasm/wasi/gosonata.wasm
task wasm:build:wasi

# Build both targets at once
task wasm:build

# Copy wasm_exec.js to all directories that need it
task wasm:copy-support:js

# Quick smoke-test js/wasm with Node.js
task wasm:test:node

# Remove all built WASM binaries and copied support files
task wasm:clean
```

### Raw build commands

```bash
# js/wasm
GOOS=js GOARCH=wasm go build -o cmd/wasm/js/gosonata.wasm ./cmd/wasm/js/

# wasip1
GOOS=wasip1 GOARCH=wasm go build -o cmd/wasm/wasi/gosonata.wasm ./cmd/wasm/wasi/

# Copy wasm_exec.js from current Go toolchain
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" cmd/wasm/js/
```

---

## js/wasm — Browser & Node.js

### JavaScript API

After loading the WASM module the global `gosonata` object is exposed with:

```typescript
interface GoSonataWASM {
  /** Semantic version string, e.g. "v0.1.0-dev" */
  version(): string;

  /**
   * Evaluate a JSONata query against JSON-serialised data.
   * @param query  JSONata expression string
   * @param data   JSON string of the input data (null for no data)
   * @returns      JSON string of the result
   * @throws       string error message on evaluation failure
   */
  eval(query: string, dataJSON: string): string;

  /**
   * Compile a JSONata expression once for repeated evaluation.
   * @param query  JSONata expression string
   * @returns      Object with a single `eval(dataJSON)` method
   * @throws       string error message on parse failure
   */
  compile(query: string): { eval(dataJSON: string): string };
}
```

All return values are JSON-serialised strings: call `JSON.parse()` on the result.

### Browser integration

Copy these three files to the same directory served by your web server:

- `cmd/wasm/js/gosonata.wasm`
- `cmd/wasm/js/wasm_exec.js`
- `examples/wasm/browser/gosonata_wasm.js` (convenience loader, optional)

#### Minimal example (vanilla JS)

```html
<!DOCTYPE html>
<html>
<head><title>GoSonata WASM</title></head>
<body>
<script src="wasm_exec.js"></script>
<script>
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch('gosonata.wasm'), go.importObject)
    .then(({ instance }) => {
      go.run(instance);
      // The runtime registers `globalThis.gosonata` asynchronously.
      // Poll until it is ready.
      const poll = setInterval(() => {
        if (typeof globalThis.gosonata !== 'undefined') {
          clearInterval(poll);
          const result = JSON.parse(
            globalThis.gosonata.eval('$.name', JSON.stringify({ name: 'Alice' }))
          );
          console.log(result); // 'Alice'
        }
      }, 5);
    });
</script>
</body>
</html>
```

#### Using the gosonata_wasm.js loader

```html
<script src="wasm_exec.js"></script>
<script src="gosonata_wasm.js"></script>
<script>
  GoSonataWasm.load('gosonata.wasm').then(gs => {
    console.log(gs.version());
    const result = JSON.parse(gs.eval('$.x + $.y', JSON.stringify({ x: 1, y: 2 })));
    console.log(result); // 3
  });
</script>
```

### Node.js integration

Copy these files to your Node.js project:

- `cmd/wasm/js/gosonata.wasm`
- `cmd/wasm/js/wasm_exec.js`

```javascript
'use strict';
require('./wasm_exec.js');
const fs = require('fs');

async function loadGoSonata(wasmPath = './gosonata.wasm') {
  const go = new Go();
  const buf = fs.readFileSync(wasmPath);
  const { instance } = await WebAssembly.instantiate(buf, go.importObject);
  go.run(instance);
  await new Promise(r => setImmediate(r)); // let the Go runtime register the global
  return globalThis.gosonata;
}

(async () => {
  const gs = await loadGoSonata();
  console.log('version:', gs.version());

  // One-shot evaluation
  const result = JSON.parse(gs.eval('$.users[age > 25].name', JSON.stringify({
    users: [
      { name: 'Alice', age: 30 },
      { name: 'Bob',   age: 20 },
    ]
  })));
  console.log(result); // ['Alice']

  // Compile once, evaluate many times
  const compiled = gs.compile('$.users[age > 25].name');
  for (const dataset of datasets) {
    const r = JSON.parse(compiled.eval(JSON.stringify(dataset)));
    process.stdout.write(JSON.stringify(r) + '\n');
  }
})();
```

See [examples/wasm/node/example.js](../examples/wasm/node/example.js) for the complete runnable example.

---

## wasip1 — WASI runtimes

The WASI binary reads a single JSON request from stdin and writes a JSON response to stdout. This makes it easy to call from any language or shell script.

### Protocol

**Request** (stdin, single JSON object followed by newline):

```json
{"query": "<JSONata expression>", "data": <any JSON value>}
```

**Response** (stdout, single JSON object followed by newline):

```json
{"result": <any JSON value>}
```

On error:

```json
{"error": "<error message>"}
```

### wasmtime

```bash
# Simple eval
echo '{"query":"$.name","data":{"name":"Alice"}}' \
  | wasmtime cmd/wasm/wasi/gosonata.wasm
# → {"result":"Alice"}

# Filter + aggregate
echo '{"query":"$sum($.items.price)","data":{"items":[{"price":10},{"price":20}]}}' \
  | wasmtime cmd/wasm/wasi/gosonata.wasm
# → {"result":30}

# No input data
echo '{"query":"1 + 2 * 3"}' \
  | wasmtime cmd/wasm/wasi/gosonata.wasm
# → {"result":7}
```

### From Go or any language

```go
cmd := exec.Command("wasmtime", "gosonata.wasm")
cmd.Stdin = strings.NewReader(`{"query":"$.x","data":{"x":42}}`)
out, _ := cmd.Output()
// out → {"result":42}
```

---

## Performance notes

GoSonata WASM (js/wasm via Node.js) runs noticeably slower than native Go due to:

1. **Go runtime overhead**: The full Go runtime is bundled inside the WASM binary (~5.4 MB).
2. **syscall/js marshalling**: Every call crosses the Go↔JS boundary with string-serialised JSON.
3. **JIT warmup**: The V8 JIT needs several evaluations before reaching peak speed.
4. **No concurrency**: WASM runs single-threaded; `WithConcurrency(false)` is set automatically by `pkg/evaluator/evaluator_wasm.go`.

For production Go services always prefer the native Go API. WASM is the right choice when:

- you need JSONata in a browser application,
- you are integrating GoSonata into a non-Go environment,
- or you need a portable, sandboxed CLI via WASI.

### Measured three-way comparison (Apple M2, Go 1.26, Node.js v24)

Eval-only timings (expression pre-compiled). All three engines produce identical results (`TestWASMCorrectness`).

| Scenario | Go ns/op | JS ns/op | WASM ns/op | Go vs JS | Go vs WASM | JS vs WASM |
|--|--:|--:|--:|--:|--:|--:|
| SimplePath / 1 user | 736 | 945 | 19,695 | **1.3×** | **26.8×** | 20.9× |
| Filter / 10 users | 2,599 | 10,861 | 72,124 | **4.2×** | **27.8×** | 6.6× |
| Filter / 100 users | 18,836 | 121,162 | 531,210 | **6.4×** | **28.2×** | 4.4× |
| Aggregation / 10 users | 1,328 | 3,622 | 63,835 | **2.7×** | **48.1×** | 17.6× |
| Aggregation / 100 users | 5,226 | 19,861 | 445,592 | **3.8×** | **85.3×** | 22.4× |
| Transform / 10 users | 2,997 | 9,912 | 81,005 | **3.3×** | **27.0×** | 8.2× |
| Transform / 100 users | 15,444 | 48,063 | 520,219 | **3.1×** | **33.7×** | 10.8× |
| Sort / 10 users | 6,974 | 44,368 | 147,293 | **6.4×** | **21.1×** | 3.3× |
| Arithmetic | 1,000 | 2,700 | 18,379 | **2.7×** | **18.4×** | 6.8× |

> `Go vs JS` = JS ns/op ÷ Go ns/op. `Go vs WASM` = WASM ns/op ÷ Go ns/op. `JS vs WASM` = WASM ns/op ÷ JS ns/op. Higher = left engine is faster.

**Summary**: GoSonata native Go is ~2–85× faster than WASM, and JSONata JS is ~3–22× faster than WASM. Use WASM only when a native Go process is not an option.

### Reproduce the benchmark

```bash
task wasm:build:js
task wasm:copy-support:js
go test -run TestTripleComparison -v -count=1 ./tests/comparison/...
```

---

## Comparison tests

The `tests/comparison/wasm_comparison_test.go` file provides two test functions:

| Test | What it checks |
|--|--|
| `TestWASMCorrectness` | Go native = JSONata JS = GoSonata WASM (identical JSON output for 8 queries) |
| `TestTripleComparison` | Prints a performance table with Go, JS and WASM columns |

Both tests skip automatically when `cmd/wasm/js/gosonata.wasm` is not present.

```bash
# All WASM comparison tests
task test:comparison:wasm

# Correctness only
go test -run TestWASMCorrectness -v -count=1 ./tests/comparison/...

# Performance table only
go test -run TestTripleComparison -v -count=1 ./tests/comparison/...
```

---

## Troubleshooting

### `wasm_exec.js not found`

Run `task wasm:copy-support:js` to copy the file from `$(go env GOROOT)/lib/wasm/`.

### `gosonata.wasm not found`

Run `task wasm:build:js` (for js/wasm) or `task wasm:build:wasi` (for WASI).

### `globalThis.gosonata is undefined` in Node.js

The Go runtime registers the global asynchronously. Add `await new Promise(r => setImmediate(r))` after `go.run(instance)`.

### WASM binary is very large (~5.4 MB)

The Go runtime is statically linked. Strip debug info with:

```bash
GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o cmd/wasm/js/gosonata.wasm ./cmd/wasm/js/
```

This reduces the binary by ~30–40%. For further reduction consider running `wasm-opt` from [Binaryen](https://github.com/WebAssembly/binaryen).

### Node.js version

Requires Node.js ≥ 18 (WebAssembly.instantiate support). Tested on Node.js v24.

---

*See also*: [ARCHITECTURE.md](ARCHITECTURE.md), [API.md](API.md), [DIFFERENCES.md](DIFFERENCES.md)
