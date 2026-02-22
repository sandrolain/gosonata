package evaluator

import (
	"fmt"
)

// EvalContext maintains evaluation state including variable bindings and current data.
type EvalContext struct {
	// data is the current context data ($)
	data interface{}

	// parent is the parent context
	parent *EvalContext

	// root is the root context ($$ always refers to this)
	root *EvalContext

	// bindings stores variable assignments
	bindings map[string]interface{}

	// depth tracks recursion depth to prevent stack overflow
	depth int

	// isArrayItem marks this context as created during array iteration in a path.
	// Only contexts created this way are valid targets for the % (parent) operator.
	isArrayItem bool
}

// NewContext creates a new evaluation context.
func NewContext(data interface{}) *EvalContext {
	ctx := &EvalContext{
		data:  data,
		depth: 0,
		// bindings is nil; allocated lazily by SetBinding/SetBindings.
	}
	// Root context points to itself
	ctx.root = ctx
	return ctx
}

// NewChildContext creates a child context with new data.
func (c *EvalContext) NewChildContext(data interface{}) *EvalContext {
	return &EvalContext{
		data:   data,
		parent: c,
		root:   c.root,
		depth:  c.depth + 1,
		// bindings is nil; allocated lazily.
	}
}

// NewArrayItemContext creates a child context for array iteration.
// Contexts created this way are valid targets for the % (parent) operator.
func (c *EvalContext) NewArrayItemContext(data interface{}) *EvalContext {
	return &EvalContext{
		data:        data,
		parent:      c,
		root:        c.root,
		depth:       c.depth + 1,
		isArrayItem: true,
		// bindings is nil; allocated lazily.
	}
}

// IsArrayItem returns whether this context was created during array iteration.
func (c *EvalContext) IsArrayItem() bool {
	return c.isArrayItem
}

// Data returns the current context data.
func (c *EvalContext) Data() interface{} {
	return c.data
}

// Parent returns the parent context.
func (c *EvalContext) Parent() *EvalContext {
	return c.parent
}

// Root returns the root context ($$ always refers to this).
func (c *EvalContext) Root() *EvalContext {
	return c.root
}

// Depth returns the current recursion depth.
func (c *EvalContext) Depth() int {
	return c.depth
}

// SetBinding sets a variable binding.
func (c *EvalContext) SetBinding(name string, value interface{}) {
	if c.bindings == nil {
		c.bindings = make(map[string]interface{})
	}
	c.bindings[name] = value
}

// GetBinding retrieves a variable binding.
// It searches the current context and parent contexts.
func (c *EvalContext) GetBinding(name string) (interface{}, bool) {
	// Check current context
	if value, ok := c.bindings[name]; ok {
		return value, true
	}

	// Check parent context
	if c.parent != nil {
		return c.parent.GetBinding(name)
	}

	return nil, false
}

// SetBindings sets multiple variable bindings at once.
func (c *EvalContext) SetBindings(bindings map[string]interface{}) {
	if len(bindings) == 0 {
		return
	}
	if c.bindings == nil {
		c.bindings = make(map[string]interface{}, len(bindings))
	}
	for name, value := range bindings {
		c.bindings[name] = value
	}
}

// Clone creates a shallow copy of the context with the same bindings.
func (c *EvalContext) Clone() *EvalContext {
	var newBindings map[string]interface{}
	if len(c.bindings) > 0 {
		newBindings = make(map[string]interface{}, len(c.bindings))
		for k, v := range c.bindings {
			newBindings[k] = v
		}
	}

	return &EvalContext{
		data:     c.data,
		parent:   c.parent,
		root:     c.root, // Preserve root reference
		bindings: newBindings,
		depth:    c.depth,
	}
}

// CloneDeeper creates a shallow copy with depth incremented by 1.
// Used for lambda calls to track recursion depth.
func (c *EvalContext) CloneDeeper() *EvalContext {
	cloned := c.Clone()
	cloned.depth++
	return cloned
}

// String returns a string representation of the context.
func (c *EvalContext) String() string {
	return fmt.Sprintf("Context{depth=%d, bindings=%d}", c.depth, len(c.bindings))
}
