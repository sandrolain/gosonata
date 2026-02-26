// Package extstring provides extended string functions for GoSonata beyond the
// official JSONata spec. Register them via gosonata.WithFunctions or
// via the top-level ext.WithString() helper.
package extstring

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/sandrolain/gosonata/pkg/ext/extutil"
	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended string function definitions.
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		StartsWith(),
		EndsWith(),
		IndexOf(),
		LastIndexOf(),
		Capitalize(),
		TitleCase(),
		CamelCase(),
		SnakeCase(),
		KebabCase(),
		Repeat(),
		Words(),
		Template(),
	}
}

// AllEntries returns all string function definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions]:
//
//	gosonata.WithFunctions(extstring.AllEntries()...)
func AllEntries() []functions.FunctionEntry {
	all := All()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// StartsWith returns the definition for $startsWith(str, prefix).
func StartsWith() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "startsWith",
		Signature: "<s-s:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok1 := args[0].(string)
			prefix, ok2 := args[1].(string)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("$startsWith: arguments must be strings")
			}
			return strings.HasPrefix(str, prefix), nil
		},
	}
}

// EndsWith returns the definition for $endsWith(str, suffix).
func EndsWith() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "endsWith",
		Signature: "<s-s:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok1 := args[0].(string)
			suffix, ok2 := args[1].(string)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("$endsWith: arguments must be strings")
			}
			return strings.HasSuffix(str, suffix), nil
		},
	}
}

// IndexOf returns the definition for $indexOf(str, search [, start]).
// Returns -1 (as float64) when not found.
func IndexOf() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "indexOf",
		Signature: "<s-s<n>?:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok1 := args[0].(string)
			search, ok2 := args[1].(string)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("$indexOf: first two arguments must be strings")
			}
			start := 0
			if len(args) >= 3 && args[2] != nil {
				n, ok := toInt(args[2])
				if !ok {
					return nil, fmt.Errorf("$indexOf: start argument must be a number")
				}
				if n < 0 {
					n = 0
				}
				start = n
			}
			if start >= len(str) {
				return float64(-1), nil
			}
			idx := strings.Index(str[start:], search)
			if idx == -1 {
				return float64(-1), nil
			}
			return float64(idx + start), nil
		},
	}
}

// LastIndexOf returns the definition for $lastIndexOf(str, search).
// Returns -1 (as float64) when not found.
func LastIndexOf() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "lastIndexOf",
		Signature: "<s-s:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok1 := args[0].(string)
			search, ok2 := args[1].(string)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("$lastIndexOf: arguments must be strings")
			}
			idx := strings.LastIndex(str, search)
			return float64(idx), nil
		},
	}
}

// Capitalize returns the definition for $capitalize(str).
// Uppercases the first character, lowercases the rest.
func Capitalize() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "capitalize",
		Signature: "<s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$capitalize: argument must be a string")
			}
			if str == "" {
				return str, nil
			}
			runes := []rune(str)
			runes[0] = unicode.ToUpper(runes[0])
			for i := 1; i < len(runes); i++ {
				runes[i] = unicode.ToLower(runes[i])
			}
			return string(runes), nil
		},
	}
}

// TitleCase returns the definition for $titleCase(str).
// Uppercases the first character of each word.
func TitleCase() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "titleCase",
		Signature: "<s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$titleCase: argument must be a string")
			}
			return strings.Title(strings.ToLower(str)), nil //nolint:staticcheck
		},
	}
}

// splitWords splits a string into words by camelCase, snake_case, kebab-case, and spaces.
var splitWordsRe = regexp.MustCompile(`[_\-\s]+|([a-z])([A-Z])`)

func splitIntoWords(str string) []string {
	// Insert space before uppercase letters following lowercase letters (camelCase)
	expanded := splitWordsRe.ReplaceAllStringFunc(str, func(s string) string {
		if len(s) == 2 && s[0] >= 'a' && s[0] <= 'z' {
			return string(s[0]) + " " + string(s[1])
		}
		return " "
	})
	parts := strings.Fields(expanded)
	return parts
}

// CamelCase returns the definition for $camelCase(str).
func CamelCase() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "camelCase",
		Signature: "<s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$camelCase: argument must be a string")
			}
			words := splitIntoWords(str)
			if len(words) == 0 {
				return "", nil
			}
			var b strings.Builder
			b.WriteString(strings.ToLower(words[0]))
			for _, w := range words[1:] {
				if w == "" {
					continue
				}
				runes := []rune(strings.ToLower(w))
				runes[0] = unicode.ToUpper(runes[0])
				b.WriteString(string(runes))
			}
			return b.String(), nil
		},
	}
}

// SnakeCase returns the definition for $snakeCase(str).
func SnakeCase() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "snakeCase",
		Signature: "<s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$snakeCase: argument must be a string")
			}
			words := splitIntoWords(str)
			for i, w := range words {
				words[i] = strings.ToLower(w)
			}
			return strings.Join(words, "_"), nil
		},
	}
}

// KebabCase returns the definition for $kebabCase(str).
func KebabCase() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "kebabCase",
		Signature: "<s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$kebabCase: argument must be a string")
			}
			words := splitIntoWords(str)
			for i, w := range words {
				words[i] = strings.ToLower(w)
			}
			return strings.Join(words, "-"), nil
		},
	}
}

// Repeat returns the definition for $repeat(str, n).
func Repeat() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "repeat",
		Signature: "<s-n:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$repeat: first argument must be a string")
			}
			n, ok := toInt(args[1])
			if !ok || n < 0 {
				return nil, fmt.Errorf("$repeat: second argument must be a non-negative integer")
			}
			return strings.Repeat(str, n), nil
		},
	}
}

// Words returns the definition for $words(str).
// Splits on whitespace, returning a deduplicated list of non-empty words.
func Words() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "words",
		Signature: "<s:a<s>>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$words: argument must be a string")
			}
			parts := strings.Fields(str)
			if len(parts) == 0 {
				return nil, nil
			}
			result := make([]interface{}, len(parts))
			for i, p := range parts {
				result[i] = p
			}
			return result, nil
		},
	}
}

// Template returns the definition for $template(str, bindings).
// Replaces {{key}} placeholders with values from the bindings object.
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

// toInt converts a JSONata numeric value (float64) to int.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
