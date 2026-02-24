#!/usr/bin/env node
/**
 * GoSonata WASM — Node.js example
 * =====================================
 * Prerequisites (run from project root):
 *   task wasm:build:js
 *   task wasm:copy-support:js
 *
 * Then run this file:
 *   node examples/wasm/node/example.js
 */

"use strict";

const path = require("path");
const fs = require("fs");

// ── 1. Bootstrap the Go WASM runtime ─────────────────────────────────────────
// wasm_exec.js registers globalThis.Go; we copy it from $(go env GOROOT)/lib/wasm/
const wasmExecPath = path.join(__dirname, "wasm_exec.js");
if (!fs.existsSync(wasmExecPath)) {
  console.error(
    `wasm_exec.js not found at ${wasmExecPath}\n` +
      "Run: task wasm:copy-support:js  (or copy manually from $(go env GOROOT)/lib/wasm/wasm_exec.js)",
  );
  process.exit(1);
}
require(wasmExecPath);

// ── 2. Loader ─────────────────────────────────────────────────────────────────
async function loadGoSonataWasm(wasmPath) {
  const go = new Go(); // eslint-disable-line no-undef
  const buf = fs.readFileSync(wasmPath);
  const { instance } = await WebAssembly.instantiate(buf, go.importObject);

  // Run the Go main() — it sets globalThis.gosonata then blocks.
  go.run(instance);

  // Yield to the event loop so Go's init goroutines can complete.
  await new Promise((r) => setImmediate(r));

  if (!globalThis.gosonata) {
    throw new Error("gosonata global not registered after WASM init");
  }

  const gs = globalThis.gosonata;
  return {
    version: () => gs.version(),
    /**
     * eval(query, data) — data can be a JS object or a JSON string.
     * Returns the result as a parsed JS value.
     */
    eval: (query, data) => {
      const dataJSON = typeof data === "string" ? data : JSON.stringify(data);
      return JSON.parse(gs.eval(query, dataJSON));
    },
    /**
     * compile(query) — returns a compiled expression.
     * compiled.eval(data) evaluates against the given data.
     */
    compile: (query) => {
      const compiled = gs.compile(query);
      return {
        eval: (data) => {
          const dataJSON =
            typeof data === "string" ? data : JSON.stringify(data);
          return JSON.parse(compiled.eval(dataJSON));
        },
      };
    },
  };
}

// ── 3. Demo ───────────────────────────────────────────────────────────────────
async function main() {
  // Look for gosonata.wasm next to this file first, then in cmd/wasm/js/ (project root).
  const localPath = path.join(__dirname, "gosonata.wasm");
  const buildPath = path.join(
    __dirname,
    "..",
    "..",
    "..",
    "cmd",
    "wasm",
    "js",
    "gosonata.wasm",
  );
  const wasmPath = fs.existsSync(localPath) ? localPath : buildPath;
  if (!fs.existsSync(wasmPath)) {
    console.error(
      `gosonata.wasm not found.\nTried:\n  ${localPath}\n  ${buildPath}\nRun: task wasm:build:js`,
    );
    process.exit(1);
  }

  console.log("Loading GoSonata WASM…");
  const gs = await loadGoSonataWasm(wasmPath);
  console.log(`GoSonata ${gs.version()} ready\n`);

  const catalog = {
    store: "GoShop",
    currency: "EUR",
    products: [
      { id: 1, name: "Widget", price: 49.99, category: "tools", stock: 120 },
      { id: 2, name: "Gadget", price: 149.99, category: "tech", stock: 3 },
      { id: 3, name: "Doohickey", price: 9.99, category: "tools", stock: 55 },
      { id: 4, name: "Thingamajig", price: 299.0, category: "tech", stock: 0 },
    ],
  };

  const examples = [
    ["Simple path", "$.store"],
    ["Filter by price", "$.products[price > 50].name"],
    ["Aggregation", "$sum($.products.price)"],
    ["Transform", '$.products.{"item": name, "eur": price}'],
    ["Conditional", '$.products[stock = 0].name & " (out of stock)"'],
    ["Count by category (tools)", '$count($.products[category = "tools"])'],
  ];

  // One-shot eval
  console.log("── One-shot eval ──────────────────────────────────────────");
  for (const [label, query] of examples) {
    const result = gs.eval(query, catalog);
    console.log(`  ${label.padEnd(32)} ${JSON.stringify(result)}`);
  }

  // Compile once, eval many times
  console.log("\n── Compile once, eval many times ──────────────────────────");
  // $$.threshold accesses the root data's `threshold` field from within the predicate.
  const expr = gs.compile("$.products[price > $$.threshold].name");

  for (const threshold of [10, 50, 100, 200]) {
    const result = expr.eval({ ...catalog, threshold });
    console.log(
      `  price > ${String(threshold).padEnd(4)} → ${JSON.stringify(result)}`,
    );
  }

  console.log("\nDone.");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
