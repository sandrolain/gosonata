package evaluator

import (
	"context"
	"sync"

	"github.com/sandrolain/gosonata/pkg/types"
)

// FunctionDef defines a built-in function.
type FunctionDef struct {
	Name           string
	MinArgs        int
	MaxArgs        int  // -1 for unlimited
	AcceptsContext bool // If true, pass context value as first arg when called with no args
	Impl           FunctionImpl
}

// FunctionImpl is the implementation of a function.

type FunctionImpl func(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error)

// Lambda represents a lambda function.

type Lambda struct {
	Params    []string
	Body      *types.ASTNode
	Ctx       *EvalContext // Closure context
	Signature *Signature   // Parsed signature for type validation
}

// OrderedObject preserves insertion order for JSON stringification.

var (
	builtinFunctions     map[string]*FunctionDef
	builtinFunctionsOnce sync.Once
)

// initBuiltinFunctions initializes the built-in function registry.

func initBuiltinFunctions() {
	builtinFunctionsOnce.Do(func() {
		builtinFunctions = map[string]*FunctionDef{
			// Aggregation functions
			"sum":     {Name: "sum", MinArgs: 1, MaxArgs: 1, Impl: fnSum},
			"count":   {Name: "count", MinArgs: 1, MaxArgs: 1, Impl: fnCount},
			"average": {Name: "average", MinArgs: 1, MaxArgs: 1, Impl: fnAverage},
			"min":     {Name: "min", MinArgs: 1, MaxArgs: 1, Impl: fnMin},
			"max":     {Name: "max", MinArgs: 1, MaxArgs: 1, Impl: fnMax},

			// Array functions
			"map":      {Name: "map", MinArgs: 2, MaxArgs: 2, Impl: fnMap},
			"filter":   {Name: "filter", MinArgs: 2, MaxArgs: 2, Impl: fnFilter},
			"reduce":   {Name: "reduce", MinArgs: 2, MaxArgs: 3, Impl: fnReduce},
			"single":   {Name: "single", MinArgs: 1, MaxArgs: 2, Impl: fnSingle},
			"sort":     {Name: "sort", MinArgs: 1, MaxArgs: 2, Impl: fnSort},
			"append":   {Name: "append", MinArgs: 2, MaxArgs: 2, Impl: fnAppend},
			"reverse":  {Name: "reverse", MinArgs: 1, MaxArgs: 1, Impl: fnReverse},
			"distinct": {Name: "distinct", MinArgs: 1, MaxArgs: 1, Impl: fnDistinct},
			"shuffle":  {Name: "shuffle", MinArgs: 1, MaxArgs: 1, Impl: fnShuffle},
			"zip":      {Name: "zip", MinArgs: 1, MaxArgs: -1, Impl: fnZip},

			// String functions
			"string":          {Name: "string", MinArgs: 0, MaxArgs: 2, AcceptsContext: true, Impl: fnString},
			"length":          {Name: "length", MinArgs: 1, MaxArgs: 1, Impl: fnLength},
			"substring":       {Name: "substring", MinArgs: 2, MaxArgs: 3, Impl: fnSubstring},
			"uppercase":       {Name: "uppercase", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnUppercase},
			"lowercase":       {Name: "lowercase", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnLowercase},
			"trim":            {Name: "trim", MinArgs: 0, MaxArgs: 1, AcceptsContext: true, Impl: fnTrim},
			"contains":        {Name: "contains", MinArgs: 2, MaxArgs: 2, Impl: fnContains},
			"split":           {Name: "split", MinArgs: 2, MaxArgs: 3, Impl: fnSplit},
			"join":            {Name: "join", MinArgs: 1, MaxArgs: 2, Impl: fnJoin},
			"pad":             {Name: "pad", MinArgs: 2, MaxArgs: 3, Impl: fnPad},
			"substringBefore": {Name: "substringBefore", MinArgs: 2, MaxArgs: 2, AcceptsContext: true, Impl: fnSubstringBefore},
			"substringAfter":  {Name: "substringAfter", MinArgs: 2, MaxArgs: 2, AcceptsContext: true, Impl: fnSubstringAfter},

			// Type functions
			"type":    {Name: "type", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnType},
			"exists":  {Name: "exists", MinArgs: 1, MaxArgs: 1, Impl: fnExists},
			"number":  {Name: "number", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnNumber},
			"boolean": {Name: "boolean", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnBoolean},
			"not":     {Name: "not", MinArgs: 1, MaxArgs: 1, Impl: fnNot},

			// Math functions
			"abs":    {Name: "abs", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnAbs},
			"floor":  {Name: "floor", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnFloor},
			"ceil":   {Name: "ceil", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnCeil},
			"round":  {Name: "round", MinArgs: 1, MaxArgs: 2, AcceptsContext: true, Impl: fnRound},
			"sqrt":   {Name: "sqrt", MinArgs: 1, MaxArgs: 1, AcceptsContext: true, Impl: fnSqrt},
			"power":  {Name: "power", MinArgs: 2, MaxArgs: 2, Impl: fnPower},
			"random": {Name: "random", MinArgs: 0, MaxArgs: 0, Impl: fnRandom},

			// Object functions
			"each":   {Name: "each", MinArgs: 2, MaxArgs: 2, AcceptsContext: true, Impl: fnEach},
			"sift":   {Name: "sift", MinArgs: 2, MaxArgs: 2, AcceptsContext: true, Impl: fnSift},
			"keys":   {Name: "keys", MinArgs: 1, MaxArgs: 1, Impl: fnKeys},
			"lookup": {Name: "lookup", MinArgs: 2, MaxArgs: 2, Impl: fnLookup},
			"merge":  {Name: "merge", MinArgs: 1, MaxArgs: 1, Impl: fnMerge},
			"spread": {Name: "spread", MinArgs: 1, MaxArgs: 1, Impl: fnSpread},
			"error":  {Name: "error", MinArgs: 0, MaxArgs: 1, Impl: fnError},
			"assert": {Name: "assert", MinArgs: 1, MaxArgs: 2, Impl: fnAssert},
			"eval":   {Name: "eval", MinArgs: 0, MaxArgs: 2, Impl: fnEval},

			// Regex functions
			"match":   {Name: "match", MinArgs: 2, MaxArgs: 3, Impl: fnMatch},
			"replace": {Name: "replace", MinArgs: 3, MaxArgs: 4, Impl: fnReplace},

			// Date/Time functions
			"now":        {Name: "now", MinArgs: 0, MaxArgs: 2, Impl: fnNow},
			"millis":     {Name: "millis", MinArgs: 0, MaxArgs: 0, Impl: fnMillis},
			"fromMillis": {Name: "fromMillis", MinArgs: 1, MaxArgs: 3, Impl: fnFromMillis},
			"toMillis":   {Name: "toMillis", MinArgs: 1, MaxArgs: 2, Impl: fnToMillis},

			// Encoding functions
			"base64encode":       {Name: "base64encode", MinArgs: 0, MaxArgs: 1, Impl: fnBase64Encode},
			"base64decode":       {Name: "base64decode", MinArgs: 0, MaxArgs: 1, Impl: fnBase64Decode},
			"encodeUrl":          {Name: "encodeUrl", MinArgs: 1, MaxArgs: 1, Impl: fnEncodeUrl},
			"decodeUrl":          {Name: "decodeUrl", MinArgs: 1, MaxArgs: 1, Impl: fnDecodeUrl},
			"encodeUrlComponent": {Name: "encodeUrlComponent", MinArgs: 1, MaxArgs: 1, Impl: fnEncodeUrlComponent},
			"decodeUrlComponent": {Name: "decodeUrlComponent", MinArgs: 1, MaxArgs: 1, Impl: fnDecodeUrlComponent},

			// Number formatting functions
			"formatNumber":  {Name: "formatNumber", MinArgs: 1, MaxArgs: 3, Impl: fnFormatNumber},
			"formatBase":    {Name: "formatBase", MinArgs: 1, MaxArgs: 2, Impl: fnFormatBase},
			"formatInteger": {Name: "formatInteger", MinArgs: 1, MaxArgs: 2, Impl: fnFormatInteger},
			"parseInteger":  {Name: "parseInteger", MinArgs: 1, MaxArgs: 2, Impl: fnParseInteger},
		}
	})
}

// GetFunction retrieves a built-in function by name.

func GetFunction(name string) (*FunctionDef, bool) {
	initBuiltinFunctions()
	fn, ok := builtinFunctions[name]
	return fn, ok
}

// --- Aggregation Functions ---
