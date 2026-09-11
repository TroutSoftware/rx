package rxtest

import (
	"testing"

	"github.com/TroutSoftware/rx"
)

func TestMatchNodes(t *testing.T) {
	cases := []struct {
		node, target *rx.Node
	}{
		{rx.Get("<div>"), rx.Get("<div>")},
		{rx.Get(`<div data-testid="123">Find me</div>`), rx.Get(`<div data-testid="123">`)},
	}

	for _, c := range cases {
		if !match(c.node, c.target) {
			t.Errorf("no match %s, %s", c.node.ToHTML(), c.target.ToHTML())
		}
	}
}
