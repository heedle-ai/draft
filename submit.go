// Package draft — submit.go provides the Submit phase and the
// end-to-end Compile/Run entry points that compose every phase.
//
// The full translator pipeline:
//
//	Source >>> Parse >>> Resolve >>> Lower >>> Finalize >>> Validate >>> Submit
//
// Compile runs everything up to (not including) Submit, producing a
// validated plan.Plan the caller can submit, inspect, or serialize.
// Run does Compile then Submit, returning a workflow handle. Both are the
// library entry points a front end calls.
package draft

import (
	"context"

	"go.temporal.io/sdk/client"

	"github.com/heedle-ai/shuttle/plan"
	"github.com/heedle-ai/weft"

	"github.com/heedle-ai/draft/ast"
	"github.com/heedle-ai/draft/registry"
	"github.com/heedle-ai/draft/resolved"
)

// Compile runs Parse >=> Resolve >=> Lower >=> Finalize >=> Validate,
// producing a validated plan.Plan. It does not submit.
//
// The pipeline is built by composing weft.Arrow values with weft.Pipe —
// not by hand-threading errors. Each phase is an Arrow[A,B]; composition
// is type-checked at every seam (a phase whose output type doesn't match
// the next phase's input simply won't compile), and weft.Compose handles
// the error short-circuiting. This is the "everything composes" contract
// made literal: Compile IS the composition of the phase arrows.
//
// Compile is the heart of the "draft as serialization layer" idea:
// source text in, validated executable Plan out, with unknown builtins,
// arity/type errors (Resolve) and malformed graphs (Validate) all
// rejected before anything runs.
func Compile(ctx context.Context, reg *registry.Registry, src Source) (plan.Plan, error) {
	return CompileArrow(reg)(ctx, src)
}

// CompileArrow returns the compile pipeline as a single composed
// weft.Arrow[Source, plan.Plan], curried over the registry. Callers that
// want to compose compilation into a larger arrow (or submit via
// SubmitWith) use this directly; Compile is the convenience that applies
// it.
func CompileArrow(reg *registry.Registry) weft.Arrow[Source, plan.Plan] {
	return weft.Pipe5(
		weft.Arrow[Source, ast.AST](Parse),
		weft.Arrow[ast.AST, resolved.AST](ResolveWith(reg)),
		weft.Arrow[resolved.AST, Lowered](Lower),
		weft.Arrow[Lowered, plan.Plan](Finalize),
		weft.Arrow[plan.Plan, plan.Plan](Validate),
	)
}

// Submit starts a compiled plan as shuttle's PlanWorkflow and returns the
// run handle without waiting for the result. opts.TaskQueue is required; an
// empty opts.ID lets Temporal choose one. Give opts.ID a value derived from
// the work (with a USE_EXISTING conflict policy) to run the same work once.
func Submit(ctx context.Context, c client.Client, p plan.Plan, opts client.StartWorkflowOptions) (client.WorkflowRun, error) {
	return plan.Submit(ctx, c, p, opts)
}

// SubmitWith curries Submit over a client and start options, giving an
// arrow from a plan to a run handle, so compilation and submission compose:
//
//	run := weft.Compose(draft.CompileArrow(reg), draft.SubmitWith(c, opts))
func SubmitWith(c client.Client, opts client.StartWorkflowOptions) weft.Arrow[plan.Plan, client.WorkflowRun] {
	return func(ctx context.Context, p plan.Plan) (client.WorkflowRun, error) {
		return Submit(ctx, c, p, opts)
	}
}
