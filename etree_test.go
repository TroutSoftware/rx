package rx

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"text/scanner"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestWalkETree(t *testing.T) {
	type gratitude struct{ callchain, subtree map[Entity][]Entity }
	g := func(ets ...Entity) gratitude {
		parents := make(map[uint32][]uint32)
		children := make(map[uint32][]uint32)
		for i := 0; i < len(ets); i += 2 {
			children[ets[i]] = append(children[ets[i]], ets[i+1])
			parents[ets[i+1]] = append(parents[ets[i+1]], ets[i])
		}
		return gratitude{parents, children}
	}

	cases := []struct {
		tree string
		want gratitude
	}{
		{"(1)", g()},
		{"(1(2))", g(1, 2)},
		{"(1(2, 3(4), 5(7)))", g(
			1, 2,
			1, 3,
			1, 4,
			1, 5,
			1, 7,
			3, 4,
			5, 7,
		)},
	}

	for _, c := range cases {
		var et etree
		loadtree(readNodes(c.tree), &et)

		for n, cs := range c.want.subtree {
			cs = append(cs, n) // entity is part of sub-tree
			gs := pntt(et.children(n))
			if !cmp.Equal(cs, gs, cmpopts.SortSlices(func(i, j uint32) bool { return i < j })) {
				t.Errorf("Children of %d: diff %s", n, cmp.Diff(cs, gs, cmpopts.SortSlices(func(i, j uint32) bool { return i < j })))
			}
		}

		t.Log("g1, g0", et.g1, et.g0)
		for n, ps := range c.want.callchain {
			ps = append(ps, n) // entity is part of parent chain
			gs := pntt(et.parents(n))
			if !cmp.Equal(ps, gs, cmpopts.SortSlices(func(i, j uint32) bool { return i < j })) {
				t.Errorf("Parents of %d: diff %s", n, cmp.Diff(ps, gs, cmpopts.SortSlices(func(i, j uint32) bool { return i < j })))
			}
		}
	}
}

// the client could be asking for parents / children of missing nodes
// make sure the error message make sense
func TestMissingEntries(t *testing.T) {
	t.Run("parents", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("no recovery? see line 88-89")
			}
			if !strings.Contains(fmt.Sprint(r), "does not exist in the element tree") {
				t.Errorf("bad panic message: %s", r)
			}
		}()

		var et etree
		loadtree(readNodes("(2(4, 6))"), &et)
		t.Log(et.parents(3))
		t.Fatal("finding an missing parent should panic")
	})
}

func loadtree(n *Node, et *etree) {
	var rec func(n *Node)
	rec = func(n *Node) {
		idx := et.add(n.Entity)
		for i := range n.Children {
			rec(n.Children[i])
		}
		et.closeScope(idx)
	}
	rec(n)
	et.ngen()
}

func pntt(in []prenode) []Entity {
	out := make([]Entity, len(in))
	for i, v := range in {
		out[i] = v.ntt
	}
	return out
}

// string representation of a node tree
// it is fairly verbose to use Node when working on the inners of the tree structure,
// so we rely instead of a more concise string encoding, e.g. :
//
//	(1(2, 3(4))), matching the tree 1
//	                                    2 3
//	                                      4
//
// this representation can trivially be converted to nodes, e.g.:
//
// (1(2, 3(4))) -> {Entity: 1, Children: {Entity: 2}, {Entity:3, Children: {Entity: 4}}}
//
// (forthcoming) unknown entities will be represented by “?”
func readNodes(short string) *Node {
	sc := scanner.Scanner{
		Error: func(s *scanner.Scanner, msg string) {
			panic(fmt.Sprintf("scanning at %v: %s", s.Position, msg))
		},
		Mode: scanner.ScanInts,
	}
	sc.Init(strings.NewReader(short))
	var stack []*Node
	pop := func() *Node {
		if len(stack) == 0 {
			return nil
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return top
	}
	push := func(n *Node) { stack = append(stack, n) }

	for sc.Peek() != scanner.EOF {
		switch sc.Scan() {
		case '(':
			push(new(Node))
		case ')':
			n := pop()
			if par := pop(); par != nil {
				par.Children = append(par.Children, n)
				push(par)
			}
		case ',':
			sib := pop()
			par := pop()
			par.Children = append(par.Children, sib)
			push(par)
			push(new(Node))
		case scanner.Int:
			nv, err := strconv.ParseInt(sc.TokenText(), 10, 16)
			if err != nil {
				panic(err)
			}
			n := pop()
			n.Entity = Entity(nv)
			push(n)
		}
	}

	return stack[:1][0]
}
