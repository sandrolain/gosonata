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
	NodeFunction    NodeType = "function"    // Function call
	NodeLambda      NodeType = "lambda"      // Lambda function
	NodePartial     NodeType = "partial"     // Partial application
	NodePlaceholder NodeType = "placeholder" // ? placeholder for partial application

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
	StrValue string  // Pre-typed string value; set by parser for all string-valued nodes (eliminates .(string) assertions in evaluator)
	NumValue float64 // Pre-typed numeric value; set by parser for NodeNumber (eliminates .(float64) assertions in evaluator)
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
	Signature string // Function signature (e.g., "<n-n:n>")

	// Object constructor semantics
	IsGrouping bool // True for infix expr{...}, false for prefix {...} or path-applied

	// Error recovery
	Errors []error
}

// NewASTNode creates a new AST node of the specified type.
// Prefer NodeArena.Alloc when parsing to reduce per-node heap allocations.
func NewASTNode(nodeType NodeType, position int) *ASTNode {
	return &ASTNode{
		Type:     nodeType,
		Position: position,
	}
}

// arenaChunkSize is the number of ASTNode values pre-allocated per arena chunk.
// 64 nodes ≈ 12-13 KB; most JSONata expressions fit in a single chunk.
const arenaChunkSize = 64

// NodeArena is a bump-pointer allocator for ASTNode values.
//
// Instead of allocating each node individually on the heap (one GC-tracked
// object per node), the arena pre-allocates fixed-size chunks of ASTNode
// structs and returns pointers into them. A typical expression (< 64 nodes)
// requires only a single chunk allocation, reducing parse-time allocations
// by roughly N-1 where N is the node count.
//
// # Lifetime
//
// The arena MUST stay alive as long as any pointer returned by Alloc is
// reachable. Attaching the arena to the [Expression] achieves this: the GC
// collects the arena (and all its chunks) automatically when the Expression
// is released — including when it is evicted from the LRU cache.
//
// # Thread safety
//
// NodeArena is NOT thread-safe. Each [Parser] owns its own arena and the
// arena is never shared across goroutines.
type NodeArena struct {
	chunks [][]ASTNode
	pos    int // next free index in the last chunk
}

// NewNodeArena allocates an arena pre-warmed with one initial chunk.
func NewNodeArena() *NodeArena {
	return &NodeArena{
		chunks: [][]ASTNode{make([]ASTNode, arenaChunkSize)},
		pos:    0,
	}
}

// Alloc returns a pointer to a zero-valued ASTNode inside the arena,
// with Type and Position set. All other fields remain at their zero values
// (nil pointers, empty slices, false booleans) and must be filled by the caller.
func (a *NodeArena) Alloc(nodeType NodeType, position int) *ASTNode {
	if a.pos >= arenaChunkSize {
		// Current chunk exhausted — allocate a new one.
		a.chunks = append(a.chunks, make([]ASTNode, arenaChunkSize))
		a.pos = 0
	}
	n := &a.chunks[len(a.chunks)-1][a.pos]
	a.pos++
	// Zero-init the node (re-use of pool'd memory is safe because chunks are
	// freshly make()'d and pos advances monotonically; nodes are never recycled).
	n.Type = nodeType
	n.Position = position
	return n
}

// String returns a string representation of the node type.
func (n *ASTNode) String() string {
	return string(n.Type)
}
