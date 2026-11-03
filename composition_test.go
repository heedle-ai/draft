package draft_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/heedle-ai/draft"
	"github.com/heedle-ai/shuttle/plan"
)

// fromShape derives what each step waits for from a shape alone: a pipe's
// stage waits for every exit of the stage before it; a par's branches
// share their input. It returns the shape's steps in order, its entries
// and exits, and adds to waits.
func fromShape(s draft.Shape, waits map[plan.NodeID][]plan.NodeID) (steps, entries, exits []plan.NodeID) {
	switch s.Kind {
	case "step":
		return []plan.NodeID{s.Node}, []plan.NodeID{s.Node}, []plan.NodeID{s.Node}
	case "pipe":
		var prev []plan.NodeID
		for i, p := range s.Parts {
			st, en, ex := fromShape(p, waits)
			steps = append(steps, st...)
			if i == 0 {
				entries = en
			} else {
				for _, e := range en {
					waits[e] = append(waits[e], prev...)
				}
			}
			prev = ex
		}
		return steps, entries, prev
	case "par":
		for _, p := range s.Parts {
			st, en, ex := fromShape(p, waits)
			steps, entries, exits = append(steps, st...), append(entries, en...), append(exits, ex...)
		}
		return steps, entries, exits
	}
	panic("unknown kind " + s.Kind)
}

// TestLaw_CompositionMatchesPlan: a program's shape names exactly its
// plan's nodes, in plan order, each once; and what each step waits for,
// derived from the shape alone, is what its plan node requires. So a
// drawing of the shape shows the plan, no more and no less.
func TestLaw_CompositionMatchesPlan(t *testing.T) {
	reg := draft.DefaultRegistry()
	for _, src := range []string{
		`(echo "a")`,
		`(pipe (echo "a") echo)`,
		`(par (echo "a") (echo "b"))`,
		`(pipe (par (echo "a") (echo "b")) echo)`,
		`(pipe (par (echo "a") (echo "b")) (par echo (echo "c")) (par echo (echo "d")) echo)`,
		`(pipe (echo "x") (par (pipe echo echo) (par echo (pipe (echo "y") echo))) (par echo echo) echo)`,
		`(par (pipe (echo "a") (par echo echo)) (echo "b") (pipe (echo "c") echo))`,
	} {
		p, err := draft.Compile(context.Background(), reg, draft.Source(src))
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		shape, err := draft.Composition(context.Background(), reg, draft.Source(src))
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		waits := map[plan.NodeID][]plan.NodeID{}
		steps, _, _ := fromShape(shape, waits)
		var ids []plan.NodeID
		for _, n := range p.Nodes {
			ids = append(ids, n.ID)
			got, want := slices.Clone(waits[n.ID]), slices.Clone(n.Requires)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("%s: %s waits for %v by its shape, requires %v in its plan", src, n.ID, got, want)
			}
		}
		if !slices.Equal(steps, ids) {
			t.Errorf("%s: shape steps %v, plan nodes %v", src, steps, ids)
		}
		var single func(draft.Shape) bool
		single = func(s draft.Shape) bool {
			if s.Kind != "step" && len(s.Parts) < 2 {
				return true
			}
			return slices.ContainsFunc(s.Parts, single)
		}
		if single(shape) {
			t.Errorf("%s: a pipe or par of one part: %+v", src, shape)
		}
	}
	if _, err := draft.Composition(context.Background(), reg, `(nope)`); err == nil {
		t.Error("a program that doesn't compile has a shape")
	}
}

func ExampleComposition() {
	shape, _ := draft.Composition(context.Background(), draft.DefaultRegistry(), `(pipe (par (echo "a") (echo "b")) echo)`)
	b, _ := json.Marshal(shape)
	fmt.Println(string(b))
	// Output: {"kind":"pipe","parts":[{"kind":"par","parts":[{"kind":"step","node":"n0"},{"kind":"step","node":"n1"}]},{"kind":"step","node":"n2"}]}
}
