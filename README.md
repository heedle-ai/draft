# draft

A small s-expression language that compiles to [shuttle](https://github.com/heedle-ai/shuttle) plans. Part of [heedle-ai](https://github.com/heedle-ai).

```lisp
(pipe
  (par (echo "a") (echo "b"))   ; two branches, run concurrently
  echo)                         ; waits for both, receives "a b"
```

`pipe` is sequential composition and `par` is the parallel product. Each command names a registered shuttle activity. Compiling checks names, arguments and graph shape before anything runs.

```go
p, err := draft.Compile(ctx, draft.DefaultRegistry(), draft.Source(src))
run, err := draft.Submit(ctx, temporalClient, p, client.StartWorkflowOptions{TaskQueue: "q"})
```

Documentation: **[wiki](https://github.com/heedle-ai/draft/wiki)**

```bash
go test -race ./...   # clone weft and shuttle next to this repo first
```

## Provenance

Copied from [`vinodhalaharvi/agentscript`](https://github.com/vinodhalaharvi/agentscript) at `0e426f7`: the `pkg/script` translator only, retargeted from Sibyl to shuttle. Left behind: the 100+ command plugins, LLM backends, the prose-to-DSL translation step, the grammar-discovery step, the full historical verb catalog, the in-memory interpreter and the CLI. Changed: `temporal` is the default backend; `par` now compiles (it previously stopped with "not yet supported"); and the hand-written s-expression reader was replaced by [participle](https://github.com/alecthomas/participle), with no change in behaviour.
