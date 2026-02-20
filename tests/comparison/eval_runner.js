#!/usr/bin/env node
/**
 * JSONata JS single-evaluation runner for result correctness checks.
 *
 * Accepts a JSON payload on stdin:
 *   { "query": "...", "data": ... }
 *
 * Outputs a single JSON line to stdout:
 *   { "success": true, "result": ... }
 *   { "success": false, "error": "..." }
 */
const path = require("path");
const jsonata = require(
  path.resolve(__dirname, "../conformance/node_modules/jsonata"),
);

async function main() {
  let input = "";
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) {
    input += chunk;
  }
  try {
    const { query, data } = JSON.parse(input);
    const expr = jsonata(query);
    const result = await expr.evaluate(data);
    // undefined → null for JSON serialisation
    console.log(
      JSON.stringify({
        success: true,
        result: result === undefined ? null : result,
      }),
    );
  } catch (err) {
    console.log(
      JSON.stringify({ success: false, result: null, error: err.message }),
    );
    process.exit(1);
  }
}
main();
