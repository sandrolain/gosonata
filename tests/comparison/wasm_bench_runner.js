#!/usr/bin/env node
/**
 * GoSonata WASM benchmark runner (js/wasm target).
 *
 * Accepts a single JSON payload on stdin:
 *   { "query": "...", "data": ..., "iterations": N, "warmup": W }
 *
 * Outputs a single JSON line to stdout:
 *   { "success": true, "nsPerOp": ..., "opsPerSec": ..., "iterations": N }
 *   { "success": false, "error": "..." }
 *
 * The WASM binary path is taken from the GOSONATA_WASM env var, defaulting to
 * ./cmd/wasm/js/gosonata.wasm relative to the project root (set by the test).
 */

"use strict";

const path = require("path");
const fs = require("fs");

// ── bootstrap wasm_exec.js ────────────────────────────────────────────────────
const wasmExecPath =
  process.env.WASM_EXEC_JS ||
  path.join(__dirname, "..", "..", "cmd", "wasm", "js", "wasm_exec.js");
if (!fs.existsSync(wasmExecPath)) {
  writeError(`wasm_exec.js not found at ${wasmExecPath}`);
  process.exit(1);
}
require(wasmExecPath);

function writeError(msg) {
  process.stdout.write(
    JSON.stringify({
      success: false,
      nsPerOp: 0,
      opsPerSec: 0,
      iterations: 0,
      error: msg,
    }) + "\n",
  );
}

async function loadGoSonataWasm() {
  const wasmPath =
    process.env.GOSONATA_WASM ||
    path.join(__dirname, "..", "..", "cmd", "wasm", "js", "gosonata.wasm");

  if (!fs.existsSync(wasmPath)) {
    throw new Error(
      `gosonata.wasm not found at ${wasmPath}. Run: task wasm:build:js`,
    );
  }

  const go = new Go(); // eslint-disable-line no-undef
  const buf = fs.readFileSync(wasmPath);
  const { instance } = await WebAssembly.instantiate(buf, go.importObject);
  go.run(instance);
  await new Promise((r) => setImmediate(r));

  if (!globalThis.gosonata) throw new Error("gosonata global not registered");
  return globalThis.gosonata;
}

async function runBench(gs, query, data, iterations, warmup) {
  const dataJSON = JSON.stringify(data);
  // Compile once — mirrors how Go benchmarks pre-parse the expression.
  const compiled = gs.compile(query);

  // Warmup
  for (let i = 0; i < warmup; i++) compiled.eval(dataJSON);

  // Timed run
  const start = process.hrtime.bigint();
  for (let i = 0; i < iterations; i++) compiled.eval(dataJSON);
  const elapsed = process.hrtime.bigint() - start;

  const nsPerOp = Number(elapsed) / iterations;
  return { nsPerOp, opsPerSec: 1e9 / nsPerOp, iterations };
}

async function main() {
  let input = "";
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) input += chunk;

  let parsed;
  try {
    parsed = JSON.parse(input);
  } catch (err) {
    writeError("invalid JSON input: " + err.message);
    process.exit(1);
  }

  const { query, data, iterations = 10000, warmup = 500 } = parsed;

  try {
    const gs = await loadGoSonataWasm();
    const result = await runBench(gs, query, data, iterations, warmup);
    process.stdout.write(
      JSON.stringify({ success: true, error: null, ...result }) + "\n",
    );
  } catch (err) {
    writeError(err.message);
    process.exit(1);
  }
}

main();
