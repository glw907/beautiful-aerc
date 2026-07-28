// Package errorconstruction reports poplar's uerr error type (ER-1)
// constructed outside internal/uerr other than through its exported
// constructor: a composite literal of a type internal/uerr declares,
// built from any other package. Calling the constructor itself, and
// calling methods on the value it returns, are unrestricted.
package errorconstruction

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/glw907/poplar/tools/analyzers/pkgrole"
)

const doc = `check uerr construction stays inside internal/uerr

Reports a composite literal, outside internal/uerr, of a type
internal/uerr declares. internal/uerr's exported constructor is the
sanctioned way to build one.`

// Analyzer reports uerr composite literals built outside
// internal/uerr.
var Analyzer = &analysis.Analyzer{
	Name: "errorconstruction",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	role, _, ok := pkgrole.Of(pass.Pkg.Path())
	if ok && role == "uerr" {
		return nil, nil
	}

	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if name, bad := declaredInUerr(pass, lit); bad {
				pass.Reportf(lit.Pos(),
					"%s constructed outside internal/uerr; use internal/uerr's exported constructor", name)
			}
			return true
		})
	}
	return nil, nil
}

// declaredInUerr reports whether lit's type is a named type
// internal/uerr declares, and that type's qualified name.
func declaredInUerr(pass *analysis.Pass, lit *ast.CompositeLit) (name string, bad bool) {
	named, ok := pass.TypesInfo.TypeOf(lit).(*types.Named)
	if !ok {
		return "", false
	}
	obj := named.Obj()
	if obj.Pkg() == nil {
		return "", false
	}
	role, _, ok := pkgrole.Of(obj.Pkg().Path())
	if !ok || role != "uerr" {
		return "", false
	}
	return obj.Pkg().Name() + "." + obj.Name(), true
}
