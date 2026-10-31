package draft_test

import (
	"context"
	"fmt"

	"github.com/heedle-ai/draft"
)

func ExampleCompile() {
	src := draft.Source(`(pipe (par (echo "a") (echo "b")) echo)`)

	p, err := draft.Compile(context.Background(), draft.DefaultRegistry(), src)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, n := range p.Nodes {
		fmt.Println(n.ID, n.Activity, n.Args, n.Requires)
	}
	// Output:
	// n0 Echo [a] []
	// n1 Echo [b] []
	// n2 Echo [] [n0 n1]
}

func ExampleCompile_unknownCommand() {
	_, err := draft.Compile(context.Background(), draft.DefaultRegistry(), draft.Source(`(pipe (echo "a") shout)`))
	fmt.Println(err != nil)
	// Output: true
}
