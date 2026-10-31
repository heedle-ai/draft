// Package sexpr implements the s-expression reader: text in, a generic
// Form tree out.
//
// This is deliberately the *only* thing this package does. It knows
// nothing about draft verbs, pipelines, backends, or the AST. The
// mapping from Form to ast lives in the parent script package
// (sexpr_ast.go), so adding a surface form later never touches the
// tokenizer, and the reader can be tested without pulling in the
// translator.
//
// Lexing and parsing are done by participle
// (github.com/alecthomas/participle), from the token rules and grammar
// below. This package only classifies atoms and decodes strings:
//
//	form    = list | string | atom
//	list    = "(" form* ")"
//	atom    = keyword | number | symbol   (decided after parsing)
//	keyword = ":" name
//	string  = '"' ( \n | \t | \r | \" | \\ | any other char but '"' or '\' )* '"'
//
// Comments run to end of line and start with ';' (lisp convention) or
// '//'. Both are accepted.
package sexpr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// === Forms =================================================================

// Kind discriminates the Form variants.
type Kind int

const (
	// KindInvalid is the zero value; never produced by a successful read.
	KindInvalid Kind = iota
	// KindList is a parenthesized sequence of forms.
	KindList
	// KindSymbol is a bare identifier: pipe, summarize, github/clone.
	KindSymbol
	// KindKeyword is a colon-prefixed option name: :backend. Text holds
	// the name without the colon.
	KindKeyword
	// KindString is a double-quoted literal. Text holds the unescaped value.
	KindString
	// KindNumber is a numeric literal. Num holds the value, Text the
	// source spelling.
	KindNumber
)

// String implements fmt.Stringer for error messages.
func (k Kind) String() string {
	switch k {
	case KindList:
		return "list"
	case KindSymbol:
		return "symbol"
	case KindKeyword:
		return "keyword"
	case KindString:
		return "string"
	case KindNumber:
		return "number"
	default:
		return "invalid"
	}
}

// Pos is a 1-based line and column in the source text.
type Pos struct {
	Line int
	Col  int
}

// String implements fmt.Stringer, rendering as "line:col".
func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Form is one node of the generic s-expression tree. Which fields are
// meaningful depends on Kind:
//
//	KindList             → Items
//	KindSymbol, Keyword  → Text
//	KindString           → Text (unescaped)
//	KindNumber           → Num, Text
type Form struct {
	Kind  Kind
	Pos   Pos
	Text  string
	Num   float64
	Items []Form
}

// Head returns the leading symbol of a list form. The second result is
// false for non-lists, empty lists, and lists whose first element is not
// a symbol.
func (f Form) Head() (string, bool) {
	if f.Kind != KindList || len(f.Items) == 0 {
		return "", false
	}
	if f.Items[0].Kind != KindSymbol {
		return "", false
	}
	return f.Items[0].Text, true
}

// IsSymbol reports whether f is the named symbol.
func (f Form) IsSymbol(name string) bool {
	return f.Kind == KindSymbol && f.Text == name
}

// === Errors ================================================================

// SyntaxError is a read failure with the position that caused it.
type SyntaxError struct {
	Pos Pos
	Msg string
}

// Error implements error.
func (e SyntaxError) Error() string {
	return fmt.Sprintf("sexpr: %s: %s", e.Pos, e.Msg)
}

// === Reader ================================================================

// The token rules. Order matters: a comment is tried before an atom, so
// "//" at the start of a token begins a comment, while "a//b" is one atom.
var sexprLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `;[^\n]*|//[^\n]*`},
	// Whitespace is what unicode.IsSpace accepts: ASCII spaces, \v, U+0085
	// and every Unicode space separator.
	{Name: "Whitespace", Pattern: `[\s\v\x{85}\p{Z}]+`},
	{Name: "String", Pattern: `"(\\[ntr"\\]|[^"\\])*"`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "Atom", Pattern: `[^\s\v\x{85}\p{Z}()";]+`},
})

// The grammar, as participle struct tags.
type file struct {
	Forms []*node `parser:"@@*"`
}

type node struct {
	Pos    lexer.Position
	List   *list   `parser:"  @@"`
	String *string `parser:"| @String"`
	Atom   *string `parser:"| @Atom"`
}

type list struct {
	Pos   lexer.Position
	Items []*node `parser:"\"(\" @@* \")\""`
}

var parser = participle.MustBuild[file](
	participle.Lexer(sexprLexer),
	participle.Elide("Comment", "Whitespace"),
)

// Read parses source text into a sequence of top-level forms. An empty
// (or comment-only) source reads as zero forms without error; rejecting
// that is the caller's business.
func Read(src string) ([]Form, error) {
	f, err := parser.ParseString("", src)
	if err != nil {
		return nil, syntaxError(err)
	}
	out := make([]Form, 0, len(f.Forms))
	for _, n := range f.Forms {
		form, err := convert(n)
		if err != nil {
			return nil, err
		}
		out = append(out, form)
	}
	return out, nil
}

func syntaxError(err error) error {
	var perr participle.Error
	if errors.As(err, &perr) {
		p := perr.Position()
		msg := perr.Message()
		// Two of participle's messages are reworded in the language's terms.
		switch {
		case strings.HasPrefix(msg, `invalid input text "\"`):
			msg = `unterminated string, or an escape other than \n \t \r \" \\`
		case strings.HasPrefix(msg, `unexpected token "<EOF>" (expected ")")`):
			msg = "unclosed '(' at end of input"
		}
		return SyntaxError{Pos: Pos{Line: p.Line, Col: p.Column}, Msg: msg}
	}
	return SyntaxError{Msg: err.Error()}
}

func convert(n *node) (Form, error) {
	pos := Pos{Line: n.Pos.Line, Col: n.Pos.Column}
	switch {
	case n.List != nil:
		list := Form{Kind: KindList, Pos: pos}
		for _, item := range n.List.Items {
			f, err := convert(item)
			if err != nil {
				return Form{}, err
			}
			list.Items = append(list.Items, f)
		}
		return list, nil
	case n.String != nil:
		// The lexer admits only escapes Go also defines, so Go's own
		// unquoting decodes them exactly. A string may span lines; Go's
		// syntax spells a line break as \n, so convert before unquoting.
		text, err := strconv.Unquote(strings.ReplaceAll(*n.String, "\n", `\n`))
		if err != nil {
			return Form{}, SyntaxError{Pos: pos, Msg: "invalid string " + *n.String}
		}
		return Form{Kind: KindString, Pos: pos, Text: text}, nil
	default:
		return atom(pos, *n.Atom)
	}
}

// atom classifies an atom as a keyword, number or symbol.
func atom(pos Pos, text string) (Form, error) {
	if strings.HasPrefix(text, ":") {
		if text == ":" {
			return Form{}, SyntaxError{Pos: pos, Msg: "keyword has no name"}
		}
		return Form{Kind: KindKeyword, Pos: pos, Text: text[1:]}, nil
	}
	if looksNumeric(text) {
		if n, err := strconv.ParseFloat(text, 64); err == nil {
			return Form{Kind: KindNumber, Pos: pos, Text: text, Num: n}, nil
		}
	}
	return Form{Kind: KindSymbol, Pos: pos, Text: text}, nil
}

// looksNumeric is a cheap guard so ParseFloat is only consulted for
// atoms that could plausibly be numbers. Without it, symbols like "inf"
// and "nan" would parse as floats.
func looksNumeric(s string) bool {
	c := s[0]
	if c == '-' || c == '+' || c == '.' {
		return len(s) > 1 && s[1] >= '0' && s[1] <= '9'
	}
	return c >= '0' && c <= '9'
}
