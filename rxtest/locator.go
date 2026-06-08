// Package rxtest contains facilities to test rendering of RX trees without browser
package rxtest

import (
	"fmt"
	"regexp"
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

func (e Element) String() string {
	if e == notFound {
		return "<not-found>"
	}

	return e.rxNode.ToHTML()
}

func Root(n *rx.Node) Element { return Element{rxNode: n} }

// Matcher represent a way to find element in a page
type Matcher func(*rx.Node) bool

// Locate Element in the Widget tree produced by [rx.Build].
// The first locator is finding a specific node, while later node filter the resulting set.
//
//	Locate(n, ByRole("heading"), ByText("hello"))
//
// returns the first h1, h2, … node that contain a node "hello"
func Locate(e Element, locs ...Matcher) Element {
	if len(locs) == 0 {
		return e
	}

	loc, filters := locs[0], locs[1:]
	if loc(e.rxNode) {
		switch len(filters) {
		default:
			panic("too many filters given")
		case 0:
			return e
		case 1:
			if matchChild(e.rxNode, filters[0]) {
				return e
			}
			return notFound
		}
	}

	var matches []Element
	for _, c := range e.rxNode.Children {
		if r := Locate(Element{rxNode: c, parent: &e}, locs...); r != notFound {
			matches = append(matches, r)
		}
	}

	// strict matcher
	if len(matches) == 1 {
		return matches[0]
	}

	return notFound
}

func matchChild(n *rx.Node, m Matcher) bool {
	for _, c := range n.Children {
		if m(c) || matchChild(c, m) {
			return true
		}
	}
	return false
}

// Expect expresses a test assertion that an element E matches all predicates
func Expect(e Element, preds ...Matcher) bool {
	if e == notFound {
		return false
	}

	for _, m := range preds {
		if !m(e.rxNode) {
			return false
		}
	}
	return true
}

type RoleOption struct {
	Name string
}

var ariaRoles = map[string][]string{ // Document structure roles
	"article":      {"article"},
	"cell":         {"td"},
	"columnheader": {"th"},
	"definition":   {"dfn"},
	"directory":    {"ul"},
	"document":     {"body"},
	"feed":         {"div"},
	"figure":       {"figure"},
	"group":        {"div"},
	"heading":      {"h1", "h2", "h3", "h4", "h5", "h6"},
	"img":          {"img"},
	"list":         {"ul", "ol"},
	"listitem":     {"li"},
	"math":         {"math"},
	"note":         {"aside", "footer"},
	"presentation": {"div", "span"},
	"row":          {"tr"},
	"rowgroup":     {"thead", "tbody", "tfoot"},
	"rowheader":    {"th"},
	"separator":    {"hr", "div"},
	"table":        {"table"},
	"term":         {"dfn", "dt"},
	"toolbar":      {"div"},
	"tooltip":      {"div"},
	"application":  {"div"},

	"associationlist":          {"dl"},
	"associationlistitemkey":   {"dt"},
	"associationlistitemvalue": {"dd"},
	"blockquote":               {"blockquote"},
	"caption":                  {"caption"},
	"code":                     {"code"},
	"deletion":                 {"del", "s"},
	"emphasis":                 {"em", "i"},
	"insertion":                {"ins", "u"},
	"paragraph":                {"p"},
	"strong":                   {"strong", "b"},
	"subscript":                {"sub"},
	"superscript":              {"sup"},
	"time":                     {"time"},

	"button":           {"button", "input"},
	"checkbox":         {"input"},
	"gridcell":         {"td", "div"},
	"link":             {"a"},
	"menuitem":         {"li", "div"},
	"menuitemcheckbox": {"li", "div"},
	"menuitemradio":    {"li", "div"},
	"option":           {"option", "div"},
	"progressbar":      {"progress", "div"},
	"radio":            {"input"},
	"scrollbar":        {"div"},
	"searchbox":        {"input", "div"},
	"slider":           {"input", "div"},
	"spinbutton":       {"div"},
	"status":           {"div", "span", "output"},
	"switch":           {"button", "div"},
	"tab":              {"button", "div", "li"},
	"textbox":          {"input", "textarea", "div"},

	"combobox":   {"div", "input"},
	"grid":       {"div", "table"},
	"listbox":    {"div", "select", "ul", "ol"},
	"menu":       {"ul", "div"},
	"menubar":    {"ul", "div"},
	"radiogroup": {"fieldset", "div"},
	"tablist":    {"div", "ul"},
	"tree":       {"ul", "div"},
	"treegrid":   {"table", "div"},

	"banner":        {"header", "div"},
	"complementary": {"aside", "div"},
	"contentinfo":   {"footer", "div"},
	"form":          {"form", "div"},
	"main":          {"main", "div"},
	"navigation":    {"nav", "div"},
	"region":        {"section", "div"},
	"search":        {"search", "div"},

	"alert":   {"div", "span"},
	"log":     {"div", "span"},
	"marquee": {"div"},
	"timer":   {"div", "span"},

	"alertdialog": {"dialog", "div"},
	"dialog":      {"dialog", "div"},

	"command":     {"button", "div"},
	"composite":   {"div"},
	"input":       {"input", "textarea", "select", "div"},
	"landmark":    {"div", "span"},
	"range":       {"input", "div"},
	"roletype":    {"div"},
	"section":     {"div", "section"},
	"sectionhead": {"h1", "h2", "h3", "h4", "h5", "h6"},
	"select":      {"select", "div"},
	"structure":   {"div"},
	"widget":      {"div"},
	"window":      {"div"},

	"comment":    {"div", "span", "aside"},
	"generic":    {"div", "span"},
	"mark":       {"mark", "span"},
	"meter":      {"meter", "div"},
	"suggestion": {"div", "span"},

	"none": {"div", "span"},
}

var notFound = Element{}

func HasRole(role string, opts RoleOption) Matcher {
	tags := ariaRoles[role]
	if len(tags) == 0 {
		// allow self element
		tags = []string{role}
	}

	return func(n *rx.Node) bool {
		if slices.Contains(tags, n.TagName) {
			return true
		}

		return false
	}
}

// HasText finds a node whose text content matches the given regex pattern.
func HasText(pattern string) Matcher {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(fmt.Sprintf("invalid regexp %s: %s", pattern, err))
	}
	return func(n *rx.Node) bool {
		return re.MatchString(n.Text)
	}
}

