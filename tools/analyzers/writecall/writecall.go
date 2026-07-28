// Package writecall reports code outside internal/store that
// constructs or calls a store-write entry point directly, the
// package-boundary half of ADR-0003 (technical design section 2):
// a write happens only inside internal/store's writer path, or
// through the intent gateway (internal/store's Apply/Dispatch
// functions, which accept the intent types).
//
// The analyzer is syntactic, not type-based: it does not verify
// that a call site actually reaches a write against a live
// connection, and it does not forbid Exec on a read connection.
// That half is not statically decidable in go/analysis, so the
// store's read API owns it instead, by returning a named
// read-only handle type with no Exec method.
package writecall

import (
	"go/ast"
	"go/token"
	"regexp"

	"golang.org/x/tools/go/analysis"

	"github.com/glw907/poplar/tools/analyzers/pkgrole"
)

const doc = `check store-write calls stay inside internal/store

Reports a call or composite literal outside internal/store that
reaches a write entry point: a function named New*Writer or
Write*, or a Writer composite literal. The sanctioned way in is
internal/store's intent gateway (Apply, Dispatch).`

// Analyzer reports store-write entry points reached from outside
// internal/store.
var Analyzer = &analysis.Analyzer{
	Name: "writecall",
	Doc:  doc,
	Run:  run,
}

// entryPoint matches the names of store-write entry points: writer
// constructors (NewWriter, NewThreadWriter, ...) and write
// operations (WriteMessage, Write, ...).
var entryPoint = regexp.MustCompile(`^(New\w*Writer|Write\w*)$`)

// writerType matches the store's writer type by name, for the
// composite-literal form of construction (store.Writer{...}).
var writerType = regexp.MustCompile(`^\w*Writer$`)

func run(pass *analysis.Pass) (any, error) {
	role, _, ok := pkgrole.Of(pass.Pkg.Path())
	if ok && role == "store" {
		return nil, nil
	}

	for _, f := range pass.Files {
		storeAlias := storeImportAlias(f)
		if storeAlias == "" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
					checkEntryPoint(pass, n.Pos(), sel, storeAlias, entryPoint, "reaches")
				}
			case *ast.CompositeLit:
				if sel, ok := n.Type.(*ast.SelectorExpr); ok {
					checkEntryPoint(pass, n.Pos(), sel, storeAlias, writerType, "constructs")
				}
			}
			return true
		})
	}
	return nil, nil
}

// storeImportAlias returns the local identifier file uses for
// internal/store, or "" if the file does not import it.
func storeImportAlias(f *ast.File) string {
	for _, imp := range f.Imports {
		role, _, ok := pkgrole.Of(pkgrole.ImportPath(imp))
		if !ok || role != "store" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "store"
	}
	return ""
}

// checkEntryPoint reports pos if sel is storeAlias.X for an X
// matching pattern: a call to a write function, or a composite
// literal of the writer type, reaching outside internal/store.
func checkEntryPoint(pass *analysis.Pass, pos token.Pos, sel *ast.SelectorExpr, storeAlias string, pattern *regexp.Regexp, verb string) {
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != storeAlias || !pattern.MatchString(sel.Sel.Name) {
		return
	}
	pass.Reportf(pos,
		"%s.%s %s a store-write entry point outside internal/store; go through internal/store's intent gateway (Apply, Dispatch)",
		storeAlias, sel.Sel.Name, verb)
}
