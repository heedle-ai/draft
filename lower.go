// Package draft — lower.go implements the Lower and Finalize phases of
// the translator: ResolvedAST → fragment → shuttle plan.
//
// Lower folds the resolved AST into a *fragment*: a partial plan with a
// known entry node (where upstream output flows in) and exit node (whose
// output is the fragment's result). Fragments compose:
//
//   - A single Call lowers to a one-node fragment (entry == exit).
//   - A Pipeline lowers each stage and chains them: stage[i]'s exit
//     becomes stage[i+1]'s required upstream. The fragment's entry is
//     the first stage's entry, its exit is the last stage's exit.
//
// Keeping a fragment (rather than building the Plan directly) makes
// lowerPipeline a clean fold over sub-fragments with well-defined
// splice points, and isolates node-ID generation. Finalize then flattens
// the fragment's accumulated nodes into a plan.Plan.
//
// Why two phases (Lower then Finalize) instead of one: Lower is pure
// agentscript (no shuttle types in its signature beyond what a node needs);
// Finalize is the single seam that produces the plan.Plan value. This
// keeps the shuttle dependency at one well-marked boundary.
package draft

import (
	"context"
	"fmt"
	"strconv"

	"github.com/heedle-ai/shuttle/plan"

	"github.com/heedle-ai/draft/ast"
	"github.com/heedle-ai/draft/resolved"
)

// fragment is a partial plan under construction. Nodes accumulate as the
// AST is folded; entries and exits mark the splice points for composition.
// A call has one of each; a par has one per branch.
type fragment struct {
	nodes []plan.Node
	// entries are the nodes that receive the fragment's upstream input.
	entries []plan.NodeID
	// exits are the nodes whose outputs are the fragment's result.
	exits []plan.NodeID
}

// Lowered is the output of Lower: the fragment plus the block's backend
// and mode (carried through for Finalize / future backends).
type Lowered struct {
	Fragment fragment
	Backend  ast.Backend
	Mode     ast.Mode
}

// Lower converts a resolved AST into a Lowered fragment. It handles a
// single block whose body is any nesting of calls, pipes and pars.
// Multiple blocks are rejected with a clear error.
func Lower(_ context.Context, in resolved.AST) (Lowered, error) {
	if len(in.Blocks) != 1 {
		return Lowered{}, fmt.Errorf("lower: expected exactly 1 block, got %d (multi-block not yet supported)", len(in.Blocks))
	}
	b := in.Blocks[0]

	g := &idgen{}
	frag, err := lowerNode(g, b.Body)
	if err != nil {
		return Lowered{}, err
	}
	return Lowered{Fragment: frag, Backend: b.Backend, Mode: b.Mode}, nil
}

// idgen issues stable, sequential node IDs (n0, n1, ...). Sequential IDs
// keep plans deterministic and readable in the Temporal UI.
type idgen struct{ n int }

func (g *idgen) next() plan.NodeID {
	id := plan.NodeID(fmt.Sprintf("n%d", g.n))
	g.n++
	return id
}

func lowerNode(g *idgen, n resolved.Node) (fragment, error) {
	switch node := n.(type) {
	case resolved.Call:
		return lowerCall(g, node), nil
	case resolved.Pipeline:
		return lowerPipeline(g, node)
	case resolved.Parallel:
		return lowerParallel(g, node)
	default:
		return fragment{}, fmt.Errorf("lower: unknown resolved node type %T", n)
	}
}

// lowerCall produces a one-node fragment. The node's activity is the
// builtin's resolved AgentID (the registered shuttle activity name); its
// args are the call's literal arguments stringified.
func lowerCall(g *idgen, c resolved.Call) fragment {
	id := g.next()
	node := plan.Node{
		ID:       id,
		Activity: c.Spec.AgentID,
		Args:     argStrings(c.Args),
	}
	return fragment{nodes: []plan.Node{node}, entries: []plan.NodeID{id}, exits: []plan.NodeID{id}}
}

