package rx

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type Node struct {
	Entity   // simple reference
	TagName  string
	Classes  string
	Text     string
	Focused  bool
	Children []*Node
	Attrs    []Attr // for arbitrary HTML elements

	visited bool

	old Entity // for reuse nodes
	hdl intentHandler
}

func (n *Node) SetText(text string) *Node { n.Text = text; return n }

func (n *Node) AddChildren(cs ...*Node) *Node { n.Children = append(n.Children, cs...); return n }

// Deprecated: use [Keep] instead
func (n *Node) GiveKey(ctx Context) *Node { n.Entity = ctx.ng.cnt.Inc(); return n }

// AddAttr adds or replace the arguments in the node.
// Arguments must be given by alternating keys and values.
func (n *Node) AddAttr(kv ...string) *Node {
	for i := 0; i < len(kv); i += 2 {
		idx := slices.IndexFunc(n.Attrs, func(a Attr) bool { return a.Name == kv[i] })
		if idx == -1 {
			n.Attrs = append(n.Attrs, Attr{Name: kv[i], Value: kv[i+1]})
		} else {
			n.Attrs[idx].Value = kv[i+1]
		}
	}
	return n
}

// GetAttr returns the value set for the attribute.
// An empty string is returned if no value is set.
func (n *Node) GetAttr(attr string) string {
	for _, a := range n.Attrs {
		if a.Name == attr {
			return a.Value
		}
	}
	return ""
}

// OnIntent attaches the action to the intent.
//
// When the intent is fired on the node (browser-side),
// the action is executed on the current context, leading to a new context.
// The new view is then rendered based on the new context.
func (n *Node) OnIntent(evt IntentType, h Action) *Node {
	if h == nil {
		return n
	}

	n.hdl[evt] = h
	return n
}

// React is a executes state mutators on an event
func (n *Node) React(evt IntentType, mutators ...any) *Node {
	return n.OnIntent(evt, Mutate(mutators...))
}

// Focus calls the [focus] method on the final element
//
// [focus]: https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/focus
func (n *Node) Focus(ctx Context) *Node { n.Focused = true; return n.GiveKey(ctx) }

// Set ARIA role, using the "role" property
// Useful for reliable tests
func (n *Node) AddRole(role string) *Node {
	// it's "role", not "aria-role"
	return n.AddAttr("role", role)
}

// AddClasses to a node.
// If the classes already exists, they are not modified.
// Empty classes are simply ignored.
//
// Example:
//
//	GetNode("td").SetClass("table-cell"); td().AddClass("bg-blue")
func (n *Node) AddClasses(cls ...string) *Node {

	c := strings.Join(cls, " ")
	if n.Classes == "" {
		n.Classes = c
	} else {
		n.Classes = n.Classes + " " + c
	}
	return n
}

// Can be used for attributes such as checkbox "checked", button "disabled"
// Example:
// GetNode("button").AddBoolAttr("disabled", isDisabled)
func (n *Node) AddBoolAttr(key string, val bool) *Node {
	if val {
		n.Attrs = append(n.Attrs, Attr{Name: key, Value: ""})
	}
	return n
}

func (n *Node) IsNothing() bool {
	return n.TagName == "nothing"
}

// ElementID returns the Element.id property.
// This can be used in referencing, e.g. "for" properties.
func (n *Node) ElementID(ctx Context) string {
	if n.Entity == 0 {
		n.GiveKey(ctx)
	}
	return strconv.FormatUint(uint64(n.Entity), 10)
}

// ActionFor returns the action registered in n for event of type t.
// This is mostly useful for tests.
func ActionFor(n *Node, t IntentType) Action { return n.hdl[t] }

// Visit is an internal function used to ensure there are no cycle during rendering.
func (n *Node) Visit() {
	if n.visited {
		panic("cycle detected")
	}
	n.visited = true
}

// Nothing returns a node that does not appear in the DOM.
// This is useful in conditionals, making branches regular, e.g.:
//
//	  x := Nothing()
//	  if val > threshold {
//			x = alert()
//	  }
//
// During the rendering phase, Nothing is optimized away; which means that:
//
//  1. Terminal nodes will simply not exist
//  2. Children of Nothing nodes will become children of the parent of the Nothing node.
func Nothing(ws ...*Node) *Node { return getNode("nothing").AddChildren(ws...) }

type Attr struct{ Name, Value string }

type poolNode struct {
	next  *poolNode
	nodes []Node
}

var npool = struct {
	poolNode
	free *poolNode
	nmtx sync.Mutex
}{poolNode: poolNode{nodes: make([]Node, 0, 512)}}

