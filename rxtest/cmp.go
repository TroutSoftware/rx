package rxtest

import (
	"slices"
	"strings"

	"github.com/TroutSoftware/rx"
)

// Element represents a UI element (e.g. an HTML div) located in a widget tree.
// The resulting element can be acted upon by user events, such as Click.
type Element struct {
	parent *Element
	rxNode *rx.Node
}

var notFound = Element{}

func (e Element) String() string {
	if e == notFound {
		return "<not-found>"
	}

	return e.rxNode.ToHTML()
}

func (e Element) Exists() bool { return e != notFound }

func Root(n *rx.Node) Element { return Element{rxNode: n} }

// match returns true iff node match the definition by target
// matching rules are not strict equality: the target is meant to be used as
// a convenient template to filter nodes based on its shape.
//
// Specifically, a node match iff:
//   - (always) they have the same tag name or target has "any" tag
//   - (option) they have the same test id
//   - (option) each class in the target is present in the node
//   - (option) the content in target is a subtext of the node text
//   - (option) the form name attribute matches
//
// Children are not considered in the match
func match(node, target *rx.Node) bool {
	if target.TagName != "any" && node.TagName != target.TagName {
		return false
	}

	if tid := target.GetAttr("data-testid"); tid != "" && tid != node.GetAttr("data-testid") {
		return false
	}

	if target.Classes != "" {
		ncs := strings.Split(node.Classes, " ")
		for cls := range strings.SplitSeq(target.Classes, " ") {
			if !slices.Contains(ncs, cls) {
				return false
			}
		}
	}

	if txt := target.Text; txt != "" && !strings.Contains(node.Text, target.Text) {
		return false
	}
	// HTML inputs are their own little world…
	if target.TagName == "input" {
		if tag := target.GetAttr("type"); tag != "" && tag != node.GetAttr("type") {
			return false
		}
		if txt := target.GetAttr("value"); txt != "" && !strings.Contains(node.GetAttr("value"), txt) {
			return false
		}
	}

	if fn := target.GetAttr("name"); fn != "" && fn != node.GetAttr("name") {
		return false
	}

	return true
}

func LocateIn(n *rx.Node, targets ...*rx.Node) Element {
	return Locate(Root(n), targets...)
}

// Locate finds sub-elements matching targets.
func Locate(e Element, targets ...*rx.Node) Element {
	if len(targets) == 0 || e == notFound {
		return e
	}

	current, next := targets[0], targets[1:]
	if match(e.rxNode, current) {
		targets = next
		if len(targets) == 0 {
			return e
		}
	}

	var matches []Element
	for _, c := range e.rxNode.Children {
		if r := Locate(Element{rxNode: c, parent: &e}, targets...); r != notFound {
			matches = append(matches, r)
		}
	}

	// strict matcher
	if len(matches) == 1 {
		return matches[0]
	}

	return notFound
}
