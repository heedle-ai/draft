# draft

S-expression language that compiles to shuttle plans. Part of heedle-ai:
weft (algebra) → draft (language) → shuttle (execution) → warp (object store) → drawdown (UI);
loom runs them together.

## The first rule: reuse, never rewrite

Build on battle-tested tools (Git, Temporal, the Go standard library and
other long-proven software) through their public interfaces. Add on top of
them; never fork, patch, mutate or reimplement them. Prefer the real tool to
a reimplementation of it (for example the `git` CLI over a Go port of Git).
If a task seems to need a rewrite, stop and ask the owner first, explaining
why the existing tool can't do it.

## Rules for every change

1. **Keep the wiki current.** Any change that affects how a developer
   installs, builds, tests, uses or understands this repo updates
   `docs/wiki/` in the same commit or PR. That folder is the source of truth;
   `.github/workflows/wiki.yml` mirrors it to
   https://github.com/heedle-ai/draft/wiki on every push to `main`. Never edit
   the wiki on GitHub directly: the next sync overwrites it. Code, README and
   wiki must never disagree.
2. **Wiki quality bar.** Every command is copy-pasteable and has been run.
   Every code snippet compiles and is checked by a test; prefer quoting a
   Go `Example` test with an `// Output:` comment. State required versions.
   No placeholders, TODOs or "coming soon" on published pages. Mark design
   that isn't built yet as **Planned**, plainly.
3. **Platforms: Linux (amd64, arm64) and macOS on Apple Silicon only.** Don't
   write Windows instructions, scripts, CI jobs or path handling.
4. **Bare minimum.** Add only what the task needs. No speculative packages,
   dependencies or abstractions.
5. **Tests pass before commit:** `go vet ./...` and `go test -race ./...`.

## Repo-specific

- **Parsing is participle's.** The lexer rules and grammar live in
  `sexpr/sexpr.go`; don't hand-write tokenizing or parsing.
- **Grammar changes are breaking changes.** Update the Language wiki page and
  the parser tests together.
- **Compile never runs anything.** Parse, Resolve, Lower, Finalize and
  Validate are pure; only Submit talks to Temporal.
- **Every documented program has a test**: `example_test.go` for compile
  output, `run_test.go` and `command_test.go` for end-to-end runs through
  shuttle's real workflow.
- `go.mod` replaces weft and shuttle with `../weft` and `../shuttle` until
  they are published; clone the repos as siblings.
