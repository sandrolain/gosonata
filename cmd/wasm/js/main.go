//go:build js && wasm

// Command gosonata-wasm-js is the WebAssembly entrypoint for browser and Node.js.
//
// It exposes a global `gosonata` object with the following API:
//
//	gosonata.version()               → string
//	gosonata.eval(query, dataJSON)   → resultJSON  (throws on error)
//	gosonata.compile(query)          → { eval(dataJSON) → resultJSON }  (throws on error)
//
// Build:
//
//	GOOS=js GOARCH=wasm go build -o gosonata.wasm ./cmd/wasm/js/
//
// Usage in Node.js (see examples/wasm/node/):
//
//	const { load } = require('./gosonata_wasm')
//	const gs = await load()
//	const result = gs.eval('$.name', JSON.stringify({name:'Alice'}))
//	console.log(JSON.parse(result)) // 'Alice'
//
// Usage in browser (see examples/wasm/browser/):
//
//	<script src="wasm_exec.js"></script>
//	<script type="module">
//	  import { load } from './gosonata_wasm.mjs'
//	  const gs = await load()
//	  console.log(JSON.parse(gs.eval('$.x', JSON.stringify({x:42}))))
//	</script>
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/sandrolain/gosonata"
	"github.com/sandrolain/gosonata/pkg/evaluator"
)

// jsThrow panics with a JS Error so the caller receives a thrown exception.
func jsThrow(msg string) {
	js.Global().Get("Error").New(msg)
	panic(msg)
}

// jsEval implements gosonata.eval(query, dataJSON) → resultJSON.
func jsEval(_ js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		jsThrow("gosonata.eval requires 2 arguments: query (string) and data (JSON string)")
	}
	query := args[0].String()
	dataJSON := args[1].String()

	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		jsThrow(fmt.Sprintf("gosonata.eval: invalid data JSON: %v", err))
	}

	result, err := gosonata.EvalWithContext(context.Background(), query, data,
		gosonata.WithConcurrency(false),
	)
	if err != nil {
		jsThrow(fmt.Sprintf("gosonata.eval: %v", err))
	}

	out, err := json.Marshal(result)
	if err != nil {
		jsThrow(fmt.Sprintf("gosonata.eval: marshal result: %v", err))
	}
	return string(out)
}

// jsCompile implements gosonata.compile(query) → { eval(dataJSON) → resultJSON }.
func jsCompile(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		jsThrow("gosonata.compile requires 1 argument: query (string)")
	}
	query := args[0].String()

	expr, err := gosonata.Compile(query)
	if err != nil {
		jsThrow(fmt.Sprintf("gosonata.compile: %v", err))
	}

	ev := evaluator.New(gosonata.WithConcurrency(false))

	evalFn := js.FuncOf(func(_ js.Value, innerArgs []js.Value) interface{} {
		if len(innerArgs) < 1 {
			jsThrow("compiled.eval requires 1 argument: data (JSON string)")
		}
		var data interface{}
		if e := json.Unmarshal([]byte(innerArgs[0].String()), &data); e != nil {
			jsThrow(fmt.Sprintf("compiled.eval: invalid data JSON: %v", e))
		}
		r, e := ev.Eval(context.Background(), expr, data)
		if e != nil {
			jsThrow(fmt.Sprintf("compiled.eval: %v", e))
		}
		out, _ := json.Marshal(r)
		return string(out)
	})

	obj := js.ValueOf(map[string]interface{}{"eval": evalFn})
	return obj
}

func main() {
	api := map[string]interface{}{
		"eval":    js.FuncOf(jsEval),
		"compile": js.FuncOf(jsCompile),
		"version": js.FuncOf(func(_ js.Value, _ []js.Value) interface{} {
			return gosonata.Version()
		}),
	}
	js.Global().Set("gosonata", js.ValueOf(api))

	// Block forever — the JS event loop owns execution from here.
	select {}
}
