package mutate

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name:     "checkmutate",
	Doc:      "calls to mutate functions must be single pointers",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run_callptr,
}

func run_callptr(pass *analysis.Pass) (any, error) {
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
		if fn.FullName() != "github.com/TroutSoftware/rx.Mutate" {
			return
		}

		for _, arg := range call.Args {
			flit, ok := arg.(*ast.FuncLit)
			if !ok {
				pass.Reportf(arg.Pos(), "mutate function must be single pointers")
				continue
			}

			if flit.Type.Params.NumFields() != 1 {
				pass.Reportf(arg.Pos(), "mutate function must be single pointers")
				continue
			}

			if _, ptr := flit.Type.Params.List[0].Type.(*ast.StarExpr); !ptr {
				pass.Reportf(arg.Pos(), "mutate function must be single pointers")
			}
		}
	})

	return nil, nil
}
