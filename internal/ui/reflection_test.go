package ui

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestReflection_EveryScreenTypeIsRegistered is UX-1's acceptance
// test: any type under internal/ui/... shaped like a Screen (it
// declares Init, Update, View, and Entry with Screen's exact
// signatures) must appear in the package-level registry. It works
// without any screen existing yet, since screenTypesInSource scans
// source rather than a fixed list, and task 5's placeholder screens
// register themselves at init for it to find.
func TestReflection_EveryScreenTypeIsRegistered(t *testing.T) {
	found, err := screenTypesInSource(".")
	if err != nil {
		t.Fatalf("scan internal/ui for screen-shaped types: %v", err)
	}

	registeredNames := make(map[string]bool)
	for _, e := range Registered() {
		registeredNames[e.Type.Name()] = true
	}

	for name := range found {
		if !registeredNames[name] {
			t.Errorf("type %s implements Screen (Init/Update/View/Entry) but is not registered via ui.Register", name)
		}
	}
}

// screenTypesInSource returns the name of every type declared in a
// non-test .go file under dir that declares all four of Screen's
// methods with Screen's exact signatures: Init() tea.Cmd,
// Update(tea.Msg) (tea.Model, tea.Cmd), View() tea.View, and
// Entry() ScreenEntry. It reads syntax only, so it needs no importer
// for tea's own package to resolve identifiers.
func screenTypesInSource(dir string) (map[string]bool, error) {
	fset := token.NewFileSet()
	methodsByType := make(map[string]map[string]bool)

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			typeName := receiverTypeName(fn)
			method := matchScreenMethod(fset, fn)
			if typeName == "" || method == "" {
				continue
			}
			if methodsByType[typeName] == nil {
				methodsByType[typeName] = make(map[string]bool)
			}
			methodsByType[typeName][method] = true
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	screens := make(map[string]bool)
	for name, methods := range methodsByType {
		if methods["Init"] && methods["Update"] && methods["View"] && methods["Entry"] {
			screens[name] = true
		}
	}
	return screens, nil
}

// receiverTypeName returns fn's receiver type name, stripping a
// pointer or a generic instantiation's type parameters, or "" for a
// function with no receiver.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexListExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// matchScreenMethod returns which of Screen's four methods fn
// matches by name and exact signature, or "" if fn matches none.
func matchScreenMethod(fset *token.FileSet, fn *ast.FuncDecl) string {
	params := fieldTypes(fset, fn.Type.Params)
	results := fieldTypes(fset, fn.Type.Results)

	switch fn.Name.Name {
	case "Init":
		if len(params) == 0 && slices.Equal(results, []string{"tea.Cmd"}) {
			return "Init"
		}
	case "Update":
		if len(params) == 1 && params[0] == "tea.Msg" && slices.Equal(results, []string{"tea.Model", "tea.Cmd"}) {
			return "Update"
		}
	case "View":
		if len(params) == 0 && slices.Equal(results, []string{"tea.View"}) {
			return "View"
		}
	case "Entry":
		if len(params) == 0 && slices.Equal(results, []string{"ScreenEntry"}) {
			return "Entry"
		}
	}
	return ""
}

// fieldTypes renders each field type in fl as source text, in
// order, expanding a field that names multiple identifiers
// ("a, b int") into one entry per identifier.
func fieldTypes(fset *token.FileSet, fl *ast.FieldList) []string {
	if fl == nil {
		return nil
	}
	var out []string
	for _, f := range fl.List {
		text := exprString(fset, f.Type)
		n := max(len(f.Names), 1)
		for range n {
			out = append(out, text)
		}
	}
	return out
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		return ""
	}
	return buf.String()
}
