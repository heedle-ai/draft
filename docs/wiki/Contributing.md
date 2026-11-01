## Ground rules

- **Grammar changes are breaking changes.** Update [Language](Language) and the parser tests in the same change.
- **Compile never runs anything.** Only `Submit` touches Temporal.
- **Every documented program is tested.** Compile output goes in `example_test.go`; end-to-end behaviour goes in `run_test.go` or `command_test.go`, using shuttle's real workflow in Temporal's test suite.
- **Update this wiki in the same change.** The pages live in `docs/wiki/` in this repo, and every push to `main` publishes them here. Don't edit pages on GitHub directly; the next sync overwrites them.
- Supported platforms are Linux (amd64, arm64) and macOS on Apple Silicon.

- **A mistake isn't fixed until it has a row on [Invariants](Invariants)** and a guard test that fails on the old code. CI checks the page against the tests in both directions.
- **Program diagrams are generated, never drawn.** Put a ```` ```mermaid ```` block straight after a ```` ```lisp ```` block; `TestInv_WikiDiagramsMatchPlans` fails until it matches `draft.Mermaid` of the compiled program.

## Before opening a pull request

```bash
gofmt -l .        # prints nothing
go vet ./...
go test -race ./...
bash .github/invariants.sh
go test -run '^$' -fuzz FuzzParsePrint -fuzztime 30s .
```
