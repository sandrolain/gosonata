package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sandrolain/gosonata/pkg/types"
)

// StreamResult holds the output of a single streaming evaluation step.
type StreamResult struct {
	// Value is the evaluated result for one input document, or nil when Err is set.
	Value interface{}
	// Err is non-nil when evaluation of a single document failed.
	// After a fatal I/O or JSON-decode error the channel is closed; per-document
	// evaluation errors are sent individually and the stream continues.
	Err error
}

// EvalStream reads a sequence of JSON values from r (e.g. NDJSON / JSON-seq) and
// evaluates expr against each one, sending results on the returned channel.
//
// The channel is closed when all input has been consumed or the context is cancelled.
// A fatal I/O or JSON-decode error is sent as a StreamResult with a non-nil Err and
// then the channel is closed.  Per-document evaluation errors are sent as individual
// StreamResult values and the stream continues to the next document.
//
// It is the caller's responsibility to drain the channel or cancel the context to
// avoid goroutine leaks.
func (e *Evaluator) EvalStream(ctx context.Context, expr *types.Expression, r io.Reader) (<-chan StreamResult, error) {
	if expr == nil || expr.AST() == nil {
		return nil, fmt.Errorf("invalid expression")
	}

	ch := make(chan StreamResult, 16)

	go func() {
		defer close(ch)

		dec := json.NewDecoder(r)
		for {
			select {
			case <-ctx.Done():
				ch <- StreamResult{Err: ctx.Err()}
				return
			default:
			}

			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				if err == io.EOF {
					return
				}
				ch <- StreamResult{Err: err}
				return
			}

			var data interface{}
			if err := json.Unmarshal(raw, &data); err != nil {
				ch <- StreamResult{Err: err}
				return
			}

			result, err := e.Eval(ctx, expr, data)
			ch <- StreamResult{Value: result, Err: err}
		}
	}()

	return ch, nil
}
