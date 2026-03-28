package types

// FastPathKind classifies an expression for zero-copy fast-path evaluation.
//
// During compilation, the parser performs a static analysis pass on the AST
// to determine whether an expression can be evaluated directly against raw
// JSON bytes, without a full json.Unmarshal. Classified expressions are
// stored in [Expression.FastPath] and consulted at evaluation time.
//
// The three supported kinds are:
//   - [FastPathPurePath]: simple dot-separated field access ("a.b.c")
//   - [FastPathComparison]: equality/inequality against a literal ("a.b = \"x\"")
//   - [FastPathFunc]: a stdlib function applied to a pure path ("$exists(a.b)")
type FastPathKind uint8

const (
	// FastPathNone indicates the expression requires full AST evaluation.
	FastPathNone FastPathKind = iota

	// FastPathPurePath indicates the expression is a simple dot-separated
	// field navigation ("a.b.c"). The field is extracted via gjson with
	// zero-copy string views — the JSON document is never fully parsed.
	FastPathPurePath

	// FastPathComparison indicates the expression is a simple equality or
	// inequality check between a pure path and a JSON literal
	// (e.g. "user.role = \"admin\"" or "status != \"inactive\"").
	// Evaluated with 0–1 heap allocations.
	FastPathComparison

	// FastPathFunc indicates the expression calls a supported stdlib function
	// on a pure path (e.g. "$exists(a.b)", "$lowercase(name)"). The field is
	// extracted with a single gjson call and the function is applied directly.
	FastPathFunc
)

// FuncFastKind identifies which stdlib function is used in a [FastPathFunc] expression.
type FuncFastKind uint8

const (
	FuncFastExists   FuncFastKind = iota // $exists
	FuncFastContains                      // $contains
	FuncFastString                        // $string
	FuncFastBoolean                       // $boolean
	FuncFastNumber                        // $number
	FuncFastKeys                          // $keys
	FuncFastDistinct                      // $distinct
	FuncFastNot                           // $not
	FuncFastLowercase                     // $lowercase
	FuncFastUppercase                     // $uppercase
	FuncFastTrim                          // $trim
	FuncFastLength                        // $length
	FuncFastType                          // $type
	FuncFastAbs                           // $abs
	FuncFastFloor                         // $floor
	FuncFastCeil                          // $ceil
	FuncFastSqrt                          // $sqrt
	FuncFastCount                         // $count
	FuncFastReverse                       // $reverse
	FuncFastSum                           // $sum
	FuncFastMax                           // $max
	FuncFastMin                           // $min
	FuncFastAverage                       // $average
)

// FastPathInfo holds the result of the compile-time fast-path classification.
// It is embedded in [Expression] and consumed by [evaluator.EvalFast].
//
// Only one of the variant fields (CompareOp/CompareVal or FuncKind/FuncArg)
// is meaningful, depending on [Kind].
type FastPathInfo struct {
	// Kind is the classification of this expression.
	Kind FastPathKind

	// GJSONPath is the gjson-formatted path string for the root field.
	// Valid for all three fast-path kinds.
	GJSONPath string

	// CompareOp is the comparison operator ("=" or "!=").
	// Only meaningful when Kind == FastPathComparison.
	CompareOp string

	// CompareVal is the raw JSON literal to compare against (e.g. `"admin"`, `42`, `true`).
	// Only meaningful when Kind == FastPathComparison.
	CompareVal string

	// FuncKind identifies which stdlib function to apply.
	// Only meaningful when Kind == FastPathFunc.
	FuncKind FuncFastKind

	// FuncArg is the optional second argument literal (e.g. the substring for $contains).
	// Only meaningful when Kind == FastPathFunc and the function accepts two arguments.
	FuncArg string
}
