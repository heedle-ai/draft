A command has three parts: what it **does** (a weft arrow), the name the **language** uses for it (a registry entry), and the name the **worker** runs it under (a Temporal activity). The registry entry's `AgentID` must equal the activity name.

```go
// 1. What it does: any Arrow[plan.Input, string].
shout := weft.Pure(func(in plan.Input) string {
	var parts []string
	for _, v := range in.Upstream {
		parts = append(parts, v)
	}
	return strings.ToUpper(strings.Join(parts, " "))
})

// 2. The language name, mapped to an activity name.
reg := draft.DefaultRegistry()
reg.MustRegister(registry.BuiltinSpec{
	Name:     "shout",
	AgentID:  "Shout",
	Backends: []registry.Backend{registry.BackendTemporal},
})

// 3. Compile against that registry.
p, err := draft.Compile(ctx, reg, draft.Source(`(pipe (echo "hello") shout)`))

// 4. On the worker, register the arrow under the same activity name.
plan.Register(w)
w.RegisterActivityWithOptions(shuttle.ArrowAsActivity(shout),
	activity.RegisterOptions{Name: "Shout"})
```

Running `p` gives `n1 = "HELLO"`. This code is `TestCustomCommand` in `command_test.go`.

## The fields that matter

| Field | Purpose |
|---|---|
| `Name` | What programs write, for example `shout` |
| `AgentID` | The Temporal activity name the node dispatches to |
| `Backends` | **Must include `registry.BackendTemporal`.** Without it the command is treated as memory-only, and compiling fails with `builtin "shout" is not implemented on the temporal backend yet` |
| `ArgSchema` | Optional. Declares parameters (`registry.StringT`, `Optional`, …) so bad calls fail at compile time. See `EchoSpec` in `builtins.go` |
| `Bound` | Optional. Arguments every call gets first, before the program's own, written into each plan node: for example a skill's name and the version the plan pins. The program can't change them; `ArgSchema` describes only the program's arguments |
| `Options` | Optional. How the command's steps run by default (`plan.Options`: `Timeout`, `Heartbeat`, `MaxAttempts`, …), written into each of its plan nodes so the plan records them. Use it for a step that works for minutes, like pirn's `agent` |

## Rules

Commands are shuttle activities, so shuttle's rules apply: return small strings, be idempotent, and use typed non-retryable errors for bad input. See [shuttle → Writing Activities](https://github.com/heedle-ai/shuttle/wiki/Writing-Activities).

Compiling and running use different processes in general. The program that compiles needs the registry entry; the worker needs the activity. Keep both lists in sync, usually in one package that does both.
