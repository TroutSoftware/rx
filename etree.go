package rx

import "fmt"

// prenode is a node in a preorder sequential representation of a binary tree. See Knuth, 2.3.3.
type prenode struct {
	ntt Entity
	scp int
	hdl intentHandler
}

type intentHandler [Seppuku]func(Context) Context

func (i *intentHandler) Some() bool {
	for _, h := range i {
		if h != nil {
			return true
		}
	}
	return false
}

// etree is a bi-generational tree structure.
// append are done on g0, and read on g1.
// it is used in the engine, where each turn of the crank results in a new gen
type etree struct {
	g0, g1 []prenode
	sparse [10240]int
}

// ngen starts recording a new generation of entities
// [etree.parents] should not be called after this
func (t *etree) ngen() {
	t.g1, t.g0 = t.g0, t.g1[:0]
	for i, v := range t.g1 {
		t.sparse[v.ntt] = i
	}
}

// add adds an entity to the current tree.
// by default, the entity is open on the left: you must call [etree.closeLeft] if the entity does not have children
func (t *etree) add(nt Entity) int {
	assert(nt < 10240, "cannot store more than 10240 entities in event handler")

	t.g0 = append(t.g0, prenode{ntt: nt})
	return len(t.g0) - 1
}

// reuse carries from the previous generation a sub-tree
// it performs entity renaming, and calls the it function on each rename
func (t *etree) reuse(from, to Entity, c *Counter, it func(from, to Entity)) {
	start := len(t.g0)
	// help with buggy client code
	assert(start != -1, "reusing invalid entity %d", from)

	t.g0 = append(t.g0, t.children(from)...)

	for i := range t.g0[start:] {
		var nt Entity
		if i == 0 {
			// first node can have an explicit rename due to client capturing the new node
			nt = to
		} else {
			nt = c.Inc()
		}
		if nt != t.g0[start+i].ntt {
			it(t.g0[start+i].ntt, nt)
		}
		t.g0[start+i].ntt = nt
	}
}

func (t *etree) addHandler(hdl intentHandler) { t.g0[len(t.g0)-1].hdl = hdl }
func (t *etree) closeScope(of int)            { t.g0[of].scp = len(t.g0) - of }

// children returns the subtree rooted at entity nt (including the entity itself).
// if the entity does not exist, it returns a nil value.
func (t *etree) children(nt Entity) []prenode {
	i := t.locate(nt)
	if i == -1 {
		return nil
	}

	return t.g1[i : i+t.g1[i].scp]
}

func (t *etree) parents(nt Entity) []prenode {
	i := t.locate(nt)
	assert(i != -1, `entity %d does not exist in the element tree. 
	This is only possible if the state of the view and the model are out of sync.
	You may have passed an invalid entity alongside a datacell intent.
	I prefer to bail out.`, nt)

	chain := make([]prenode, 0, i)
	for j := i; j >= 0; j-- {
		if t.g1[j].scp+j > i {
			chain = append(chain, t.g1[j])
		}
	}
	return chain
}

func (t *etree) locate(nt Entity) int {
	assert(int(nt) < len(t.sparse), "need larger memory space!")

	i := t.sparse[int(nt)]
	if i >= len(t.g1) || t.g1[i].ntt != nt {
		return -1
	}

	return i
}

// DumpETree is a debug utility printing all entities in a node subtree as a tree
func DumpETree(n *Node) { recdumpetree(n, "") }

func recdumpetree(n *Node, pfx string) {
	if n.Entity != 0 {
		fmt.Println(pfx, n.TagName, "[", n.Entity, "]")
	} else {
		fmt.Println(pfx, n.TagName, "[???]")
	}

	for _, n := range n.Children {
		recdumpetree(n, pfx+" ")
	}
}
