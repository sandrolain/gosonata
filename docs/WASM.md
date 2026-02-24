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
    - [Measured four-way comparison (Apple M2, Go 1.26, Node.js v24)](#measured-four-way-comparison-apple-m2-go-126-nodejs-v24)
    - [Reproduce the benchmark](#reproduce-the-benchmark)
  - [wazero in-process runtime](#wazero-in-process-runtime)
    - [When to use wazero](#when-to-use-wazero)
    - [How it works](#how-it-works)
    - [Performance characteristics](#performance-characteristics)
    - [Build the wasip1 binary](#build-the-wasip1-binary)
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

### Measured four-way comparison (Apple M2, Go 1.26, Node.js v24)

Eval-only timings (expression pre-compiled). Native Go, JS and WASM(Node) produce identical results (`TestWASMCorrectness`). Wazero matches native Go (`TestWazeroCorrectness`).

The wazero runtime and AOT module compilation happen **once** in `TestMain` before any test runs (see [Comparison tests](#comparison-tests)). The ns/op figures below reflect only the per-eval `InstantiateModule` cost.

| Scenario | Go ns/op | JS ns/op | WASM(Node) ns/op | Wazero ns/op | Go vs JS | Go vs WASM | Go vs Wazero |
|--|--:|--:|--:|--:|--:|--:|--:|
| SimplePath / 1 user | 446 | 887 | 19,142 | 1,983,476 | **2.0×** | **42.9×** | 4,448× |
| Filter / 10 users | 2,587 | 11,162 | 75,568 | 2,117,868 | **4.3×** | **29.2×** | 819× |
| Filter / 100 users | 18,798 | 100,606 | 530,894 | 2,994,175 | **5.4×** | **28.2×** | 159× |
| Aggregation / 10 users | 1,175 | 3,564 | 64,519 | 2,100,250 | **3.0×** | **54.9×** | 1,787× |
| Aggregation / 100 users | 5,345 | 19,591 | 453,608 | 2,861,945 | **3.7×** | **84.9×** | 535× |
| Transform / 10 users | 3,075 | 10,145 | 81,720 | 2,197,427 | **3.3×** | **26.6×** | 715× |
| Transform / 100 users | 16,945 | 49,011 | 538,535 | 3,099,788 | **2.9×** | **31.8×** | 183× |
| Sort / 10 users | 6,919 | 44,309 | 147,682 | 2,328,037 | **6.4×** | **21.3×** | 336× |
| Arithmetic | 1,139 | 2,809 | 19,219 | 1,980,541 | **2.5×** | **16.9×** | 1,739× |

> Higher multiplier = left engine is faster. Wazero ns/op is the per-call `InstantiateModule` cost (~2 ms), dominated by the Go WASM runtime's own startup (single-shot protocol). The one-time AOT compile is ~1.7 s and runs in `TestMain`.

**Summary**: GoSonata native Go is 17–85× faster than WASM(Node), and WASM(Node) via Node.js is 5–104× faster than Wazero in the single-shot benchmark. See the [wazero section](#wazero-in-process-runtime) for details and when to use each approach.

### Reproduce the benchmark

```bash
task wasm:build:js
task wasm:build:wasi
task wasm:copy-support:js
go test -run TestTripleComparison -v -count=1 ./tests/comparison/...  # 3-way
go test -run TestQuadComparison   -v -count=1 ./tests/comparison/...  # 4-way
```

---

## wazero in-process runtime

[wazero](https://github.com/tetratelabs/wazero) is a pure-Go WebAssembly runtime (no CGO, no external dependencies) that can execute the GoSonata `wasip1` binary completely in-process — **no Node.js subprocess required**.

### When to use wazero

| Scenario | Recommended |
|--|--|
| Production Go service evaluating thousands of expressions | Native Go API |
| Browser / non-Go environment | js/wasm via Node.js or bundler |
| Occasional evaluation in Go without importing gosonata | **wazero + wasip1 binary** |
| Sandboxed, untrusted expression evaluation | **wazero + wasip1 binary** |
| Build constraints prevent importing gosonata directly | **wazero + wasip1 binary** |

### How it works

The `wasip1` binary reads `{"query":"…","data":…}` from stdin and writes `{"result":…}` or `{"error":"…"}` to stdout. wazero pipes these in-process:

```go
import (
    "bytes"
    "context"
    "encoding/json"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
    wazeroSys "github.com/tetratelabs/wazero/sys"
)

// Compile once, re-instantiate per eval.
ctx := context.Background()
r := wazero.NewRuntime(ctx)
defer r.Close(ctx)

wasi_snapshot_preview1.Instantiate(ctx, r)
wasmBytes, _ := os.ReadFile("gosonata.wasm") // wasip1 binary
compiled, _ := r.CompileModule(ctx, wasmBytes)

// Per evaluation:
payload, _ := json.Marshal(map[string]any{"query": "$.name", "data": data})

var stdout bytes.Buffer
modConfig := wazero.NewModuleConfig().
    WithStdin(bytes.NewReader(payload)).
    WithStdout(&stdout).
    WithArgs("gosonata").
    WithName("") // anonymous — required for multiple instantiations

_, err := r.InstantiateModule(ctx, compiled, modConfig)
if err != nil {
    var exitErr *wazeroSys.ExitError
    if !errors.As(err, &exitErr) || exitErr.ExitCode() != 0 {
        log.Fatal(err)
    }
    // exit code 0 is normal for wasip1 programs
}

var result struct {
    Result json.RawMessage `json:"result"`
    Error  string          `json:"error"`
}
json.Unmarshal(stdout.Bytes(), &result)
```

### Performance characteristics

Each `InstantiateModule` call starts the Go WASM runtime from scratch. This costs ~2 ms on Apple M2, regardless of expression complexity. This overhead dominates for simple expressions but becomes proportionally smaller for large datasets.

The one-time `r.CompileModule` (AOT compilation) costs ~1.7 s. In the comparison test suite this is done in `TestMain` **before** any test function runs, so it does not distort individual test timings.

**Optimising wazero throughput:**

- **Amortise startup**: call `r.CompileModule` once at program startup, then reuse the `CompiledModule` for every eval.
- **Batch queries**: modify the `wasip1` binary to accept a JSON array of `{query, data}` entries per invocation — one module instantiation for many queries.
- **Parallelism**: instantiate multiple modules concurrently from separate goroutines, each with its own `bytes.Buffer` for stdin/stdout.

### Build the wasip1 binary

```bash
task wasm:build:wasi
# → cmd/wasm/wasi/gosonata.wasm (5.4 MB)
```

---

## Comparison tests

The `tests/comparison/wasm_comparison_test.go` file provides four test functions:

| Test | What it checks |
|--|--|
| `TestWASMCorrectness` | Go native = JSONata JS = GoSonata WASM(Node) (identical JSON output for 8 queries) |
| `TestWazeroCorrectness` | Go native = GoSonata WASM(wazero) (identical JSON output for 8 queries) |
| `TestTripleComparison` | Performance table: Go, JS, WASM(Node) (3 columns) |
| `TestQuadComparison` | Performance table: Go, JS, WASM(Node), Wazero (4 columns) |

`TestWASMCorrectness` and `TestTripleComparison` skip when `cmd/wasm/js/gosonata.wasm` is absent.
`TestWazeroCorrectness` and `TestQuadComparison` skip when `cmd/wasm/wasi/gosonata.wasm` is absent.

**`TestMain`**: a package-level `TestMain` runs before every test function. If the `wasip1` binary is present it initialises the wazero `Runtime` and AOT-compiles the module (~1.7 s); the runtime stays open for the entire test run and is closed cleanly on exit. Individual test functions therefore pay only the per-eval `InstantiateModule` cost, not the compile cost.

```bash
# Build required binaries
task wasm:build:js
task wasm:build:wasi
task wasm:copy-support:js

# Run all WASM comparison tests
task test:comparison:wasm

# Individual tests
go test -run TestWASMCorrectness    -v -count=1 ./tests/comparison/...
go test -run TestWazeroCorrectness  -v -count=1 ./tests/comparison/...
go test -run TestTripleComparison   -v -count=1 ./tests/comparison/...
go test -run TestQuadComparison     -v -count=1 ./tests/comparison/...
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
