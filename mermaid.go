package draft

import (
	"strings"

	"github.com/heedle-ai/shuttle/plan"
)

// Mermaid renders a plan as a Mermaid flowchart, for docs: one box per
// node ("id: Activity "arg" …") and one edge per dependency, in plan order.
// GitHub renders it natively in Markdown. The wiki's program diagrams are
// checked against this output by TestInv_WikiDiagramsMatchPlans, so a
// diagram can't disagree with what its program compiles to.
func Mermaid(p plan.Plan) string {
	var b strings.Builder
	b.WriteString("graph LR\n")
	for _, n := range p.Nodes {
		label := string(n.ID) + ": " + n.Activity
		for _, a := range n.Args {
			label += " " + `"` + a + `"`
		}
		b.WriteString("  " + string(n.ID) + `["` + mermaidEscape(label) + `"]` + "\n")
	}
	for _, n := range p.Nodes {
		for _, r := range n.Requires {
			b.WriteString("  " + string(r) + " --> " + string(n.ID) + "\n")
		}
	}
	return b.String()
}

// mermaidEscape uses Mermaid's entity codes for characters that would end
// a label or be read as markup.
func mermaidEscape(s string) string {
	return strings.NewReplacer("#", "#35;", `"`, "#quot;", "<", "#lt;", ">", "#gt;").Replace(s)
}