// lowerPipeline folds the stages, making every entry of each stage require
// every exit of the stage before it. After a par, the next stage therefore
// waits for all branches and receives all their outputs (fan-in). The
// result's entries are the first stage's; its exits are the last stage's.
func lowerPipeline(g *idgen, p resolved.Pipeline) (fragment, error) {
	if len(p.Stages) == 0 {
		return fragment{}, fmt.Errorf("lower: empty pipeline")
	}

	var all []plan.Node
	var prevExits, entries []plan.NodeID

	for i, stage := range p.Stages {
		sf, err := lowerNode(g, stage)
		if err != nil {
			return fragment{}, fmt.Errorf("stage %d: %w", i, err)
		}
		if i == 0 {
			entries = sf.entries
		} else {
			for _, target := range sf.entries {
				for _, req := range prevExits {
					linkRequire(sf.nodes, target, req)
				}
			}
		}
		all = append(all, sf.nodes...)
		prevExits = sf.exits
	}

	return fragment{nodes: all, entries: entries, exits: prevExits}, nil
}

// lowerParallel places its branches side by side: no edges between them,
// so they share a layer and run concurrently. Each branch receives the
// par's upstream input, and the par's result is every branch's output.
func lowerParallel(g *idgen, p resolved.Parallel) (fragment, error) {
	if len(p.Branches) == 0 {
		return fragment{}, fmt.Errorf("lower: empty par")
	}
	var f fragment
	for i, branch := range p.Branches {
		bf, err := lowerNode(g, branch)
		if err != nil {
			return fragment{}, fmt.Errorf("branch %d: %w", i, err)
		}
		f.nodes = append(f.nodes, bf.nodes...)
		f.entries = append(f.entries, bf.entries...)
		f.exits = append(f.exits, bf.exits...)
	}
	return f, nil
}

// linkRequire adds `req` to the Requires of the node with id `target`
// within nodes. The entry node of a downstream fragment thereby depends
// on the exit of the upstream fragment.
func linkRequire(nodes []plan.Node, target, req plan.NodeID) {
	for i := range nodes {
		if nodes[i].ID == target {
			nodes[i].Requires = append(nodes[i].Requires, req)
			return
		}
	}
}

// argStrings converts resolved AST args into the []string a PlanNode
// carries. Numbers are formatted compactly; strings pass through.
func argStrings(args []ast.Arg) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		switch v := a.(type) {
		case ast.StringArg:
			out = append(out, v.Value)
		case ast.NumArg:
			out = append(out, strconv.FormatFloat(v.Value, 'g', -1, 64))
		}
	}
	return out
}

// === Finalize ==============================================================

// Finalize flattens a Lowered fragment into a plan.Plan. This is the
// single seam that produces the shuttle value. Only the temporal backend
// in static mode runs; memory and dynamic are rejected here.
func Finalize(_ context.Context, l Lowered) (plan.Plan, error) {
	if l.Backend != ast.BackendTemporal {
		return plan.Plan{}, fmt.Errorf("finalize: only the temporal backend is supported (got %v)", l.Backend)
	}
	if l.Mode != ast.ModeStatic {
		return plan.Plan{}, fmt.Errorf("finalize: only static mode is supported (got %v)", l.Mode)
	}
	return plan.Plan{Nodes: l.Fragment.nodes}, nil
}

// === Validate ==============================================================

// Validate runs plan.Plan.Validate over the finalized plan. It is a
// distinct phase so the pipeline reads Parse >>> Resolve >>> Lower >>>
// Finalize >>> Validate >>> Submit, and so validation failures are
// attributable to this step.
func Validate(_ context.Context, p plan.Plan) (plan.Plan, error) {
	if err := p.Validate(); err != nil {
		return plan.Plan{}, fmt.Errorf("validate: %w", err)
	}
	return p, nil
}
