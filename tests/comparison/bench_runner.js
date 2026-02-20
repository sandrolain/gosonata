#!/usr/bin/env node
/**
 * JSONata JS benchmark runner.
 *
 * Accepts a single JSON payload on stdin:
 *   { "query": "...", "data": ..., "iterations": N, "warmup": W }
 *
 * Outputs a single JSON line to stdout:
 *   { "success": true, "nsPerOp": ..., "opsPerSec": ..., "iterations": N }
 *
 * Node modules are resolved from the sibling conformance directory.
 */

const path = require("path");

// Resolve jsonata from the conformance node_modules
const jsonataPath = path.resolve(
  __dirname,
  "../conformance/node_modules/jsonata",
);
const jsonata = require(jsonataPath);

async function runBench(query, data, iterations, warmup) {
  // Compile once (mirrors GoSonata "eval only" benchmarks where expr is pre-parsed)
  const expr = jsonata(query);

  // Warmup
  for (let i = 0; i < warmup; i++) {
    await expr.evaluate(data);
  }

  // Timed run
  const start = process.hrtime.bigint();
  for (let i = 0; i < iterations; i++) {
    await expr.evaluate(data);
  }
  const elapsed = process.hrtime.bigint() - start;

  const nsPerOp = Number(elapsed) / iterations;
  const opsPerSec = 1e9 / nsPerOp;

  return { nsPerOp, opsPerSec, iterations };
}

async function main() {
  let input = "";
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) {
    input += chunk;
  }

  try {
    const { query, data, iterations = 10000, warmup = 500 } = JSON.parse(input);
    const result = await runBench(query, data, iterations, warmup);
    console.log(JSON.stringify({ success: true, ...result, error: null }));
  } catch (err) {
    console.log(
      JSON.stringify({
        success: false,
        nsPerOp: 0,
        opsPerSec: 0,
        iterations: 0,
        error: err.message,
      }),
    );
    process.exit(1);
  }
}

main();
