//go:build wasip1

// Command gosonata-wasm-wasi is the WASI (wasip1) entrypoint for use from any
// language that supports the WebAssembly System Interface.
//
// Protocol: single JSON object on stdin → single JSON object on stdout.
//
//	stdin:  { "query": "<jsonata>", "data": <any JSON value> }
//	stdout: { "result": <any JSON value> }    on success
//	        { "error":  "<message>"       }    on failure (exit code 1)
//
// Build:
//
//	GOOS=wasip1 GOARCH=wasm go build -o gosonata.wasm ./cmd/wasm/wasi/
//
// Usage with wasmtime CLI:
//
//	echo '{"query":"$.name","data":{"name":"Alice"}}' | wasmtime gosonata.wasm
//
// Usage from Python (wasmtime-py):
//
//	import wasmtime, json
//	engine = wasmtime.Engine()
//	...
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/sandrolain/gosonata"
)

type request struct {
	Query string      `json:"query"`
	Data  interface{} `json:"data"`
}

type response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func writeResponse(r response, exitCode int) {
	_ = json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(exitCode)
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		writeResponse(response{Error: "invalid request JSON: " + err.Error()}, 1)
	}

	result, err := gosonata.EvalWithContext(context.Background(), req.Query, req.Data,
		gosonata.WithConcurrency(false),
	)
	if err != nil {
		writeResponse(response{Error: err.Error()}, 1)
	}

	writeResponse(response{Result: result}, 0)
}
