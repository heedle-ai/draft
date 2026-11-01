package draft_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/heedle-ai/draft"
)

// TestInv_WikiDiagramsMatchPlans: every ```lisp block in the wiki that is
// immediately followed by a ```mermaid block must compile, and the diagram
// must be exactly what draft.Mermaid draws for the compiled plan.
func TestInv_WikiDiagramsMatchPlans(t *testing.T) {
	fence := regexp.MustCompile("(?m)^```mermaid$") // a fenced block, not an inline mention
	block := regexp.MustCompile("```lisp\n([^`]*)```\\s*```mermaid\n([^`]*)```")
	pages, _ := filepath.Glob("docs/wiki/*.md")
	checked := 0
	for _, page := range pages {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		pairs := block.FindAllStringSubmatch(string(b), -1)
		if n := len(fence.FindAllIndex(b, -1)); n != len(pairs) {
			t.Errorf("%s: %d mermaid blocks but only %d follow a lisp program; every diagram must be generated from its program", page, n, len(pairs))
		}
		for _, m := range pairs {
			p, err := draft.Compile(context.Background(), draft.DefaultRegistry(), draft.Source(m[1]))
			if err != nil {
				t.Errorf("%s: program doesn't compile: %v\n%s", page, err, m[1])
				continue
			}
			if want := draft.Mermaid(p); m[2] != want {
				t.Errorf("%s: diagram disagrees with the compiled plan\nprogram:\n%s\nhave:\n%s\nwant:\n%s", page, m[1], m[2], want)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no lisp+mermaid pairs found in docs/wiki; the check would be vacuous")
	}
	t.Logf("%d wiki diagrams match their compiled plans", checked)
}

// TestInv_ParserIsParticiple: tokenizing and parsing are participle's
// (reuse rule); no other package in draft imports a parser library.
func TestInv_ParserIsParticiple(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{.ImportPath}}: {{join .Imports \" \"}}", "./...").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg, imports, _ := strings.Cut(line, ": ")
		usesParticiple := strings.Contains(imports, "github.com/alecthomas/participle/v2")
		switch {
		case pkg == "github.com/heedle-ai/draft/sexpr" && !usesParticiple:
			t.Errorf("sexpr no longer parses with participle")
		case pkg != "github.com/heedle-ai/draft/sexpr" && (usesParticiple || strings.Contains(imports, "text/scanner")):
			t.Errorf("%s does its own parsing; only sexpr should", pkg)
		}
	}
}

// FuzzParsePrint: printing a parsed program and parsing it again gives an
// equal AST (PrintSExpr's documented contract), and Compile never panics.
func FuzzParsePrint(f *testing.F) {
	for _, seed := range []string{
		`(pipe (par (echo "a") (echo "b")) echo)`,
		"(pipe\v(echo \"two\nlines\")  echo\u0085)", // differential test: Unicode space, multiline string
		`(echo "say \"hi\"\n\tbye")`,
		`(block :backend memory :mode dynamic (echo 1.5))`,
		`(echo "a" "b")`, `(pipe (echo "a"`, `summarize)`, `(block : temporal (echo "a"))`,
		"; comment\n(echo \"a\") // trailing",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		ctx := context.Background()
		draft.Compile(ctx, draft.DefaultRegistry(), draft.Source(src)) // must not panic
		a, err := draft.Parse(ctx, draft.Source(src))
		if err != nil {
			return
		}
		printed, err := draft.PrintSExpr(a)
		if err != nil {
			return
		}
		b, err := draft.Parse(ctx, draft.Source(printed))
		if err != nil {
			t.Fatalf("printed form doesn't parse: %v\nsource: %q\nprinted: %q", err, src, printed)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("round trip changed the AST\nsource: %q\nprinted: %q", src, printed)
		}
	})
}
