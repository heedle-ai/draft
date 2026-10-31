package draft_test

import (
	"context"
	"strings"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/heedle-ai/draft"
	"github.com/heedle-ai/draft/registry"
	"github.com/heedle-ai/shuttle"
	"github.com/heedle-ai/shuttle/plan"
	"github.com/heedle-ai/weft"
)

// TestCustomCommand is the wiki's "Adding a command" example: a weft
// arrow becomes a shuttle activity and a draft command, then runs.
func TestCustomCommand(t *testing.T) {
	// 1. The behaviour, as a weft arrow from plan.Input to string.
	shout := weft.Pure(func(in plan.Input) string {
		var parts []string
		for _, v := range in.Upstream {
			parts = append(parts, v)
		}
		return strings.ToUpper(strings.Join(parts, " "))
	})

	// 2. The DSL name, mapped to an activity name.
	reg := draft.DefaultRegistry()
	reg.MustRegister(registry.BuiltinSpec{
		Name:     "shout",
		AgentID:  "Shout",
		Backends: []registry.Backend{registry.BackendTemporal},
	})

	// 3. Compile.
	p, err := draft.Compile(context.Background(), reg, draft.Source(`(pipe (echo "hello") shout)`))
	if err != nil {
		t.Fatal(err)
	}

	// 4. Run: the worker registers the arrow under the same activity name.
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	plan.Register(env)
	env.RegisterActivityWithOptions(shuttle.ArrowAsActivity(shout), activity.RegisterOptions{Name: "Shout"})
	env.ExecuteWorkflow(plan.WorkflowName, p)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var res plan.Result
	if err := env.GetWorkflowResult(&res); err != nil {
		t.Fatal(err)
	}
	if got := res.Outputs["n1"]; got != "HELLO" {
		t.Fatalf("n1 = %q, want HELLO", got)
	}
}

// Without Backends, a command is memory-only and draft refuses it.
func TestCustomCommand_NeedsTemporalBackend(t *testing.T) {
	reg := draft.DefaultRegistry()
	reg.MustRegister(registry.BuiltinSpec{Name: "shout", AgentID: "Shout"})
	_, err := draft.Compile(context.Background(), reg, draft.Source(`(pipe (echo "hello") shout)`))
	if err == nil {
		t.Fatal("want an error for a command without BackendTemporal")
	}
	t.Log(err)
}