// getNode returns a node from the pool, minimizing allocations.
// The pool is re-initialized as a whole during each cycle.
func getNode(tagname string) *Node {
	npool.nmtx.Lock()
	defer npool.nmtx.Unlock()

	// invariant: pool next is nil iff len(nodes) < cap(nodes)

	pool := &npool.poolNode
	for pool.next != nil {
		pool = pool.next
	}

	pool.nodes = pool.nodes[:len(pool.nodes)+1]
	if len(pool.nodes) == cap(pool.nodes) {
		if npool.free != nil {
			pool.next = &poolNode{nodes: npool.free.nodes[:0]}
			npool.free = npool.free.next
		} else {
			pool.next = &poolNode{nodes: make([]Node, 0, 512)}
		}
	}

	last := &pool.nodes[len(pool.nodes)-1]
	// reset all fields, preserve space already alloc for values
	*last = Node{TagName: tagname, Attrs: last.Attrs[:0], Children: last.Children[:0]}
	return last
}

// freePool de-allocate all nodes at once.
func freePool() {
	npool.nmtx.Lock()
	defer npool.nmtx.Unlock()

	npool.free, npool.next = npool.next, nil
	clear(npool.nodes)
	npool.nodes = npool.nodes[:0]
}

// serialize does a preorder visit of the node tree, keeping track of nodes in the entity tree
func serialize(n *Node, tree *etree, ctr *Counter, vm XAS) XAS {
	if n.visited {
		panic("cycle detected")
	}
	n.visited = true

	switch n.TagName {
	case "":
		panic("empty tag name")

	case "nothing":
		for _, c := range n.Children {
			assert(c != nil, "nil child in node: %v", n)
			vm = serialize(c, tree, ctr, vm)
		}
		return vm

	case "reuse":
		// Reuse ports the old tree to the new one
		// ReID is then updating the ID, so that the handlers fire on the correct element
		vm = vm.AddInstr(OpReuse, strconv.FormatUint(uint64(n.old), 10))
		tree.reuse(n.old, n.Entity, ctr, func(from, to Entity) {
			vm = vm.AddInstr(OpReID,
				strconv.FormatUint(uint64(from), 10),
				strconv.FormatUint(uint64(to), 10))

		})

		return vm
	}
	vm = vm.AddInstr(OpCreateElement, n.TagName)

	if len(n.Classes) > 0 {
		vm = vm.AddInstr(OpSetClass, n.Classes)
	}

	if n.Entity == 0 && n.hdl.Some() {
		// curtesy, create the entity for user
		n.Entity = ctr.Inc()
	}

	var idx int
	if n.Entity != 0 {
		idx = tree.add(n.Entity)
		if n.hdl.Some() {
			tree.addHandler(n.hdl)
		}
		vm = vm.AddInstr(OpSetID, strconv.FormatUint(uint64(n.Entity), 10))
	}

	for _, a := range n.Attrs {
		vm = vm.AddInstr(OpSetAttr, a.Name, a.Value)
	}
	if n.Text != "" {
		vm = vm.AddInstr(OpAddText, n.Text)
	}

	for _, c := range n.Children {
		vm = serialize(c, tree, ctr, vm)
	}
	if n.Entity != 0 {
		tree.closeScope(idx)
	}

	return vm.AddInstr(OpNext)
}

// Build bottoms-out the rendering tree: a node is a widget that is self
func (n *Node) Build(_ Context) *Node { return n }

// ToHTML creates a textual representation of the node tree.
// This is useful for server-side rendering.
// As such, there is no way to attach a callback to an entity.
func (n *Node) ToHTML() string {
	var buf strings.Builder
	serializeHTML(n, &buf)
	return buf.String()
}

func serializeHTML(n *Node, buf *strings.Builder) {
	// skip nothing node
	if n.IsNothing() {
		for _, c := range n.Children {
			assert(c != nil, "nil child in node: %v", n)
			serializeHTML(c, buf)
		}
		return
	}

	fmt.Fprintf(buf, "<%s ", n.TagName)
	if len(n.Classes) > 0 {
		fmt.Fprintf(buf, "class=\"%s\" ", n.Classes)
	}

	for _, a := range n.Attrs {
		fmt.Fprintf(buf, "%s=\"%s\"", a.Name, a.Value)
	}
	fmt.Fprint(buf, ">")

	if n.Text != "" {
		fmt.Fprint(buf, n.Text)
	}

	for _, c := range n.Children {
		serializeHTML(c, buf)
	}
	fmt.Fprintf(buf, "</%s>", n.TagName)
}

// using an alias let's us run go generate but do not alter existing code
//
//go:generate go tool rxabi -type OpType
type OpType = byte

const (
	OpTerm OpType = iota
	OpCreateElement
	OpSetClass
	OpSetID
	OpSetAttr
	OpAddText
	OpReuse
	OpReID
	OpNext
)

type XAS []byte

func (vm XAS) AddInstr(code byte, val ...string) XAS {
	vm = append(vm, code)
	for i := range val {
		vm = binary.BigEndian.AppendUint16(vm, uint16(len(val[i])))
		vm = append(vm, val[i]...)
	}
	return vm
}
