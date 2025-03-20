package get

import (
	"bytes"
	"errors"
	"go/ast"
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name:     "get",
	Doc:      `Check that calls to rx.Get are well-formed`,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run_wellformed,
}

func run_wellformed(pass *analysis.Pass) (any, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	funcs := []ast.Node{(*ast.CallExpr)(nil)}
	inspect.Preorder(funcs, func(node ast.Node) {
		call := node.(*ast.CallExpr)
		fn := typeutil.StaticCallee(pass.TypesInfo, call)
		if fn == nil {
			return // dynamic call
		}
		if len(call.Args) != 1 {
			return // used as value
		}
		if fn.FullName() != "github.com/TroutSoftware/rx.Get" {
			return
		}

		if _, ok := call.Args[0].(*ast.SelectorExpr); ok {
			// skip dynamic assignment
			return
		}

		// TODO: Check if the call is commented

		tpl, _ := strconv.Unquote(call.Args[0].(*ast.BasicLit).Value)

		countstart := 0
		var nodes []string
		tk := html.NewTokenizer(strings.NewReader(tpl))
	tplLoop:
		for {
			switch tk.Next() {
			case html.TextToken:
				if bytes.ContainsAny(tk.Raw(), "<>\"") {
					pass.ReportRangef(node, "no escape characters in text use &lt, &gt, …")
					return
				}
			case html.ErrorToken:
				if errors.Is(tk.Err(), io.EOF) {
					break tplLoop
				}
				pass.ReportRangef(node, "error reading value %s", tk.Err())
				return
			case html.StartTagToken:
				countstart++

				n, _ := tk.TagName()
				nodes = append(nodes, string(n))

			case html.EndTagToken:
				n, _ := tk.TagName()

				if len(nodes) == 0 {
					pass.ReportRangef(node, "extraneous close token: %s", n)
					return
				}
				if nodes[len(nodes)-1] != string(n) {
					pass.ReportRangef(node, "unmatched close token: %s closing %s", n, nodes[len(nodes)-1])
					return
				}
				nodes = nodes[:len(nodes)-1]
			}

		}
		switch countstart {
		case 0:
			// any invalid element will prevent the start at all
			pass.ReportRangef(node, "invalid element")
		case 1:
			// for single element, we don’t need to close

			return
		}

		for l := len(nodes) - 1; l >= 0; l-- {
			pass.ReportRangef(node, "unbalanced: %s", nodes[l])
		}

	})
	return nil, nil
}
