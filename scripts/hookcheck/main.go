// hookcheck flags production access to internal/uerr's log-redirect
// hook. RedirectForTest swaps uerr's log destination for the whole
// process, so one production call would divert every ER-1 error line
// for the life of that process, the exact failure the uerr seam exists
// to prevent. It has to be exported for internal/uerr/uerrtest to
// reach it from another package, which also puts it within reach of
// every other package.
//
// Two rules, both over every .go file below the working directory that
// is not a _test.go file:
//
//	UH1  a reference to RedirectForTest from outside internal/uerr.
//	UH2  an import of internal/uerr/uerrtest from outside internal/uerr.
//
// Files inside internal/uerr are exempt from both, since uerrtest's own
// wrapper is a non-test file that has to call the hook. UH2 is what
// makes that exemption sound: uerrtest is the only route into the
// exempt tree, and nothing else in the tree stops a production file
// importing it. Taking a *testing.T is no barrier on its own, because
// &testing.T{} is constructible from anywhere.
//
// Each violation prints file:line and exits 1.
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

// hook is the identifier no production file may name.
const hook = "RedirectForTest"

// uerrDir is the one package tree exempt from both rules, in
// slash-separated form regardless of the host's separator.
const uerrDir = "internal/uerr"

// uerrtestPath is the import path no production file may import: the
// only package outside internal/uerr that reaches hook.
const uerrtestPath = "internal/uerr/uerrtest"

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
		slashed := filepath.ToSlash(path)
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") && name != "." {
				return fs.SkipDir
			}
			if slashed == uerrDir || strings.HasSuffix(slashed, "/"+uerrDir) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hookcheck: %s: %v\n", path, err)
			return nil
		}
		for _, imp := range f.Imports {
			if !importsUerrtest(imp) {
				continue
			}
			report(fset, imp.Pos(), "UH2 %s imported from a non-test file outside %s; it reaches %s, which redirects every error line in the process",
				uerrtestPath, uerrDir, hook)
			hits++
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok || ident.Name != hook {
				return true
			}
			report(fset, ident.Pos(), "UH1 %s referenced from a non-test file outside %s; it redirects every error line in the process",
				hook, uerrDir)
			hits++
			return true
		})
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		fmt.Fprintf(os.Stderr, "hookcheck: %v\n", err)
		os.Exit(2)
	}
	if hits > 0 {
		os.Exit(1)
	}
}

// importsUerrtest reports whether imp names uerrtestPath, under any
// module prefix.
func importsUerrtest(imp *ast.ImportSpec) bool {
	path := strings.Trim(imp.Path.Value, `"`)
	return path == uerrtestPath || strings.HasSuffix(path, "/"+uerrtestPath)
}

func report(fset *token.FileSet, pos token.Pos, format string, args ...any) {
	p := fset.Position(pos)
	fmt.Printf("%s:%d: %s\n", p.Filename, p.Line, fmt.Sprintf(format, args...))
}
