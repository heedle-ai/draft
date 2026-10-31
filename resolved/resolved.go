// Package resolved holds the post-resolution AST. It mirrors the shape
// of package ast but every Call carries the BuiltinSpec it resolved to.
//
// Keeping a separate type (rather than mutating ast.Call to hold an
// optional spec) preserves the invariant that ast values are pure
// syntax and resolved values are syntax + registry binding. The Lower
// phase consumes ResolvedAST, never the raw AST, so it can rely on every
// call being resolved.
package resolved

import (
	"github.com/heedle-ai/draft/ast"
	"github.com/heedle-ai/draft/registry"
)

// AST is a fully resolved program: a list of resolved blocks.
type AST struct {
	Blocks []Block
}

// Block carries the same backend/mode annotations as ast.Block, with a
// resolved body.
type Block struct {
	Backend ast.Backend
	Mode    ast.Mode
	Body    Node
}

// Node is the sealed sum of resolved expression nodes. Mirrors ast.Node.
type Node interface {
	resolvedNode()
}

// Pipeline is a resolved sequence of stages.
type Pipeline struct {
	Stages []Node
}

// Parallel is a resolved fan-out: its branches run concurrently on the
// same upstream input.
type Parallel struct {
	Branches []Node
}

// Call is a resolved builtin invocation: the original name and args,
// plus the BuiltinSpec it resolved to.
type Call struct {
	Name string
	Args []ast.Arg
	Spec registry.BuiltinSpec
}

func (Pipeline) resolvedNode() {}
func (Parallel) resolvedNode() {}
func (Call) resolvedNode()     {}
