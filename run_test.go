package draft_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"go.temporal.io/sdk/testsuite"

	"github.com/heedle-ai/draft"
	"github.com/heedle-ai/shuttle/plan"
)

// requires returns each node's Requires, sorted, keyed by node ID.
func requires(p plan.Plan) map[plan.NodeID][]plan.NodeID {
	out := map[plan.NodeID][]plan.NodeID{}
	for _, n := range p.Nodes {
		r := append([]plan.NodeID(nil), n.Requires...)
		sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
		out[n.ID] = r
	}
	return out
}

func compileOK(t *testing.T, src string) plan.Plan {
	t.Helper()
	p, err := draft.Compile(context.Background(), draft.DefaultRegistry(), draft.Source(src))
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return p
}

// run executes p through shuttle's real PlanWorkflow in Temporal's
// in-process test environment.
func run(t *testing.T, p plan.Plan) plan.Result {
	t.Helper()
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	plan.Register(env)
	env.ExecuteWorkflow(plan.WorkflowName, p)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	var res plan.Result
	if err := env.GetWorkflowResult(&res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestPar_FanOutThenFanIn(t *testing.T) {
	p := compileOK(t, `(pipe (par (echo "a") (echo "b")) echo)`)

	want := map[plan.NodeID][]plan.NodeID{"n0": nil, "n1": nil, "n2": {"n0", "n1"}}
	if got := requires(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("requires = %v, want %v", got, want)
	}
	layers, _ := p.Layers()
	if len(layers) != 2 || len(layers[0]) != 2 {
		t.Fatalf("layers = %v, want [[n0 n1] [n2]]", layers)
	}

	res := run(t, p)
	if got := res.Outputs["n2"]; got != "a b" {
		t.Errorf("n2 = %q, want %q", got, "a b")
	}
}

func TestPar_BranchesShareUpstream(t *testing.T) {
	// Both branches of a par after a stage receive that stage's output.
	p := compileOK(t, `(pipe (echo "x") (par echo echo))`)
	want := map[plan.NodeID][]plan.NodeID{"n0": nil, "n1": {"n0"}, "n2": {"n0"}}
	if got := requires(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("requires = %v, want %v", got, want)
	}
	res := run(t, p)
	if res.Outputs["n1"] != "x" || res.Outputs["n2"] != "x" {
		t.Errorf("outputs = %v, want n1 and n2 = x", res.Outputs)
	}
	if !reflect.DeepEqual(res.Leaves, []string{"n1", "n2"}) {
		t.Errorf("leaves = %v, want [n1 n2]", res.Leaves)
	}
}

func TestPar_NestedPipelines(t *testing.T) {
	p := compileOK(t, `(pipe (par (pipe (echo "x") echo) (echo "y")) echo)`)
	// n0 -> n1 is the first branch; n2 the second; n3 joins n1 and n2.
	want := map[plan.NodeID][]plan.NodeID{"n0": nil, "n1": {"n0"}, "n2": nil, "n3": {"n1", "n2"}}
	if got := requires(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("requires = %v, want %v", got, want)
	}
	if got := run(t, p).Outputs["n3"]; got != "x y" {
		t.Errorf("n3 = %q, want %q", got, "x y")
	}
}

func TestMemoryBackendIsRejected(t *testing.T) {
	_, err := draft.Compile(context.Background(), draft.DefaultRegistry(),
		draft.Source(`(block :backend memory (echo "a"))`))
	if err == nil {
		t.Fatal("want an error for :backend memory")
	}
}
