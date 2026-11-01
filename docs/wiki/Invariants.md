Every invariant below is guarded by a test or a CI step, and CI checks this page against the code on every push: each test named here must exist, and every `TestLaw_*`, `TestInv_*` and `Fuzz*` test must be listed here. A mistake isn't fixed until it has a row on this page.

[[/images/invariant-loop.svg|alt=From a mistake to a codified invariant: mistake, named invariant, guard, CI gate]]

| ID | Invariant | Guarded by | Came from |
|---|---|---|---|
| DRAFT-1 | `pipe` compiles to sequential composition and `par` to a labelled product: fan-out, fan-in and nesting give exactly the expected graph, and run correctly through shuttle | `TestPar_FanOutThenFanIn`, `TestPar_BranchesShareUpstream`, `TestPar_NestedPipelines` | `par` parsed but couldn't compile ("not yet supported") |
| DRAFT-2 | Printing a parsed program and parsing it again gives an equal AST; no input makes `Compile` panic | `FuzzParsePrint`, `ci:fuzz` | `PrintSExpr` documented this but nothing checked it |
| DRAFT-3 | Tokenizing and parsing are participle's; no other package parses | `TestInv_ParserIsParticiple` | A hand-written reader broke the reuse rule |
| DRAFT-4 | Whitespace is anything Unicode treats as a space; strings may span lines | `TestReadUnicodeWhitespaceAndMultilineStrings` | Differential test between the old and new readers |
| DRAFT-5 | Syntax errors carry an exact line and column, in plain words | `TestReadErrorMessages`, `TestReadPositions` | participle's raw messages were unclear |
| DRAFT-6 | Every program diagram on this wiki is exactly what its program compiles to | `TestInv_WikiDiagramsMatchPlans` | Docs must never disagree with the code |
| DRAFT-7 | Documented compile output is exact | `ExampleCompile`, `ExampleCompile_unknownCommand` | Docs must never claim untested behaviour |
| DRAFT-8 | Only `:backend temporal :mode static` compiles; the old interpreter's modes are refused | `TestMemoryBackendIsRejected` | The in-memory interpreter was left behind |
| DRAFT-9 | A command must declare the temporal backend, or compiling fails | `TestCustomCommand`, `TestCustomCommand_NeedsTemporalBackend` | Easy to miss when adding a command |
