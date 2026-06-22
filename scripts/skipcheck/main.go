// skipcheck flags committed test functions that begin with an
// unconditional t.Skip / t.SkipNow / t.Skipf, the placeholder-
// stub shape Audit G surfaced in internal/contacts/sync_test.go.
//
// Walks every *_test.go below the working directory. For each
// func Test*(t *testing.T) whose body's first executable
// statement is a top-level t.Skip[…](…) call (not nested in if,
// switch, or select), prints file:line: SK1 and exits 1.
//
// Legitimate guarded skips (testing.Short, GOOS, env-gated
// integration) sit inside an if and pass cleanly. Build-tag
// fences keep their files out of the default compilation set,
// so their bodies never reach this walker.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	hits := 0
	fset := token.NewFileSet()
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == ".git" || strings.HasPrefix(name, ".") && name != "." {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipcheck: %s: %v\n", path, err)
			return nil
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			tName, ok := testParam(fn)
			if !ok {
				continue
			}
			stmt := firstStmt(fn.Body)
			if stmt == nil {
				continue
			}
			if call, ok := skipCall(stmt, tName); ok {
				pos := fset.Position(call.Pos())
				fmt.Printf("%s:%d: SK1 unconditional %s in %s; guard with if testing.Short()/GOOS/env or fence with a build tag\n",
					pos.Filename, pos.Line, callName(call), fn.Name.Name)
				hits++
			}
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		fmt.Fprintf(os.Stderr, "skipcheck: %v\n", err)
		os.Exit(2)
	}
	if hits > 0 {
		os.Exit(1)
	}
}

// testParam returns the parameter name bound to *testing.T for a
// top-level func TestXxx(t *testing.T) declaration.
func testParam(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv != nil || fn.Body == nil {
		return "", false
	}
	if !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "Test" {
		return "", false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return "", false
	}
	field := fn.Type.Params.List[0]
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return "", false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "testing" || sel.Sel.Name != "T" {
		return "", false
	}
	if len(field.Names) != 1 {
		return "", false
	}
	return field.Names[0].Name, true
}

func firstStmt(body *ast.BlockStmt) ast.Stmt {
	if body == nil || len(body.List) == 0 {
		return nil
	}
	return body.List[0]
}

func skipCall(stmt ast.Stmt, tName string) (*ast.CallExpr, bool) {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	switch sel.Sel.Name {
	case "Skip", "SkipNow", "Skipf":
	default:
		return nil, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != tName {
		return nil, false
	}
	return call, true
}

func callName(c *ast.CallExpr) string {
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name + "." + sel.Sel.Name
		}
	}
	return "Skip"
}
