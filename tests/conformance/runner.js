#!/usr/bin/env node

/**
 * JSONata JavaScript runner for conformance testing.
 * Accepts JSON input with query and data, executes with JSONata JS, returns result.
 */

const jsonata = require("jsonata");
const fs = require("fs");

// Read input from stdin or file
async function main() {
  let input = "";

  if (process.argv[2]) {
    // Read from file
    input = fs.readFileSync(process.argv[2], "utf8");
  } else {
    // Read from stdin
    process.stdin.setEncoding("utf8");

    for await (const chunk of process.stdin) {
      input += chunk;
    }
  }

  try {
    const testCase = JSON.parse(input);
    const { query, data, bindings } = testCase;

    // Compile and evaluate
    const expression = jsonata(query);

    // Set bindings if provided
    if (bindings) {
      for (const [name, value] of Object.entries(bindings)) {
        expression.assign(name, value);
      }
    }

    const result = await expression.evaluate(data);

    // Output result as JSON
    const output = {
      success: true,
      result: result,
      error: null,
    };

    console.log(JSON.stringify(output, null, 2));
  } catch (error) {
    // Output error
    const output = {
      success: false,
      result: null,
      error: {
        message: error.message,
        position: error.position,
        token: error.token,
        code: error.code,
      },
    };

    console.log(JSON.stringify(output, null, 2));
    process.exit(1);
  }
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
