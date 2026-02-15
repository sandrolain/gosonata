package types

// NodeType identifies the type of an AST node.
type NodeType string

// Null represents a JSONata null literal distinct from undefined (nil).
type Null struct{}

// MarshalJSON implements json.Marshaler for Null.
// This ensures that Null serializes to JSON null instead of {}.
func (Null) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

// NullValue is the singleton value used for JSONata null.
var NullValue = Null{}

// AST node types based on JSONata reference implementation.
const (
	// Literals
	NodeString  NodeType = "string"
	NodeNumber  NodeType = "number"
	NodeBoolean NodeType = "value" // true/false
	NodeNull    NodeType = "value" // null

	// Navigation
	NodePath       NodeType = "path"       // Field navigation
	NodeName       NodeType = "name"       // Identifier
	NodeWildcard   NodeType = "wildcard"   // *
	NodeDescendant NodeType = "descendant" // **
	NodeParent     NodeType = "parent"     // %

	// Operators
	NodeBinary NodeType = "binary" // +, -, *, /, =, etc.
	NodeUnary  NodeType = "unary"  // -, not

	// Functions
	NodeFunction NodeType = "function" // Function call
	NodeLambda   NodeType = "lambda"   // Lambda function
	NodePartial  NodeType = "partial"  // Partial application

	// Control flow
	NodeCondition NodeType = "condition" // ? : operator
	NodeBlock     NodeType = "block"     // ( ; ; )
	NodeBind      NodeType = "bind"      // := assignment

	// Constructors
	NodeArray  NodeType = "array"  // [...]
	NodeObject NodeType = "object" // {...}

	// Special
	NodeRegex     NodeType = "regex"     // /pattern/flags
	NodeVariable  NodeType = "variable"  // $var
	NodeTransform NodeType = "transform" // |...|...|
	NodeSort      NodeType = "sort"      // ^(...)
	NodeFilter    NodeType = "filter"    // [...]
	NodeContext   NodeType = "context"   // @
	NodeIndex     NodeType = "index"     // #
	NodeRange     NodeType = "range"     // .. (range operator)
	NodeApply     NodeType = "apply"     // ~> (chain operator)
)

// ASTNode represents a node in the Abstract Syntax Tree.
type ASTNode struct {
	Type     NodeType
	Value    interface{}
	Position int

	// Relations
	LHS         *ASTNode   // Left-hand side (binary ops, paths)
	RHS         *ASTNode   // Right-hand side (binary ops)
	Steps       []*ASTNode // Path steps
	Arguments   []*ASTNode // Function arguments
	Expressions []*ASTNode // Block expressions

	// Attributes
	KeepArray bool   // Preserve array structure
	ConsArray bool   // Force array construction
	Stage     string // Pipeline stage identifier
	Index     int    // Array index (for optimization)

	// Object constructor semantics
	IsGrouping bool // True for infix expr{...}, false for prefix {...} or path-applied

	// Error recovery
	Errors []error
}

// NewASTNode creates a new AST node of the specified type.
func NewASTNode(nodeType NodeType, position int) *ASTNode {
	return &ASTNode{
		Type:     nodeType,
		Position: position,
	}
}

// String returns a string representation of the node type.
func (n *ASTNode) String() string {
	return string(n.Type)
}
