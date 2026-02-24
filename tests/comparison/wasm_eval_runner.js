#!/usr/bin/env node
/**
 * GoSonata WASM single-evaluation runner (js/wasm target).
 *
 * Used by correctness tests: evaluates a query once and returns the result.
 *
 * stdin:  { "query": "...", "data": ... }
 * stdout: { "success": true,  "result": ... }
 *         { "success": false, "error":  "..." }
 */

"use strict";

const path = require("path");
const fs = require("fs");

const wasmExecPath =
  process.env.WASM_EXEC_JS ||
  path.join(__dirname, "..", "..", "cmd", "wasm", "js", "wasm_exec.js");
if (!fs.existsSync(wasmExecPath)) {
  process.stdout.write(
    JSON.stringify({
      success: false,
      result: null,
      error: `wasm_exec.js not found at ${wasmExecPath}`,
    }) + "\n",
  );
  process.exit(1);
}
require(wasmExecPath);

async function main() {
  let input = "";
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) input += chunk;

  const { query, data } = JSON.parse(input);

  const wasmPath =
    process.env.GOSONATA_WASM ||
    path.join(__dirname, "..", "..", "cmd", "wasm", "js", "gosonata.wasm");

  const go = new Go(); // eslint-disable-line no-undef
  const buf = fs.readFileSync(wasmPath);
  const { instance } = await WebAssembly.instantiate(buf, go.importObject);
  go.run(instance);
  await new Promise((r) => setImmediate(r));

  try {
    const raw = globalThis.gosonata.eval(query, JSON.stringify(data));
    const result = JSON.parse(raw);
    process.stdout.write(
      JSON.stringify({
        success: true,
        result: result === undefined ? null : result,
      }) + "\n",
    );
  } catch (err) {
    process.stdout.write(
      JSON.stringify({ success: false, result: null, error: err.message }) +
        "\n",
    );
    process.exit(1);
  }
}

main().catch((err) => {
  process.stdout.write(
    JSON.stringify({ success: false, result: null, error: err.message }) + "\n",
  );
  process.exit(1);
});
