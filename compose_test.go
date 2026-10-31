package draft_test

import (
	"context"
	"testing"

	"github.com/heedle-ai/shuttle/plan"
	"github.com/heedle-ai/weft"

	"github.com/heedle-ai/draft"
)

// These tests assert the COMPOSITION property: the compile pipeline is a
// composed weft.Arrow, and it can be composed further into larger arrows
// just like any other. If the phases ever stop being composable arrows,
// these won't compile — which is the point.

func TestCompileArrow_IsAComposableArrow(t *testing.T) {
	reg := draft.DefaultRegistry()

	// CompileArrow returns a weft.Arrow[Source, Plan]. Assigning it to the
	// arrow type is a compile-time proof of its shape.
	var compile weft.Arrow[draft.Source, plan.Plan] = draft.CompileArrow(reg)

	compiled, err := compile(context.Background(), `(block :backend temporal :mode static
	  (pipe
	    (echo "hi")
	    echo))`)
	if err != nil {
		t.Fatalf("compile arrow: %v", err)
	}
	if len(compiled.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(compiled.Nodes))
	}
}

func TestCompileArrow_ComposesIntoLargerArrow(t *testing.T) {
	reg := draft.DefaultRegistry()

	// Compose CompileArrow with a downstream arrow that counts nodes.
	// This is the real test of composability: the compile pipeline is
	// just an arrow, so it slots into Pipe like anything else.
	countNodes := weft.Arrow[plan.Plan, int](func(_ context.Context, p plan.Plan) (int, error) {
		return len(p.Nodes), nil
	})

	sourceToCount := weft.Pipe2(draft.CompileArrow(reg), countNodes)

	n, err := sourceToCount(context.Background(), `(block :backend temporal :mode static
	  (pipe
	    (echo "a")
	    (echo "b")
	    (echo "c")))`)
	if err != nil {
		t.Fatalf("composed arrow: %v", err)
	}
	if n != 3 {
		t.Errorf("composed pipeline counted %d nodes, want 3", n)
	}
}

func TestCompileArrow_ErrorShortCircuits(t *testing.T) {
	reg := draft.DefaultRegistry()
	compile := draft.CompileArrow(reg)

	// Unknown builtin: the error must propagate out of the composition
	// (Compose short-circuits on the first failing arrow), not panic or
	// produce a partial plan.
	_, err := compile(context.Background(), `(block :backend temporal :mode static
	  (teleport "mars"))`)
	if err == nil {
		t.Fatal("expected composition to short-circuit with an error")
	}
}
