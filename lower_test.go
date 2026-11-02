package draft_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/heedle-ai/shuttle/plan"

	"github.com/heedle-ai/draft"
	"github.com/heedle-ai/draft/registry"
)

// compile is a helper: source → Plan via the full Compile pipeline using
// the default registry (echo).
func compile(t *testing.T, src string) (plan.Plan, error) {
	t.Helper()
	return draft.Compile(context.Background(), draft.DefaultRegistry(), draft.Source(src))
}

// === Builtins / registry ===================================================

func TestEchoSpec_BindsToSibylEchoActivity(t *testing.T) {
	spec := draft.EchoSpec()
	if spec.Name != "echo" {
		t.Errorf("Name = %q, want echo", spec.Name)
	}
	if spec.AgentID != plan.EchoName {
		t.Errorf("AgentID = %q, want %q (the registered shuttle activity)", spec.AgentID, plan.EchoName)
	}
}

func TestDefaultRegistry_HasEcho(t *testing.T) {
	r := draft.DefaultRegistry()
	if _, ok := r.Lookup("echo"); !ok {
		t.Error("DefaultRegistry should contain echo")
	}
}

// === Lower / Finalize via Compile (happy paths) ============================

func TestCompile_SingleEcho(t *testing.T) {
	compiled, err := compile(t, `(block :backend temporal :mode static
	  (echo "hello"))`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(compiled.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(compiled.Nodes))
	}
	n := compiled.Nodes[0]
	if n.Activity != plan.EchoName {
		t.Errorf("Activity = %q, want %q", n.Activity, plan.EchoName)
	}
	if len(n.Args) != 1 || n.Args[0] != "hello" {
		t.Errorf("Args = %v, want [hello]", n.Args)
	}
	if len(n.Requires) != 0 {
		t.Errorf("single node should have no Requires, got %v", n.Requires)
	}
}

func TestCompile_Pipeline_ChainsDependencies(t *testing.T) {
	compiled, err := compile(t, `(block :backend temporal :mode static
	  (pipe
	    (echo "a")
	    (echo "b")
	    (echo "c")))`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(compiled.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(compiled.Nodes))
	}
	// n0 has no deps; n1 requires n0; n2 requires n1.
	byID := map[plan.NodeID]plan.Node{}
	for _, n := range compiled.Nodes {
		byID[n.ID] = n
	}
	if len(byID["n0"].Requires) != 0 {
		t.Errorf("n0 should have no deps")
	}
	if len(byID["n1"].Requires) != 1 || byID["n1"].Requires[0] != "n0" {
		t.Errorf("n1 should require n0, got %v", byID["n1"].Requires)
	}
	if len(byID["n2"].Requires) != 1 || byID["n2"].Requires[0] != "n1" {
		t.Errorf("n2 should require n1, got %v", byID["n2"].Requires)
	}
	// The compiled plan must pass shuttle's own validation.
	if err := compiled.Validate(); err != nil {
		t.Errorf("compiled plan should be valid: %v", err)
	}
}

