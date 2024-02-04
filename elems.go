package rx

import (
	"errors"
	"io"
	"sync"
)

var cachedParseResults = make(map[string]qVM)
var cachedParseResultsMx sync.Mutex

// Get parses the Get text in tpl, and returns a node matching it.
// This is a short hand for the [GetNode], [Node.AddAttr], … constructors, optimized for easy definition.
// The node returned can be further modified.
//
// When used in the short form (defining a single element), the element need not be closed:
//
//	Get(`<div class="flex">`)
//
// Get can also create a full tree:
//
//	Get(`<div class="flex"><button>Click me</button></div>`)
//
// Get panics if the template is not valid, and should not be used for untrusted inputs.
// Get uses a caching mechanism to prevent unecessary parsing, so it works best with static strings.
func Get(tpl string) *Node {
	cachedParseResultsMx.Lock()
	defer cachedParseResultsMx.Unlock()

	if ctor := cachedParseResults[tpl]; ctor != nil {
		return ctor.run(tpl)
	}

	var vm qVM
	z := newTokenizer(tpl)

	for {
		tt := z.Next()
		switch tt {
		case errorToken:
			if errors.Is(z.Err(), io.EOF) {
				cachedParseResults[tpl] = vm
				return vm.run(tpl)
			}
			panic(z.Err())
		case textToken:
			val := z.Text()
			vm = append(vm, qVMOp{Op: qText, R1: val.Start, R2: val.End})
		case startTagToken:
			tn, hasattr := z.TagName()
			vm = append(vm, qVMOp{Op: qNode, R1: tn.Start, R2: tn.End})
			for hasattr {
				var key, val span
				key, val, hasattr = z.TagAttr()
				if tpl[key.Start:key.End] == "class" {
					vm = append(vm, qVMOp{Op: qClasses, R1: val.Start, R2: val.End})
				} else {
					vm = append(vm, qVMOp{Op: qAttrs, R1: key.Start, R2: key.End, R3: val.Start, R4: val.End})
				}
			}
		case selfClosingTagToken:
			tn, hasattr := z.TagName()
			vm = append(vm, qVMOp{Op: qNode, R1: tn.Start, R2: tn.End})
			for hasattr {
				var key, val span
				key, val, hasattr = z.TagAttr()
				if tpl[key.Start:key.End] == "class" {
					vm = append(vm, qVMOp{Op: qClasses, R1: val.Start, R2: val.End})
				} else {
					vm = append(vm, qVMOp{Op: qAttrs, R1: key.Start, R2: key.End, R3: val.Start, R4: val.End})
				}
			}
			vm = append(vm, qVMOp{Op: qTerm})

		case endTagToken:
			vm = append(vm, qVMOp{Op: qTerm})
		}
	}
}

type qVMOp struct {
	Op     byte
	R1, R2 int // op-dependent
	R3, R4 int // op-dependent
}

const (
	qTerm byte = iota
	qNode
	qAttrs
	qClasses
	qText
)

type qVM []qVMOp

func (vm qVM) run(tpl string) *Node {
	p, stack := new(Node), make([]*Node, 0, 16)
	for _, op := range vm {
		switch op.Op {
		case qTerm:
			p, stack = pop(stack)
		case qNode:
			if p.TagName == "" {
				p.TagName = tpl[op.R1:op.R2]
				stack = append(stack, p) // store root twice so pop in qTerm always a good op
				continue
			}

			c := GetNode(tpl[op.R1:op.R2])
			p.AddChildren(c)
			p, stack = c, append(stack, p)
		case qAttrs:
			p.AddAttr(tpl[op.R1:op.R2], tpl[op.R3:op.R4])
		case qClasses:
			p.Classes = tpl[op.R1:op.R2]
		case qText:
			p.Text = tpl[op.R1:op.R2]
		}
	}

	if p.TagName == "" {
		panic("invalid tag")
	}
	return p
}

func pop(stack []*Node) (*Node, []*Node) {
	return stack[len(stack)-1], stack[:len(stack)-1]
}
