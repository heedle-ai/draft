## 1. Install Go 1.24+

**macOS (Apple Silicon)**

```bash
brew install go
```

**Linux (amd64 or arm64)**

```bash
GO_VERSION=1.24.7
ARCH=$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
curl -fsSLO "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-${ARCH}.tar.gz"
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile && source ~/.profile
```

## 2. Clone the repos as siblings

draft builds against local checkouts of weft and shuttle until those are published (`go.mod` replaces them with `../weft` and `../shuttle`):

```bash
mkdir -p ~/src/heedle-ai && cd ~/src/heedle-ai
git clone https://github.com/heedle-ai/weft.git
git clone https://github.com/heedle-ai/shuttle.git
git clone https://github.com/heedle-ai/draft.git
cd draft
go test -race ./...
```

No Temporal server is needed for the tests. They run compiled plans through shuttle's real workflow in Temporal's in-process test suite.

## 3. Compile a program

```go
src := draft.Source(`(pipe (par (echo "a") (echo "b")) echo)`)

p, err := draft.Compile(context.Background(), draft.DefaultRegistry(), src)
if err != nil {
	fmt.Println(err)
	return
}
for _, n := range p.Nodes {
	fmt.Println(n.ID, n.Activity, n.Args, n.Requires)
}
```

```text
n0 Echo [a] []
n1 Echo [b] []
n2 Echo [] [n0 n1]
```

This is `ExampleCompile` in `example_test.go`, so `go test` checks it.

## 4. Run it on Temporal

A compiled plan is an ordinary `plan.Plan`. Start a worker that registers shuttle's workflow (`plan.Register(w)`), then submit:

```go
run, err := draft.Submit(ctx, c, p, client.StartWorkflowOptions{TaskQueue: "shuttle-echo"})

var res plan.Result
err = run.Get(ctx, &res) // res.Outputs["n2"] == "a b"
```

For a complete runnable worker and submitter, see [shuttle's Getting Started](https://github.com/heedle-ai/shuttle/wiki/Getting-Started). Swap its `demoPlan()` for `p`.

`n2` receives `"a b"` because `echo` with no arguments joins its upstream outputs in node-ID order. `TestPar_FanOutThenFanIn` in `run_test.go` checks this.

## Errors you'll see

Compile fails before anything runs:

| Program | Error |
|---|---|
| `(pipe (echo "a") shout)` | `block 0: stage 1: unknown builtin "shout" (known: echo)` |
| `(echo "a" "b")` | `block 0: stage 0: builtin "echo": got 2 args, want 0-1` |
| `(echo 42)` | `block 0: stage 0: builtin "echo": arg 0 is number, want string` |
| `(pipe (echo "a"` | `draft.Parse: sexpr: 1:7: unclosed '('` |

Next: [Language](Language).