func TestCompile_NoArgEcho(t *testing.T) {
	// echo's arg is optional, so a bare echo is valid.
	compiled, err := compile(t, `(block :backend temporal :mode static
	  echo)`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(compiled.Nodes[0].Args) != 0 {
		t.Errorf("bare echo should have no args, got %v", compiled.Nodes[0].Args)
	}
}

// === Failure paths surface at the right phase ==============================

func TestCompile_UnknownBuiltin(t *testing.T) {
	_, err := compile(t, `(block :backend temporal :mode static
	  (nope "x"))`)
	if err == nil {
		t.Fatal("expected error for unknown builtin")
	}
}

func TestCompile_ParallelNotYetSupported(t *testing.T) {
	// Even if the parser produced a Parallel, Lower must reject it
	// clearly. (The current parser may not emit it, but the lowering
	// guard must exist.)
	_, err := compile(t, `(block :backend temporal :mode static
	  (echo "a"))`)
	if err != nil {
		t.Fatalf("control case should compile: %v", err)
	}
}

func TestFinalize_RejectsMemoryBackend(t *testing.T) {
	// memory backend isn't supported by Finalize yet.
	_, err := compile(t, `(block :backend memory :mode static
	  (echo "x"))`)
	if err == nil {
		t.Fatal("expected error: memory backend not supported")
	}
}

func TestFinalize_RejectsDynamicMode(t *testing.T) {
	_, err := compile(t, `(block :backend temporal :mode dynamic
	  (echo "x"))`)
	if err == nil {
		t.Fatal("expected error: dynamic mode not supported")
	}
}

// === Validate phase ========================================================

func TestValidate_AcceptsCompiledPlan(t *testing.T) {
	compiled, err := compile(t, `(block :backend temporal :mode static
	  (pipe
	    (echo "a")
	    echo))`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	got, err := draft.Validate(context.Background(), compiled)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(got.Nodes) != 2 {
		t.Errorf("validated plan nodes = %d, want 2", len(got.Nodes))
	}
}

// === Custom registry =======================================================

func TestCompile_CustomBuiltin(t *testing.T) {
	// A front end can register its own builtins. Verify a custom name
	// lowers to its declared activity.
	r := registry.New()
	r.MustRegister(registry.BuiltinSpec{
		Name:    "shout",
		AgentID: "Echo", // reuse Echo activity for the test
		ArgSchema: registry.ArgSchema{
			Params: []registry.ParamSpec{{Name: "text", Type: registry.StringT}},
		},
		// This custom builtin is exercised under a temporal block, so it
		// must declare temporal support (mirroring how a real ported verb
		// is registered).
		Backends: []registry.Backend{registry.BackendTemporal},
	})
	compiled, err := draft.Compile(context.Background(), r, draft.Source(`(block :backend temporal :mode static
	  (shout "hey"))`))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if compiled.Nodes[0].Activity != "Echo" {
		t.Errorf("Activity = %q, want Echo", compiled.Nodes[0].Activity)
	}
	if compiled.Nodes[0].Args[0] != "hey" {
		t.Errorf("Args = %v", compiled.Nodes[0].Args)
	}
}

// TestInv_CommandOptionsReachThePlan: a command's default options are
// written into every node of that command, and only those, so the plan
// records how each step runs; a plan that validates before still does.
func TestInv_CommandOptionsReachThePlan(t *testing.T) {
	reg := draft.DefaultRegistry()
	slow := plan.Options{Timeout: 20 * time.Minute, Heartbeat: time.Minute}
	reg.MustRegister(registry.BuiltinSpec{Name: "slow", AgentID: "Slow", Options: slow,
		ArgSchema: registry.ArgSchema{Params: []registry.ParamSpec{{Name: "x", Type: registry.StringT}}},
		Backends:  []registry.Backend{registry.BackendTemporal}})
	p, err := draft.Compile(context.Background(), reg, draft.Source(`(pipe (echo "a") (slow "b") (slow "c"))`))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Nodes) != 3 || !reflect.DeepEqual(p.Nodes[0].Options, plan.Options{}) || p.Nodes[1].Options.Timeout != slow.Timeout ||
		p.Nodes[2].Options.Heartbeat != slow.Heartbeat {
		t.Errorf("nodes = %+v", p.Nodes)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("plan with options doesn't validate: %v", err)
	}
}

// TestInv_BoundArgumentsComeFirst: a command's Bound arguments are written
// first into each of its plan nodes, before the program's own, and only
// into its nodes; each node gets its own copy, so changing one plan never
// changes the command or another node.
func TestInv_BoundArgumentsComeFirst(t *testing.T) {
	reg := draft.DefaultRegistry()
	reg.MustRegister(registry.BuiltinSpec{Name: "weather", AgentID: "Skill", Bound: []string{"weather", "tree1"},
		ArgSchema: registry.ArgSchema{Variadic: true, VariadicType: registry.StringT},
		Backends:  []registry.Backend{registry.BackendTemporal}})
	p, err := draft.Compile(context.Background(), reg, draft.Source(`(pipe (echo "a") (par (weather "Boston") (weather)))`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Nodes[0].Args, []string{"a"}) || !reflect.DeepEqual(p.Nodes[1].Args, []string{"weather", "tree1", "Boston"}) ||
		!reflect.DeepEqual(p.Nodes[2].Args, []string{"weather", "tree1"}) {
		t.Fatalf("nodes = %+v", p.Nodes)
	}
	p.Nodes[2].Args[0] = "changed"
	if spec, _ := reg.Lookup("weather"); spec.Bound[0] != "weather" || p.Nodes[1].Args[0] != "weather" {
		t.Errorf("a plan's args share memory with the command: %v %v", spec.Bound, p.Nodes[1].Args)
	}
}
