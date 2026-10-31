// Package draft compiles s-expression programs into shuttle plans.
//
//	(pipe (par (echo "a") (echo "b")) echo)
//
// pipe runs stages in sequence; par runs branches concurrently on the same
// input. Each command names a registered shuttle activity.
//
// The compiler is a pipeline of weft.Arrow values:
//
//	Source >>> Parse >>> Resolve >>> Lower >>> Finalize >>> Validate
//
// Compile runs it and returns a validated plan.Plan; Submit starts that
// plan on Temporal through shuttle.
package draft

// Source is a unit of draft text submitted to the translator. It
// is a thin alias around string so the type lineage in the memo
// (Source >>> AST >>> ...) lines up with actual Go types.
type Source string

// String implements fmt.Stringer for ergonomic logging.
func (s Source) String() string { return string(s) }
