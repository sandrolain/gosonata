/**
 * GoSonata WASM — Browser loader helper.
 *
 * Exposes window.GoSonataWasm.load(wasmPath) → Promise<GoSonataAPI>
 *
 * The returned object has:
 *   gs.version()               → string
 *   gs.eval(query, dataJSON)   → resultJSON   (throws on JSONata error)
 *   gs.compile(query)          → { eval(dataJSON) → resultJSON }
 *
 * Requires wasm_exec.js (from Go SDK) to be loaded first.
 */
(function () {
  "use strict";

  async function load(wasmPath) {
    wasmPath = wasmPath || "gosonata.wasm";

    const go = new Go(); // provided by wasm_exec.js

    let instance;
    if (typeof WebAssembly.instantiateStreaming === "function") {
      const result = await WebAssembly.instantiateStreaming(
        fetch(wasmPath),
        go.importObject,
      );
      instance = result.instance;
    } else {
      const bytes = await fetch(wasmPath).then((r) => r.arrayBuffer());
      const result = await WebAssembly.instantiate(bytes, go.importObject);
      instance = result.instance;
    }

    // Run the Go program — it will set globalThis.gosonata and then block.
    go.run(instance);

    // Wait until gosonata is registered on globalThis.
    await new Promise((resolve) => {
      const check = () => {
        if (globalThis.gosonata) resolve();
        else setTimeout(check, 5);
      };
      check();
    });

    const gs = globalThis.gosonata;
    return {
      version: () => gs.version(),
      eval: (query, dataJSON) => gs.eval(query, dataJSON),
      compile: (query) => gs.compile(query),
    };
  }

  if (typeof window !== "undefined") {
    window.GoSonataWasm = { load };
  }
  if (typeof module !== "undefined" && module.exports) {
    module.exports = { load };
  }
})();
