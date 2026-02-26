// Package ext provides optional extension functions for GoSonata that go beyond
// the official JSONata 2.1.0+ specification.
//
// The extension functions live in sub-packages grouped by category:
//   - extstring   – $startsWith, $endsWith, $indexOf, $camelCase, $template, …
//   - extnumeric  – $log, $sign, $trunc, $clamp, trig functions, $median, …
//   - extarray    – $first, $last, $take, $skip, $flatten, $chunk, set ops, …
//   - extobject   – $values, $pairs, $pick, $omit, $deepMerge, $rename, …
//   - exttypes    – $isString, $isArray, $isEmpty, $default, $identity, …
//   - extdatetime – $dateAdd, $dateDiff, $dateComponents, $dateStartOf, …
//   - extcrypto   – $uuid, $hash, $hmac
//   - extformat   – $csv, $toCSV, $template
//   - extfunc     – $pipe, $memoize (advanced/HOF)
//
// # Integration – all extensions at once
//
//	import "github.com/sandrolain/gosonata/pkg/ext"
//
//	result, err := gosonata.Eval(expr, data, ext.WithAll())
//
// # Integration – by category
//
//	result, err := gosonata.Eval(expr, data,
//	    ext.WithString(),
//	    ext.WithArray(),
//	)
//
// # Integration – single function from a sub-package
//
//	import extstring "github.com/sandrolain/gosonata/pkg/ext/extstring"
//
//	result, err := gosonata.Eval(expr, data,
//	    gosonata.WithFunctions(extstring.StartsWith()),
//	)
package ext

import (
	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/ext/extarray"
	"github.com/sandrolain/gosonata/pkg/ext/extcrypto"
	"github.com/sandrolain/gosonata/pkg/ext/extdatetime"
	"github.com/sandrolain/gosonata/pkg/ext/extformat"
	"github.com/sandrolain/gosonata/pkg/ext/extfunc"
	"github.com/sandrolain/gosonata/pkg/ext/extnumeric"
	"github.com/sandrolain/gosonata/pkg/ext/extobject"
	"github.com/sandrolain/gosonata/pkg/ext/extstring"
	"github.com/sandrolain/gosonata/pkg/ext/exttypes"
	"github.com/sandrolain/gosonata/pkg/functions"
)

// AllSimple returns all simple (non-HOF) extension function definitions.
func AllSimple() []functions.CustomFunctionDef {
	var all []functions.CustomFunctionDef
	all = append(all, extstring.All()...)
	all = append(all, extnumeric.All()...)
	all = append(all, extarray.All()...)
	all = append(all, extobject.All()...)
	all = append(all, exttypes.All()...)
	all = append(all, extdatetime.All()...)
	all = append(all, extcrypto.All()...)
	all = append(all, extformat.All()...)
	return all
}

// AllAdvanced returns all advanced (HOF) extension function definitions.
func AllAdvanced() []functions.AdvancedCustomFunctionDef {
	var all []functions.AdvancedCustomFunctionDef
	all = append(all, extarray.AllAdvanced()...)
	all = append(all, extobject.AllAdvanced()...)
	all = append(all, extfunc.AllAdvanced()...)
	return all
}

// AllEntries returns all extension function definitions (simple + advanced) as
// [functions.FunctionEntry], suitable for spreading into [gosonata.WithFunctions]:
//
//	gosonata.WithFunctions(ext.AllEntries()...)
func AllEntries() []functions.FunctionEntry {
	simple := AllSimple()
	adv := AllAdvanced()
	out := make([]functions.FunctionEntry, 0, len(simple)+len(adv))
	for _, f := range simple {
		out = append(out, f)
	}
	for _, f := range adv {
		out = append(out, f)
	}
	return out
}

// WithAll returns an EvalOption that registers all extension functions
// (both simple and advanced HOF).
func WithAll() evaluator.EvalOption {
	return evaluator.WithFunctions(AllEntries()...)
}

// WithString returns an EvalOption for the extended string functions.
func WithString() evaluator.EvalOption {
	return evaluator.WithFunctions(extstring.AllEntries()...)
}

// WithNumeric returns an EvalOption for the extended numeric functions.
func WithNumeric() evaluator.EvalOption {
	return evaluator.WithFunctions(extnumeric.AllEntries()...)
}

// WithArray returns an EvalOption for the extended array functions
// (includes both simple and HOF variants).
func WithArray() evaluator.EvalOption {
	return evaluator.WithFunctions(extarray.AllEntries()...)
}

// WithObject returns an EvalOption for the extended object functions
// (includes both simple and HOF variants).
func WithObject() evaluator.EvalOption {
	return evaluator.WithFunctions(extobject.AllEntries()...)
}

// WithTypes returns an EvalOption for the type predicate functions.
func WithTypes() evaluator.EvalOption {
	return evaluator.WithFunctions(exttypes.AllEntries()...)
}

// WithDateTime returns an EvalOption for the extended date/time functions.
func WithDateTime() evaluator.EvalOption {
	return evaluator.WithFunctions(extdatetime.AllEntries()...)
}

// WithCrypto returns an EvalOption for the cryptographic functions.
func WithCrypto() evaluator.EvalOption {
	return evaluator.WithFunctions(extcrypto.AllEntries()...)
}

// WithFormat returns an EvalOption for the data-format functions (CSV, template).
func WithFormat() evaluator.EvalOption {
	return evaluator.WithFunctions(extformat.AllEntries()...)
}

// WithFunctional returns an EvalOption for the functional utility HOFs (pipe, memoize).
func WithFunctional() evaluator.EvalOption {
	return evaluator.WithFunctions(extfunc.AllEntries()...)
}
