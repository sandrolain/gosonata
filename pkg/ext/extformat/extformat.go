// Package extformat provides data-format functions for GoSonata (CSV, templates).
// All functions use only the Go standard library.
package extformat

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"

	"github.com/sandrolain/gosonata/pkg/ext/extutil"
	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended format function definitions.
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		ParseCSV(),
		ToCSV(),
		Template(),
	}
}

// AllEntries returns all format function definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions].
func AllEntries() []functions.FunctionEntry {
	all := All()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// ParseCSV returns the definition for $csv(str [, options]).
// Parses a CSV string into an array of objects using the first row as headers.
//
// options object (all optional):
//   - "separator": field delimiter character (default ",")
//   - "comment":   comment character (default none)
func ParseCSV() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "csv",
		Signature: "<s<o>?:a<o>>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			src, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$csv: first argument must be a string")
			}

			separator := ','
			var comment rune

			if len(args) >= 2 && args[1] != nil {
				opts, ok := args[1].(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("$csv: options must be an object")
				}
				if sep, ok := opts["separator"].(string); ok && len(sep) > 0 {
					separator = rune(sep[0])
				}
				if c, ok := opts["comment"].(string); ok && len(c) > 0 {
					comment = rune(c[0])
				}
			}

			r := csv.NewReader(strings.NewReader(src))
			r.Comma = separator
			if comment != 0 {
				r.Comment = comment
			}
			r.TrimLeadingSpace = true

			records, err := r.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("$csv: parse error: %w", err)
			}
			if len(records) < 2 {
				return nil, nil // no data rows
			}

			headers := records[0]
			result := make([]interface{}, 0, len(records)-1)
			for _, row := range records[1:] {
				obj := make(map[string]interface{}, len(headers))
				for i, h := range headers {
					if i < len(row) {
						obj[h] = row[i]
					} else {
						obj[h] = ""
					}
				}
				result = append(result, obj)
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// ToCSV returns the definition for $toCSV(array, columns).
// Converts an array of objects to a CSV string with a header row.
//
// columns is an optional array of column names. When omitted, keys of the first
// object are used.
func ToCSV() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "toCSV",
		Signature: "<a<o><a<s>>?:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, ok := args[0].([]interface{})
			if !ok {
				return nil, fmt.Errorf("$toCSV: first argument must be an array")
			}
			if len(arr) == 0 {
				return "", nil
			}

			// Determine columns
			var columns []string
			if len(args) >= 2 && args[1] != nil {
				colsRaw, ok := args[1].([]interface{})
				if !ok {
					return nil, fmt.Errorf("$toCSV: columns must be an array")
				}
				for _, c := range colsRaw {
					if s, ok := c.(string); ok {
						columns = append(columns, s)
					}
				}
			}
			if len(columns) == 0 {
				// Use keys from first object
				if first, ok := arr[0].(map[string]interface{}); ok {
					for k := range first {
						columns = append(columns, k)
					}
				}
			}
			if len(columns) == 0 {
				return nil, fmt.Errorf("$toCSV: cannot determine columns")
			}

			var buf bytes.Buffer
			w := csv.NewWriter(&buf)

			// Write header
			if err := w.Write(columns); err != nil {
				return nil, fmt.Errorf("$toCSV: %w", err)
			}

			// Write rows
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("$toCSV: all array elements must be objects")
				}
				row := make([]string, len(columns))
				for i, col := range columns {
					if v, exists := obj[col]; exists {
						row[i] = fmt.Sprint(v)
					}
				}
				if err := w.Write(row); err != nil {
					return nil, fmt.Errorf("$toCSV: %w", err)
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return nil, fmt.Errorf("$toCSV: %w", err)
			}
			return buf.String(), nil
		},
	}
}

// Template returns the definition for $template(str, bindings).
// Replaces {{key}} placeholders with values from the bindings object.
// This mirrors the extstring.Template function for convenience when only the
// format package is imported.
func Template() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "template",
		Signature: "<s-o:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			tmpl, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$template: first argument must be a string")
			}
			bindings, err := extutil.AsObjectMap(args[1])
			if err != nil {
				return nil, fmt.Errorf("$template: second argument must be an object")
			}
			result := regexp.MustCompile(`\{\{(\w+)\}\}`).ReplaceAllStringFunc(tmpl, func(match string) string {
				key := match[2 : len(match)-2]
				if val, exists := bindings[key]; exists {
					return fmt.Sprint(val)
				}
				return match
			})
			return result, nil
		},
	}
}
