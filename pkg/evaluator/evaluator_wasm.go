//go:build (js && wasm) || wasip1

package evaluator

// init sets WebAssembly-specific defaults for all Evaluators created in this
// process.
//
// On js/wasm (browser / Node.js) the JavaScript runtime is single-threaded:
// goroutines are multiplexed cooperatively on the same OS thread, so spawning
// additional goroutines for sub-evaluations can cause deadlocks when a
// goroutine blocks waiting for a channel that is never drained.
// Disabling concurrency (Concurrency = false) is therefore mandatory for
// js/wasm builds.
//
// On wasip1 (Wasmtime, WasmEdge, Deno, etc.) the same conservative default
// applies: the WASI threading proposal is still experimental and not yet
// supported by the Go runtime.
func init() {
	defaultConcurrency = false
}