// hasAttr checks if a node has an attribute with the given name
func hasAttr(n *rx.Node, name string) bool {
	for _, a := range n.Attrs {
		if a.Name == name {
			return true
		}
	}
	return false
}

// ByTestID finds a node with the matching data-testid attribute.
func ByTestID(testid string) Matcher {
	return func(n *rx.Node) bool {
		return n.GetAttr("data-testid") == testid
	}
}

// IsChecked returns a matcher that checks if a checkbox/radio is checked.
func IsChecked() Matcher {
	return func(n *rx.Node) bool {
		// Check for checked attribute (HTML boolean attribute)
		if hasAttr(n, "checked") {
			return true
		}
		// Check for aria-checked="true"
		if n.GetAttr("aria-checked") == "true" {
			return true
		}
		return false
	}
}

// IsDisabled returns a matcher that checks if an element is disabled.
func IsDisabled() Matcher {
	return func(n *rx.Node) bool {
		// Check for disabled attribute (HTML boolean attribute)
		if hasAttr(n, "disabled") {
			return true
		}
		// Check for aria-disabled="true"
		if n.GetAttr("aria-disabled") == "true" {
			return true
		}
		return false
	}
}

// IsEditable returns a matcher that checks if an element is editable.
func IsEditable() Matcher {
	return func(n *rx.Node) bool {
		// Check if it's an input/textarea/select without disabled or readonly
		if slices.Contains([]string{"input", "textarea", "select"}, n.TagName) {
			if !hasAttr(n, "disabled") && !hasAttr(n, "readonly") {
				return true
			}
		}
		// Check for contenteditable attribute
		if n.GetAttr("contenteditable") == "true" {
			return true
		}
		// Check for aria-editable="true"
		if n.GetAttr("aria-editable") == "true" {
			return true
		}
		return false
	}
}

// IsHidden returns a matcher that checks if an element is hidden.
func IsHidden() Matcher {
	return func(n *rx.Node) bool {

		// Check for hidden attribute
		if hasAttr(n, "hidden") {
			return true
		}
		// Check for aria-hidden="true"
		if n.GetAttr("aria-hidden") == "true" {
			return true
		}
		return false
	}
}

// IsVisible returns a matcher that checks if an element is visible.
func IsVisible() Matcher {
	return func(n *rx.Node) bool {
		return !IsHidden()(n)
	}
}

// HasClass returns a matcher that checks if an element has the specified class.
func HasClass(class string) Matcher {
	return func(n *rx.Node) bool {
		if n.Classes == "" {
			return false
		}
		// Check Classes field first
		return slices.Contains(strings.Fields(n.Classes), class)
	}
}
